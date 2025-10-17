package notifier

import (
	"context"
	"fmt"
	"time"

	"github.com/igodwin/notifier/internal/domain"
)

// StdoutNotifier sends notifications to stdout (useful for debugging)
type StdoutNotifier struct {
	BaseNotifier
}

// NewStdoutNotifier creates a new stdout notifier
func NewStdoutNotifier() *StdoutNotifier {
	return &StdoutNotifier{
		BaseNotifier: BaseNotifier{
			notificationType: domain.TypeStdout,
		},
	}
}

// Send sends a notification to stdout
func (s *StdoutNotifier) Send(ctx context.Context, notification *domain.Notification) (*domain.NotificationResult, error) {
	if err := ValidateContext(ctx); err != nil {
		return nil, err
	}

	if err := s.Validate(notification); err != nil {
		return nil, err
	}

	fmt.Println("========================================")
	fmt.Printf("Notification ID: %s\n", notification.ID)
	fmt.Printf("Type: %s\n", notification.Type)
	fmt.Printf("Priority: %d\n", notification.Priority)
	fmt.Printf("Recipients: %v\n", notification.Recipients)
	fmt.Printf("Subject: %s\n", notification.Subject)
	fmt.Printf("Body:\n%s\n", notification.Body)
	fmt.Println("========================================")

	return &domain.NotificationResult{
		NotificationID: notification.ID,
		Success:        true,
		Message:        "Notification printed to stdout",
		SentAt:         time.Now(),
	}, nil
}
