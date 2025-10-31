package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	grpcapi "github.com/igodwin/notifier/api/grpc"
	pb "github.com/igodwin/notifier/api/grpc/pb"
	"github.com/igodwin/notifier/api/rest"
	"github.com/igodwin/notifier/internal/auth"
	"github.com/igodwin/notifier/internal/config"
	"github.com/igodwin/notifier/internal/domain"
	"github.com/igodwin/notifier/internal/logging"
	"github.com/igodwin/notifier/internal/notifier"
	"github.com/igodwin/notifier/internal/queue"
	"github.com/igodwin/notifier/internal/service"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var (
	// Build information - set via ldflags during build
	// Example: go build -ldflags "-X main.Version=1.0.0 -X main.GitCommit=$(git rev-parse HEAD)"
	Version   = "dev"
	GitCommit = "unknown"
	BuildTime = "unknown"
)

func main() {
	// Print service identifier and build info
	fmt.Printf("====================================\n")
	fmt.Printf("Notifier Service\n")
	fmt.Printf("Version:    %s\n", Version)
	fmt.Printf("Git Commit: %s\n", GitCommit)
	fmt.Printf("Build Time: %s\n", BuildTime)
	fmt.Printf("====================================\n")

	// Load configuration
	cfg, err := config.Load("")
	if err != nil {
		// Use basic logger before we have config
		logger, _ := logging.NewFromConfig("info", "stdout")
		logger.Warnf("Failed to load config, using defaults: %v", err)
		cfg = getDefaultConfig()
	}

	// Create logger from config
	logger, err := logging.NewFromConfig(cfg.Logging.Level, cfg.Logging.OutputPath)
	if err != nil {
		// Fallback to stdout if log file can't be opened
		logger, _ = logging.NewFromConfig(cfg.Logging.Level, "stdout")
		logger.Warnf("Failed to open log file, using stdout: %v", err)
	}

	// Log which config file was loaded
	logger.Infof("Loaded configuration from: %s", cfg.ConfigFile)

	// Log sanitized config (with sensitive data redacted)
	if sanitized, err := json.MarshalIndent(cfg.Sanitize(), "", "  "); err == nil {
		logger.Infof("Configuration:\n%s", string(sanitized))
	}

	logger.Infof("Starting Notifier Service in mode: %s", cfg.Server.Mode)

	// Create context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize queue
	var q domain.Queue
	if cfg.Queue.Type == "local" {
		q, err = queue.NewLocalQueue(cfg.Queue.Local)
		if err != nil {
			logger.Fatalf("Failed to create queue: %v", err)
		}
		logger.Info("Using local queue")
	} else {
		logger.Fatalf("Queue type %s not implemented yet", cfg.Queue.Type)
	}

	// Initialize authentication if enabled (must be before service creation for RBAC)
	var authStore *auth.APIKeyStore
	var hybridKeyStore *auth.HybridKeyStore
	var authz *auth.NotifierAuthz
	if cfg.Auth.Enabled {
		authStore = auth.NewAPIKeyStore()
		authz = auth.NewNotifierAuthz()
		logger.Info("API authentication enabled")

		// Create database backend if configured
		var dbStore *auth.KeyStoreDB
		if cfg.Auth.Database.URL != "" {
			dbStore, err = auth.NewKeyStoreDB(cfg.Auth.Database.URL)
			if err != nil {
				logger.Fatalf("Failed to create database key store: %v", err)
			}
			logger.Infof("Connected to authentication database: %s", cfg.Auth.Database.URL)
		} else {
			logger.Warn("No database configured for authentication - API keys will only be stored in memory")
		}

		// Create hybrid key store for key management (in-memory cache + database backend)
		hybridKeyStore = auth.NewHybridKeyStore(authStore, dbStore)
		logger.Debugf("Initialized hybrid key store for API key management")

		// Bootstrap admin key if configured
		if cfg.Auth.Bootstrap.Enabled {
			bootstrapCfg := &auth.BootstrapConfig{
				Enabled:          cfg.Auth.Bootstrap.Enabled,
				AdminKeyFileName: cfg.Auth.Bootstrap.AdminKeyFileName,
				PrintToStdout:    cfg.Auth.Bootstrap.PrintToStdout,
			}

			// Try to load existing key from Kubernetes secret first
			existingKey, err := auth.LoadAdminKeyFromKubernetesSecret(
				ctx,
				cfg.Auth.Bootstrap.KubernetesSecretName,
				cfg.Auth.Bootstrap.KubernetesSecretKey,
				logger,
			)
			if err != nil {
				logger.Warnf("Error loading from Kubernetes secret: %v", err)
			}

			// If we have an existing key, use it
			if existingKey != "" {
				if _, err := auth.RegisterAdminKeyInMemory(authStore, existingKey, logger); err != nil {
					logger.Warnf("Failed to register existing admin key: %v", err)
				}
			} else {
				// Generate new key
				if apiKey, err := auth.BootstrapAdminKeyInMemory(authStore, bootstrapCfg, logger); err != nil {
					logger.Warnf("Bootstrap admin key creation failed: %v", err)
				} else if apiKey != nil {
					// Store in Kubernetes secret if configured
					if cfg.Auth.Bootstrap.KubernetesSecretName != "" {
						if err := auth.CreateKubernetesSecret(
							ctx,
							cfg.Auth.Bootstrap.KubernetesSecretName,
							cfg.Auth.Bootstrap.KubernetesSecretKey,
							apiKey.Key,
							logger,
						); err != nil {
							logger.Warnf("Failed to create Kubernetes secret: %v", err)
						}
					}
				}
			}
		}
	}

	// Initialize notifier factory and register notifiers
	factory := notifier.NewFactory()
	registerNotifiers(cfg, factory, logger)

	// Check if any notifiers are registered
	if len(factory.SupportedTypes()) == 0 {
		logger.Fatal("No notifiers configured. Please enable at least one notifier in notifier.config")
	}

	logger.Infof("Supported notification types: %v", factory.SupportedTypes())

	// Register authorization rules for notifiers (after factory registration)
	if authz != nil {
		registerAuthorizationRules(cfg, authz, logger)
	}

	// Create notification service (pass config as account resolver and authz for RBAC)
	svc := service.NewNotificationService(factory, q, cfg.Queue.WorkerCount, cfg, authz, logger)

	// Configure notification retention if enabled
	if err := svc.WithRetentionConfig(cfg.Retention); err != nil {
		logger.Warnf("Failed to configure retention: %v", err)
		// Log defaults that will be used
		logger.Infof("Using default retention config: enabled=%v", cfg.Retention.Enabled)
	} else if cfg.Retention.Enabled {
		logger.Infof("Configured notification retention: ttl=%s, check_frequency=%s, max_size=%d",
			cfg.Retention.TTL, cfg.Retention.CheckFrequency, cfg.Retention.MaxSize)
	}

	// Start workers
	if err := svc.Start(ctx); err != nil {
		logger.Fatalf("Failed to start service: %v", err)
	}
	logger.Infof("Started %d worker(s)", cfg.Queue.WorkerCount)

	// Wait group for both servers
	var wg sync.WaitGroup

	// Start gRPC server if enabled
	var grpcServer *grpc.Server
	if cfg.Server.Mode == "both" || cfg.Server.Mode == "grpc" {
		wg.Add(1)
		grpcServer = startGRPCServer(ctx, &wg, cfg, svc, logger, authStore)
	}

	// Start REST server if enabled
	var restServer *http.Server
	if cfg.Server.Mode == "both" || cfg.Server.Mode == "rest" {
		wg.Add(1)
		restServer = startRESTServer(ctx, &wg, cfg, svc, logger, authStore, hybridKeyStore)
	}

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	logger.Info("Shutting down servers...")

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// Stop REST server
	if restServer != nil {
		if err := restServer.Shutdown(shutdownCtx); err != nil {
			logger.Errorf("Error during REST server shutdown: %v", err)
		}
	}

	// Stop gRPC server
	if grpcServer != nil {
		grpcServer.GracefulStop()
	}

	// Wait for servers to stop
	wg.Wait()

	// Stop service
	if err := svc.Stop(); err != nil {
		logger.Errorf("Error stopping service: %v", err)
	}

	logger.Info("Servers stopped")
}

