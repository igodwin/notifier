package notifier

import (
	"fmt"
	"net/smtp"

	"github.com/igodwin/notifier/internal/config"
)

type SMTPNotifier struct {
	config config.SMTPConfig
}

func NewSMTPNotifier(config config.SMTPConfig) (*SMTPNotifier, error) {
	return &SMTPNotifier{config: config}, nil
}

func (s *SMTPNotifier) Send(notification Notification) error {
	addr := fmt.Sprintf("%s:%d", s.config.Server, s.config.Port)
	auth := smtp.PlainAuth("", s.config.Username, s.config.Password, s.config.Server)

	msg := []byte(fmt.Sprintf("To: %s\r\nSubject: Notification\r\n\r\n%s\r\n", notification.Recipient, notification.Message))

	err := smtp.SendMail(addr, auth, s.config.Username, []string{notification.Recipient}, msg)
	if err != nil {
		return fmt.Errorf("failed to send email: %v", err)
	}
	return nil
}
