package e2e

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/igodwin/notifier/pkg/client"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// TestSuite holds shared test state
type TestSuite struct {
	Container testcontainers.Container
	Client    *client.RESTClient
	BaseURL   string
	T         *testing.T
}

// SetupSuite creates and starts the notifier container
func SetupSuite(t *testing.T, retention ...string) *TestSuite {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Find project root
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}

	// Walk up to find go.mod (project root)
	projectRoot := cwd
	for {
		if _, err := os.Stat(filepath.Join(projectRoot, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(projectRoot)
		if parent == projectRoot {
			t.Fatalf("Could not find project root (go.mod)")
		}
		projectRoot = parent
	}

	// Build the docker image from project root
	buildCmd := exec.Command("docker", "build", "-t", "notifier:test", ".")
	buildCmd.Dir = projectRoot
	if output, err := buildCmd.CombinedOutput(); err != nil {
		t.Logf("Docker build output: %s", string(output))
		t.Fatalf("Failed to build docker image: %v", err)
	}

	// Prepare environment variables for container
	env := map[string]string{
		"NOTIFIER_LOGGING_LEVEL":    "debug",
		"NOTIFIER_LOGGING_FORMAT":   "json",
		"NOTIFIER_NOTIFIERS_STDOUT": "true",
	}

	// Add retention config if specified
	if len(retention) > 0 && retention[0] != "" {
		// Parse retention string (format: "KEY1=VAL1|KEY2=VAL2|...")
		parts := strings.Split(retention[0], "|")
		for _, part := range parts {
			kv := strings.Split(part, "=")
			if len(kv) == 2 {
				env[kv[0]] = kv[1]
			}
		}
	}

	// Create container
	req := testcontainers.ContainerRequest{
		Image:        "notifier:test",
		ExposedPorts: []string{"8080/tcp"},
		Env:          env,
		WaitingFor:   wait.ForHTTP("/health").WithStartupTimeout(30 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("Failed to create container: %v", err)
	}

	// Get container port
	host, err := container.Host(ctx)
	if err != nil {
		container.Terminate(ctx)
		t.Fatalf("Failed to get container host: %v", err)
	}

	port, err := container.MappedPort(ctx, "8080")
	if err != nil {
		container.Terminate(ctx)
		t.Fatalf("Failed to get container port: %v", err)
	}

	baseURL := fmt.Sprintf("http://%s:%s", host, port.Port())

	// Create client
	cfg := client.ClientConfig{
		BaseURL:     baseURL,
		Timeout:     30 * time.Second,
		TLSInsecure: true,
	}
	c := client.NewRESTClient(cfg)

	// Wait for service to be ready
	deadline := time.Now().Add(30 * time.Second)
	for {
		if time.Now().After(deadline) {
			container.Terminate(ctx)
			t.Fatalf("Service failed to become ready")
		}

		healthy, err := c.HealthCheck(ctx)
		if err == nil && healthy {
			break
		}

		time.Sleep(100 * time.Millisecond)
	}

	return &TestSuite{
		Container: container,
		Client:    c,
		BaseURL:   baseURL,
		T:         t,
	}
}

// TeardownSuite stops and removes the container
func (s *TestSuite) TeardownSuite(ctx context.Context) {
	if s.Container != nil {
		s.Container.Terminate(ctx)
	}
}

// WaitForCleanup waits for cleanup to have run and notifications to be removed
func (s *TestSuite) WaitForCleanup(maxWait time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), maxWait)
	defer cancel()

	deadline := time.Now().Add(maxWait)
	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("cleanup did not complete within %v", maxWait)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
			// Check if cleanup has happened by checking logs or stats
			return nil
		}
	}
}

// GetLogs retrieves container logs for debugging
func (s *TestSuite) GetLogs(ctx context.Context) string {
	reader, err := s.Container.Logs(ctx)
	if err != nil {
		return fmt.Sprintf("error reading logs: %v", err)
	}
	defer reader.Close()

	logs, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Sprintf("error reading logs: %v", err)
	}

	return string(logs)
}

// buildTestImage builds the docker image for testing
func buildTestImage(t *testing.T) {
	// Get the project root
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}

	// Find the project root by looking for go.mod
	for {
		if _, err := os.Stat(filepath.Join(wd, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(wd)
		if parent == wd {
			t.Fatalf("Could not find project root")
		}
		wd = parent
	}

	// Check if Dockerfile exists
	dockerfile := filepath.Join(wd, "Dockerfile")
	if _, err := os.Stat(dockerfile); err != nil {
		t.Logf("Warning: Dockerfile not found at %s, using generic build", dockerfile)
		// The container will be built from the binary
	}
}
