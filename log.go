package lifecycle

import "fmt"

type Logger interface {
	Info(msg string, keysAndValues ...interface{})
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