func registerNotifiers(cfg *config.Config, factory *notifier.Factory, logger *logging.Logger) {
	if cfg.Notifiers.Stdout {
		stdoutNotifier := notifier.NewStdoutNotifier()
		if err := factory.RegisterNotifier(domain.TypeStdout, "", stdoutNotifier); err != nil {
			logger.Fatalf("Failed to register stdout notifier: %v", err)
		}
		logger.Info("Registered stdout notifier")
	}

	// Register SMTP notifiers (now supports multiple accounts)
	for accountName, smtpConfig := range cfg.Notifiers.SMTP {
		smtpNotifier, err := notifier.NewSMTPNotifier(smtpConfig)
		if err != nil {
			logger.Warnf("Failed to create SMTP notifier for account '%s': %v", accountName, err)
		} else {
			if err := factory.RegisterNotifier(domain.TypeEmail, accountName, smtpNotifier); err != nil {
				logger.Fatalf("Failed to register SMTP notifier for account '%s': %v", accountName, err)
			}
			defaultStr := ""
			if smtpConfig.Default {
				defaultStr = " (default)"
			}
			logger.Infof("Registered SMTP notifier for account '%s'%s", accountName, defaultStr)
		}
	}

	// Register Slack notifiers (now supports multiple accounts)
	for accountName, slackConfig := range cfg.Notifiers.Slack {
		slackNotifier, err := notifier.NewSlackNotifier(slackConfig)
		if err != nil {
			logger.Warnf("Failed to create Slack notifier for account '%s': %v", accountName, err)
		} else {
			if err := factory.RegisterNotifier(domain.TypeSlack, accountName, slackNotifier); err != nil {
				logger.Fatalf("Failed to register Slack notifier for account '%s': %v", accountName, err)
			}
			defaultStr := ""
			if slackConfig.Default {
				defaultStr = " (default)"
			}
			logger.Infof("Registered Slack notifier for account '%s'%s", accountName, defaultStr)
		}
	}

	// Register Ntfy notifiers (now supports multiple accounts)
	for accountName, ntfyConfig := range cfg.Notifiers.Ntfy {
		ntfyNotifier, err := notifier.NewNtfyNotifier(ntfyConfig)
		if err != nil {
			logger.Warnf("Failed to create Ntfy notifier for account '%s': %v", accountName, err)
		} else {
			if err := factory.RegisterNotifier(domain.TypeNtfy, accountName, ntfyNotifier); err != nil {
				logger.Fatalf("Failed to register Ntfy notifier for account '%s': %v", accountName, err)
			}
			defaultStr := ""
			if ntfyConfig.Default {
				defaultStr = " (default)"
			}
			logger.Infof("Registered Ntfy notifier for account '%s'%s", accountName, defaultStr)
		}
	}
}

