package logging

import (
	"io"
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var Log *Logger

func init() {
	Log = NewLogger(&LogOptions{
		Level:  "info",
		Format: "text",
	})
}

// LogOptions configures the behavior of the logging system.
type LogOptions struct {
	Level  string // Log level (e.g., "debug", "info", "warn", "error") default "info"
	Format string // Format is the output format of the logs (e.g., "json", "text")
}

func Info(msg string, keyValues ...interface{}) {
	Log.LogInfo(msg, keyValues...)
}

func Debug(msg string, keyValues ...interface{}) {
	Log.LogDebug(msg, keyValues...)
}

func Warn(msg string, keyValues ...interface{}) {
	Log.LogWarn(msg, keyValues...)
}

func Error(err error, msg string, keyValues ...interface{}) {
	Log.LogError(err, msg, keyValues...)
}

func Fatal(err error, msg string, keyValues ...interface{}) {
	Log.LogFatal(err, msg, keyValues...)
}

// Logger is a wrapper around zerolog.Logger providing a high-performance,
// configuration-driven logging utility.
type Logger struct {
	logger zerolog.Logger
}

// NewLogger initializes and returns a new Logger instance based on the provided LogOptions.
// It configures the log level, output format (JSON/Console), and adds a timestamp.
func NewLogger(opts *LogOptions) *Logger {

	level, err := zerolog.ParseLevel(opts.Level)
	if err != nil {
		level = zerolog.InfoLevel
		log.Warn().Str("config_level", opts.Level).Msg("Invalid log level configured, defaulting to Info.")
	}

	// Set the global level. This is crucial for zerolog's performance optimization:
	// log calls below this level will be zero-allocated and skipped quickly.
	zerolog.SetGlobalLevel(level)

	var output io.Writer

	if opts.Format == "json" {
		output = os.Stdout
	} else {
		output = zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: "2006-01-02 15:04:05", // Custom time format for readability
		}
	}

	return &Logger{
		logger: zerolog.New(output).With().Timestamp().Logger(),
	}
}

// LogDebug records a debugging message.
// This method is designed for zero-overhead in production environments
// where the global log level is set higher (e.g., Info, Warn, or Disabled).
func (l Logger) LogDebug(msg string, keyValues ...interface{}) {
	// Crucial Performance Check: Check if Debug level is enabled before building the log event.
	// If not enabled, the method returns immediately, avoiding memory allocation
	// and processing of keyValues.
	if l.logger.Debug().Enabled() {
		// Use Fields() to support the variadic keyValues structure (key1, val1, key2, val2...)
		l.logger.Debug().Fields(keyValues).Msg(msg)
	}
}

// LogInfo records informational messages about normal application flow.
func (l Logger) LogInfo(msg string, keyValues ...interface{}) {
	l.logger.Info().Fields(keyValues).Msg(msg)
}

// LogWarn records messages about potential issues that do not immediately stop the application.
func (l Logger) LogWarn(msg string, keyValues ...interface{}) {
	l.logger.Warn().Fields(keyValues).Msg(msg)
}

// LogError records messages about failures, including an explicit error object.
func (l Logger) LogError(err error, msg string, keyValues ...interface{}) {
	// .Err(err) adds the error object to the structured log
	l.logger.Error().Err(err).Fields(keyValues).Msg(msg)
}

// LogFatal records a critical error and then exits the application with os.Exit(1).
func (l Logger) LogFatal(err error, msg string, keyValues ...interface{}) {
	l.logger.Fatal().Err(err).Fields(keyValues).Msg(msg)
}
