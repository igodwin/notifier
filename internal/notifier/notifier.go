package notifier

import (
	"fmt"

	. "github.com/igodwin/notifier/internal/config"
)

type Notifier interface {
	Send(notification Notification) error
}

type Notification struct {
	Recipient string
	Message   string
}

func NewNotifier(mode string, config interface{}) (Notifier, error) {
	switch mode {
	case "ntfy":
		return NewNtfyNotifier(config.(NtfyConfig))
	case "smtp":
		return NewSMTPNotifier(config.(SMTPConfig))
	case "stdout":
		return NewStdoutNotifier(config.(StdoutConfig))
	// case "slack":
	// 	return NewSlackNotifier(config.(SlackConfig))
	// Other notification modes can be added here
	default:
		return nil, fmt.Errorf("unsupported notification mode: %s", mode)
	}
}