func startGRPCServer(ctx context.Context, wg *sync.WaitGroup, cfg *config.Config, svc domain.NotificationService, logger *logging.Logger, authStore *auth.APIKeyStore) *grpc.Server {
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.GRPCPort)

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		logger.Fatalf("Failed to listen on %s: %v", addr, err)
	}

	// Create gRPC server options
	var serverOpts []grpc.ServerOption

	// Add authentication interceptors if enabled
	if authStore != nil {
		authMiddleware := auth.NewGRPCAuthMiddleware(authStore, logger)
		serverOpts = append(serverOpts,
			grpc.UnaryInterceptor(authMiddleware.UnaryInterceptor()),
			grpc.StreamInterceptor(authMiddleware.StreamInterceptor()),
		)
	}

	grpcServer := grpc.NewServer(serverOpts...)

	// Create and register gRPC handler
	grpcHandler := grpcapi.NewNotifierHandler(svc, logger)
	pb.RegisterNotifierServiceServer(grpcServer, grpcHandler)

	// Enable reflection for tools like grpcurl
	reflection.Register(grpcServer)

	logger.Info("Registered gRPC NotifierService")

	go func() {
		defer wg.Done()
		logger.Infof("gRPC server listening on %s", addr)
		if err := grpcServer.Serve(lis); err != nil {
			logger.Fatalf("Failed to serve gRPC: %v", err)
		}
	}()

	return grpcServer
}

func startRESTServer(ctx context.Context, wg *sync.WaitGroup, cfg *config.Config, svc domain.NotificationService, logger *logging.Logger, authStore *auth.APIKeyStore, hybridKeyStore *auth.HybridKeyStore) *http.Server {
	var router *mux.Router
	if authStore != nil && hybridKeyStore != nil {
		router = rest.NewRouterWithAuthAndKeyStore(svc, logger, authStore, hybridKeyStore)
	} else if authStore != nil {
		router = rest.NewRouterWithAuth(svc, logger, authStore)
	} else {
		router = rest.NewRouter(svc, logger)
	}

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.RESTPort)
	server := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		defer wg.Done()
		logger.Infof("REST server listening on %s", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("Failed to start REST server: %v", err)
		}
	}()

	return server
}

func registerAuthorizationRules(cfg *config.Config, authz *auth.NotifierAuthz, logger *logging.Logger) {
	// Register SMTP authorization rules
	for accountName, smtpConfig := range cfg.Notifiers.SMTP {
		if len(smtpConfig.AllowedRoles) > 0 {
			authz.RegisterRule(domain.TypeEmail, accountName, smtpConfig.AllowedRoles)
			logger.Infof("Registered auth rule for SMTP account '%s' - allowed roles: %v", accountName, smtpConfig.AllowedRoles)
		}
	}

	// Register Slack authorization rules
	for accountName, slackConfig := range cfg.Notifiers.Slack {
		if len(slackConfig.AllowedRoles) > 0 {
			authz.RegisterRule(domain.TypeSlack, accountName, slackConfig.AllowedRoles)
			logger.Infof("Registered auth rule for Slack account '%s' - allowed roles: %v", accountName, slackConfig.AllowedRoles)
		}
	}

	// Register Ntfy authorization rules
	for accountName, ntfyConfig := range cfg.Notifiers.Ntfy {
		if len(ntfyConfig.AllowedRoles) > 0 {
			authz.RegisterRule(domain.TypeNtfy, accountName, ntfyConfig.AllowedRoles)
			logger.Infof("Registered auth rule for Ntfy account '%s' - allowed roles: %v", accountName, ntfyConfig.AllowedRoles)
		}
	}
}

func getDefaultConfig() *config.Config {
	return &config.Config{
		Server: config.ServerConfig{
			GRPCPort: 50051,
			RESTPort: 8080,
			Host:     "0.0.0.0",
			Mode:     "both",
		},
		Queue: domain.QueueConfig{
			Type:          "local",
			WorkerCount:   5,
			RetryAttempts: 3,
			Local: &domain.LocalQueueConfig{
				BufferSize: 1000,
			},
		},
		Notifiers: config.NotifiersConfig{
			Stdout: true,
		},
	}
}
