package main

import (
	"fmt"
	"net/http"

	"go.tickamp.dev/lifecycle"
)

// MyHTTPServer is a simple HTTP server.
type MyHTTPServer struct {
	*lifecycle.Worker

	server *http.Server
}

// NewHTTPServer creates a new HTTP server.
func NewHTTPServer(addr string) *MyHTTPServer {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(rw http.ResponseWriter, req *http.Request) {
		rw.Write([]byte("Hello!"))
	})
	server := http.Server{Addr: addr, Handler: mux}
	return &MyHTTPServer{
		Worker: lifecycle.NewWorkerWithOptions(&lifecycle.Hooks{
			Start:     lifecycle.DropContext(server.ListenAndServe),
			Shutdown:  server.Shutdown,
			Terminate: lifecycle.DropContext(server.Close),
			Error: func(event lifecycle.Event) error {
				if event.Error == http.ErrServerClosed {
					return nil
				}
				return event.Error
			},
		}, &lifecycle.ServiceOptions{
			Logger: simpleLogger{},
		}),
		server: &server,
	}
}

func main() {
	NewHTTPServer(":8080").Start()
}

type simpleLogger struct{}

func (s simpleLogger) Info(msg string, keysAndValues ...interface{}) {
	fmt.Println(msg, keysAndValues)
}

func (s simpleLogger) Error(err error, msg string,
	keysAndValues ...interface{}) {
	fmt.Println(msg, append(keysAndValues, "error", err))
}
