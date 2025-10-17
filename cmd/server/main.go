package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/igodwin/notifier/api/rest"
	"github.com/igodwin/notifier/internal/config"
	"github.com/igodwin/notifier/internal/domain"
	"github.com/igodwin/notifier/internal/notifier"
	"github.com/igodwin/notifier/internal/queue"
	"github.com/igodwin/notifier/internal/service"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	// Load configuration
	cfg, err := config.Load("")
	if err != nil {
		log.Printf("Warning: failed to load config, using defaults: %v", err)
		cfg = getDefaultConfig()
	}

	log.Printf("Starting Notifier Service in mode: %s", cfg.Server.Mode)

	// Create context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize queue
	var q domain.Queue
	if cfg.Queue.Type == "local" {
		q, err = queue.NewLocalQueue(cfg.Queue.Local)
		if err != nil {
			log.Fatalf("Failed to create queue: %v", err)
		}
		log.Println("Using local queue")
	} else {
		log.Fatalf("Queue type %s not implemented yet", cfg.Queue.Type)
	}

	// Initialize notifier factory and register notifiers
	factory := notifier.NewFactory()
	registerNotifiers(cfg, factory)

	// Check if any notifiers are registered
	if len(factory.SupportedTypes()) == 0 {
		log.Fatal("No notifiers configured. Please enable at least one notifier in config.yaml")
	}

	log.Printf("Supported notification types: %v", factory.SupportedTypes())

	// Create notification service
	svc := service.NewNotificationService(factory, q, cfg.Queue.WorkerCount)

	// Start workers
	if err := svc.Start(ctx); err != nil {
		log.Fatalf("Failed to start service: %v", err)
	}
	log.Printf("Started %d worker(s)", cfg.Queue.WorkerCount)

	// Wait group for both servers
	var wg sync.WaitGroup

	// Start gRPC server if enabled
	var grpcServer *grpc.Server
	if cfg.Server.Mode == "both" || cfg.Server.Mode == "grpc" {
		wg.Add(1)
		grpcServer = startGRPCServer(ctx, &wg, cfg, svc)
	}

	// Start REST server if enabled
	var restServer *http.Server
	if cfg.Server.Mode == "both" || cfg.Server.Mode == "rest" {
		wg.Add(1)
		restServer = startRESTServer(ctx, &wg, cfg, svc)
	}

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down servers...")

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// Stop REST server
	if restServer != nil {
		if err := restServer.Shutdown(shutdownCtx); err != nil {
			log.Printf("Error during REST server shutdown: %v", err)
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
		log.Printf("Error stopping service: %v", err)
	}

	log.Println("Servers stopped")
}

func registerNotifiers(cfg *config.Config, factory *notifier.Factory) {
	if cfg.Notifiers.Stdout {
		stdoutNotifier := notifier.NewStdoutNotifier()
		if err := factory.RegisterNotifier(domain.TypeStdout, stdoutNotifier); err != nil {
			log.Fatalf("Failed to register stdout notifier: %v", err)
		}
		log.Println("Registered stdout notifier")
	}

	if cfg.Notifiers.SMTP != nil {
		smtpNotifier, err := notifier.NewSMTPNotifier(cfg.Notifiers.SMTP)
		if err != nil {
			log.Printf("Warning: failed to create SMTP notifier: %v", err)
		} else {
			if err := factory.RegisterNotifier(domain.TypeEmail, smtpNotifier); err != nil {
				log.Fatalf("Failed to register SMTP notifier: %v", err)
			}
			log.Println("Registered SMTP notifier")
		}
	}

	if cfg.Notifiers.Slack != nil {
		slackNotifier, err := notifier.NewSlackNotifier(cfg.Notifiers.Slack)
		if err != nil {
			log.Printf("Warning: failed to create Slack notifier: %v", err)
		} else {
			if err := factory.RegisterNotifier(domain.TypeSlack, slackNotifier); err != nil {
				log.Fatalf("Failed to register Slack notifier: %v", err)
			}
			log.Println("Registered Slack notifier")
		}
	}

	if cfg.Notifiers.Ntfy != nil {
		ntfyNotifier, err := notifier.NewNtfyNotifier(cfg.Notifiers.Ntfy)
		if err != nil {
			log.Printf("Warning: failed to create Ntfy notifier: %v", err)
		} else {
			if err := factory.RegisterNotifier(domain.TypeNtfy, ntfyNotifier); err != nil {
				log.Fatalf("Failed to register Ntfy notifier: %v", err)
			}
			log.Println("Registered Ntfy notifier")
		}
	}
}

func startGRPCServer(ctx context.Context, wg *sync.WaitGroup, cfg *config.Config, svc domain.NotificationService) *grpc.Server {
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.GRPCPort)

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Failed to listen on %s: %v", addr, err)
	}

	grpcServer := grpc.NewServer()

	// TODO: Register gRPC service implementation when protobuf is generated
	// pb.RegisterNotifierServiceServer(grpcServer, grpcHandler)

	// Enable reflection for tools like grpcurl
	reflection.Register(grpcServer)

	go func() {
		defer wg.Done()
		log.Printf("gRPC server listening on %s", addr)
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("Failed to serve gRPC: %v", err)
		}
	}()

	return grpcServer
}

func startRESTServer(ctx context.Context, wg *sync.WaitGroup, cfg *config.Config, svc domain.NotificationService) *http.Server {
	router := rest.NewRouter(svc)

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
		log.Printf("REST server listening on %s", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start REST server: %v", err)
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
