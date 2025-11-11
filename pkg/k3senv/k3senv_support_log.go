package k3senv

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/testcontainers/testcontainers-go"
	tclog "github.com/testcontainers/testcontainers-go/log"

	"k8s.io/utils/ptr"
)

var (
	// emojiPattern matches common emoji Unicode ranges.
	emojiPattern = regexp.MustCompile(`[\x{1F300}-\x{1F9FF}\x{2600}-\x{26FF}\x{2700}-\x{27BF}]`)
)

// loggerConsumer forwards testcontainer logs to the k3senv Logger.
type loggerConsumer struct {
	logger Logger
}

func (lc *loggerConsumer) Accept(log testcontainers.Log) {
	if lc.logger != nil {
		message := strings.TrimSpace(string(log.Content))
		if message != "" {
			lc.logger.Logf("[k3s] %s", message)
		}
	}
}

// testcontainersLogger implements the testcontainers log.Logger interface
// to forward testcontainers lifecycle logs without emojis to the k3senv Logger.
type testcontainersLogger struct {
	logger Logger
}

func (tcl *testcontainersLogger) Printf(format string, v ...any) {
	if tcl.logger == nil {
		return
	}

	message := fmt.Sprintf(format, v...)
	message = emojiPattern.ReplaceAllString(message, "")
	message = strings.TrimSpace(message)

	if message != "" {
		tcl.logger.Logf("[testcontainers] %s", message)
	}
}

// noopLogger is a logger that discards all log messages.
type noopLogger struct {
}

func (noopLogger) Printf(format string, v ...any) {
}

// configureTestcontainersLogger sets up the global testcontainers logger.
// WARNING: This modifies global state and affects all testcontainers in this process.
// The global logger is set based on the Logging.Enabled configuration:
// - If disabled: uses a no-op logger (suppresses all testcontainers lifecycle logs)
// - If enabled and Logger is set: forwards logs to Logger without emojis
// - If enabled and Logger is nil: uses a no-op logger.
func (e *K3sEnv) configureTestcontainersLogger() {
	if !ptr.Deref(e.options.Logging.Enabled, true) {
		tclog.SetDefault(noopLogger{})
		return
	}

	if e.options.Logger != nil {
		tclog.SetDefault(&testcontainersLogger{logger: e.options.Logger})
	} else {
		tclog.SetDefault(noopLogger{})
	}
}
