package notifier_test

import (
	"bytes"
	"os"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/igodwin/notifier/internal/config"
	"github.com/igodwin/notifier/internal/notifier"
)

const expectedNotificationMessage = "this is a test notification message"

var (
	stdoutNotifier *notifier.StdoutNotifier
	cfg            config.StdoutConfig
	buffer         *bytes.Buffer
	reader         *os.File
	writer         *os.File
	originalStdout *os.File
)

var _ = Describe("StdoutNotifier", func() {
	BeforeEach(func() {
		cfg = config.StdoutConfig{}
		stdoutNotifier, _ = notifier.NewStdoutNotifier(cfg)

		buffer = &bytes.Buffer{}
		reader, writer, _ = os.Pipe()
		os.Stdout = writer
		originalStdout = os.Stdout
		os.Stdout = writer
	})

	It("should output the correct message to stdout", func() {
		testNotification := notifier.Notification{Message: expectedNotificationMessage}
		Expect(stdoutNotifier.Send(testNotification)).To(Succeed())
		resetStdout()

		_, _ = buffer.ReadFrom(reader)
		Expect(buffer.String()).To(ContainSubstring(expectedNotificationMessage))
	})

	It("should not output anything if message is empty", func() {
		testNotification := notifier.Notification{}
		Expect(stdoutNotifier.Send(testNotification)).To(Succeed())
		resetStdout()

		_, _ = buffer.ReadFrom(reader)
		Expect(strings.TrimSpace(buffer.String())).To(BeEmpty())
	})
})

func resetStdout() {
	Expect(writer.Close()).To(Succeed())
	os.Stdout = originalStdout
}
