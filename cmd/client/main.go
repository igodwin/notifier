package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/igodwin/notifier/pkg/client"
)

func main() {
	// Command
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "send":
		cmdSend(os.Args[2:])
	case "status":
		cmdStatus(os.Args[2:])
	case "list":
		cmdList(os.Args[2:])
	case "stats":
		cmdStats(os.Args[2:])
	case "notifiers":
		cmdNotifiers(os.Args[2:])
	case "health":
		cmdHealth(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Print(`Notifier Client - CLI for sending notifications

Usage:
  client <command> [options]

Commands:
  send       Send a notification
  status     Get notification status
  list       List notifications
  stats      Get notification statistics
  notifiers  List available notifiers
  health     Check service health

Global Options:
  --url      Service URL (default: http://localhost:8080)
  --key      API key for authentication (optional)
  --timeout  Request timeout (default: 30s)

Examples:
  # Send email notification
  client send --type email --subject "Alert" --body "System down" --recipients user@example.com

  # Check notification status
  client status --id <notification-id>

  # List recent notifications
  client list --limit 10

  # Get service stats
  client stats

  # Check health
  client health --url http://localhost:8080
`)
}

func cmdSend(args []string) {
	fs := flag.NewFlagSet("send", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Print(`Send a notification

Usage:
  client send [options]

Options:
  --url          Service URL (default: http://localhost:8080)
  --key          API key (optional)
  --type         Notification type (stdout, email, slack, ntfy) - required
  --subject      Subject line
  --body         Message body - required
  --account      Account name (optional, uses default)
  --recipients   Comma-separated recipients
  --timeout      Request timeout (default: 30s)
`)
	}

	baseURL := fs.String("url", "http://localhost:8080", "")
	apiKey := fs.String("key", "", "")
	timeout := fs.Duration("timeout", 30*time.Second, "")
	notifType := fs.String("type", "", "")
	subject := fs.String("subject", "", "")
	body := fs.String("body", "", "")
	account := fs.String("account", "", "")
	recipients := fs.String("recipients", "", "")

	fs.Parse(args)

	if *notifType == "" || *body == "" {
		fmt.Fprintf(os.Stderr, "Error: --type and --body are required\n")
		fs.Usage()
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	cfg := client.ClientConfig{
		BaseURL:     *baseURL,
		APIKey:      *apiKey,
		Timeout:     *timeout,
		TLSInsecure: false,
	}

	c := client.NewRESTClient(cfg)

	recipientList := []string{}
	if *recipients != "" {
		recipientList = strings.Split(*recipients, ",")
		for i := range recipientList {
			recipientList[i] = strings.TrimSpace(recipientList[i])
		}
	}

	req := client.NotificationRequest{
		Type:       *notifType,
		Subject:    *subject,
		Body:       *body,
		Account:    *account,
		Recipients: recipientList,
	}

	resp, err := c.Send(ctx, req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	data, _ := json.MarshalIndent(resp, "", "  ")
	fmt.Println(string(data))
}

func cmdStatus(args []string) {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Print(`Get notification status

Usage:
  client status [options]

Options:
  --url       Service URL (default: http://localhost:8080)
  --key       API key (optional)
  --id        Notification ID - required
  --timeout   Request timeout (default: 30s)
`)
	}

	baseURL := fs.String("url", "http://localhost:8080", "")
	apiKey := fs.String("key", "", "")
	timeout := fs.Duration("timeout", 30*time.Second, "")
	id := fs.String("id", "", "")

	fs.Parse(args)

	if *id == "" {
		fmt.Fprintf(os.Stderr, "Error: --id is required\n")
		fs.Usage()
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	cfg := client.ClientConfig{
		BaseURL:     *baseURL,
		APIKey:      *apiKey,
		Timeout:     *timeout,
		TLSInsecure: false,
	}

	c := client.NewRESTClient(cfg)

	notif, err := c.GetNotification(ctx, *id)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	data, _ := json.MarshalIndent(notif, "", "  ")
	fmt.Println(string(data))
}

func cmdList(args []string) {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Print(`List notifications

Usage:
  client list [options]

Options:
  --url       Service URL (default: http://localhost:8080)
  --key       API key (optional)
  --type      Filter by type (comma-separated)
  --status    Filter by status (comma-separated)
  --limit     Limit results (default: 10)
  --offset    Offset (default: 0)
  --timeout   Request timeout (default: 30s)
`)
	}

	baseURL := fs.String("url", "http://localhost:8080", "")
	apiKey := fs.String("key", "", "")
	timeout := fs.Duration("timeout", 30*time.Second, "")
	filterType := fs.String("type", "", "")
	filterStatus := fs.String("status", "", "")
	limit := fs.Int("limit", 10, "")
	offset := fs.Int("offset", 0, "")

	fs.Parse(args)

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	cfg := client.ClientConfig{
		BaseURL:     *baseURL,
		APIKey:      *apiKey,
		Timeout:     *timeout,
		TLSInsecure: false,
	}

	c := client.NewRESTClient(cfg)

	filter := client.ListNotificationsRequest{
		Limit:  *limit,
		Offset: *offset,
	}

	if *filterType != "" {
		filter.Types = strings.Split(*filterType, ",")
	}

	if *filterStatus != "" {
		statuses := strings.Split(*filterStatus, ",")
		for _, s := range statuses {
			filter.Statuses = append(filter.Statuses, client.NotificationStatus(strings.TrimSpace(s)))
		}
	}

	resp, err := c.ListNotifications(ctx, filter)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	data, _ := json.MarshalIndent(resp, "", "  ")
	fmt.Println(string(data))
}

func cmdStats(args []string) {
	fs := flag.NewFlagSet("stats", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Print(`Get notification statistics

Usage:
  client stats [options]

Options:
  --url       Service URL (default: http://localhost:8080)
  --key       API key (optional)
  --timeout   Request timeout (default: 30s)
`)
	}

	baseURL := fs.String("url", "http://localhost:8080", "")
	apiKey := fs.String("key", "", "")
	timeout := fs.Duration("timeout", 30*time.Second, "")

	fs.Parse(args)

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	cfg := client.ClientConfig{
		BaseURL:     *baseURL,
		APIKey:      *apiKey,
		Timeout:     *timeout,
		TLSInsecure: false,
	}

	c := client.NewRESTClient(cfg)

	stats, err := c.GetStats(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	data, _ := json.MarshalIndent(stats, "", "  ")
	fmt.Println(string(data))
}

func cmdNotifiers(args []string) {
	fs := flag.NewFlagSet("notifiers", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Print(`List available notifiers

Usage:
  client notifiers [options]

Options:
  --url       Service URL (default: http://localhost:8080)
  --key       API key (optional)
  --timeout   Request timeout (default: 30s)
`)
	}

	baseURL := fs.String("url", "http://localhost:8080", "")
	apiKey := fs.String("key", "", "")
	timeout := fs.Duration("timeout", 30*time.Second, "")

	fs.Parse(args)

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	cfg := client.ClientConfig{
		BaseURL:     *baseURL,
		APIKey:      *apiKey,
		Timeout:     *timeout,
		TLSInsecure: false,
	}

	c := client.NewRESTClient(cfg)

	notifiers, err := c.GetNotifiers(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	data, _ := json.MarshalIndent(notifiers, "", "  ")
	fmt.Println(string(data))
}

func cmdHealth(args []string) {
	fs := flag.NewFlagSet("health", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Print(`Check service health

Usage:
  client health [options]

Options:
  --url       Service URL (default: http://localhost:8080)
  --timeout   Request timeout (default: 30s)
`)
	}

	baseURL := fs.String("url", "http://localhost:8080", "")
	timeout := fs.Duration("timeout", 30*time.Second, "")

	fs.Parse(args)

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	cfg := client.ClientConfig{
		BaseURL:     *baseURL,
		Timeout:     *timeout,
		TLSInsecure: false,
	}

	c := client.NewRESTClient(cfg)

	healthy, err := c.HealthCheck(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if healthy {
		fmt.Println("Service is healthy")
		os.Exit(0)
	} else {
		fmt.Println("Service is unhealthy")
		os.Exit(1)
	}
}
