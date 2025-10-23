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

	// Initialize notifier factory and register notifiers
	factory := notifier.NewFactory()
	registerNotifiers(cfg, factory, logger)

	// Check if any notifiers are registered
	if len(factory.SupportedTypes()) == 0 {
		logger.Fatal("No notifiers configured. Please enable at least one notifier in notifier.config")
	}

	logger.Infof("Supported notification types: %v", factory.SupportedTypes())

	// Create notification service (pass config as account resolver)
	svc := service.NewNotificationService(factory, q, cfg.Queue.WorkerCount, cfg, logger)

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
		grpcServer = startGRPCServer(ctx, &wg, cfg, svc, logger)
	}

	// Start REST server if enabled
	var restServer *http.Server
	if cfg.Server.Mode == "both" || cfg.Server.Mode == "rest" {
		wg.Add(1)
		restServer = startRESTServer(ctx, &wg, cfg, svc, logger)
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

func startGRPCServer(ctx context.Context, wg *sync.WaitGroup, cfg *config.Config, svc domain.NotificationService, logger *logging.Logger) *grpc.Server {
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.GRPCPort)

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		logger.Fatalf("Failed to listen on %s: %v", addr, err)
	}

	grpcServer := grpc.NewServer()

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

func startRESTServer(ctx context.Context, wg *sync.WaitGroup, cfg *config.Config, svc domain.NotificationService, logger *logging.Logger) *http.Server {
	router := rest.NewRouter(svc, logger)

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
