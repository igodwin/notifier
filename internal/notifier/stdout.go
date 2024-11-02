package notifier

import (
	"fmt"
	"github.com/igodwin/notifier/internal/config"
)

type StdoutNotifier struct {
	config config.StdoutConfig
}

func NewStdoutNotifier(cfg config.StdoutConfig) (*StdoutNotifier, error) {
	return &StdoutNotifier{config: cfg}, nil
}

func (s *StdoutNotifier) Send(notification Notification) error {
	fmt.Println(notification.Message)
	return nil
}
