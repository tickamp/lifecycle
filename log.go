package lifecycle

import "fmt"

// Logger is a simple logger interface accepting key-value pair parameters.
type Logger interface {
	// Logs an info message.
	Info(msg string, keysAndValues ...interface{})
	// Logs an error.
	Error(err error, msg string, keysAndValues ...interface{})
}

type simpleLogger struct{}

func (s simpleLogger) Info(msg string, keysAndValues ...interface{}) {
	fmt.Println(msg, keysAndValues)
}

func (s simpleLogger) Error(err error, msg string,
	keysAndValues ...interface{}) {
	fmt.Println(msg, append(keysAndValues, "error", err))
}
