# Lifecycle

[![GoDoc reference example](https://img.shields.io/badge/godoc-reference-blue.svg)](https://pkg.go.dev/go.tickamp.dev/lifecycle)
[![Build](https://github.com/tickamp/lifecycle/workflows/CI/badge.svg)](https://github.com/tickamp/lifecycle/actions?query=workflow%3ACI+branch%3Amaster)
[![GitHub go.mod Go version of a Go module](https://img.shields.io/github/go-mod/go-version/gomods/athens.svg)](https://github.com/tickamp/lifecycle)
[![Go Report Card](https://goreportcard.com/badge/go.tickamp.dev/lifecycle)](https://goreportcard.com/report/go.tickamp.dev/lifecycle)

Package lifecycle provides simple service management primitives.

A typical HTTP server could look like:

```go
type MyHttpServer struct {
    server http.Server
}

func NewHttpServer() *MyHttpServer {
    mux := http.NewServeMux()
    mux.HandleFunc("/", func(rw http.ResponseWriter, req *http.Request) {
        rw.Write([]byte("Hello!"))
    })
    server := http.Server{Addr: ":8090", Handler: mux}
    return &MyHttpServer{
        server: server,
    }
}
func (m *MyHttpServer) Start() error {
    return m.server.ListenAndServe()
}
```

This is pretty simple code for simple projects, but lacks powerful state
management. To add signal handling, graceful shutdown with automatic
termination after a certain delay or readiness probes, one would need to
write boulerplate code.

Using lifecycle, you can modify this code to look like:

```go
type MyHttpServer struct {
    *lifecycle.Worker
    server http.Server
}

func NewHttpServer() *MyHttpServer {
    mux := http.NewServeMux()
    mux.HandleFunc("/", func(rw http.ResponseWriter, req *http.Request) {
        rw.Write([]byte("Hello!"))
    })
    server := http.Server{Addr: ":8090", Handler: mux}
    return &MyHttpServer{
        Worker: lifecycle.NewWorker(&lifecycle.Hooks{
            Start:     lifecycle.DropContext(server.ListenAndServe),
            Shutdown:  server.Shutdown,
            Terminate: lifecycle.DropContext(server.Close),
        }),
        server: server,
    }
}

// No need to add Start, Stop and other lifecycle controlling methods, which are
// inherited from Worker.
```

Out of the box, this provides:

- Start, Stop and Terminate methods
- StartBackground, providing a non-blocking alternative
- Graceful termination timeout, terminating the service
- Error handling and filtering, for example to ignore http.ErrServerClosed
- Logging by providing your logger
- Readiness probes
- Observers to listen to the service state changes and errors

This package also provides a Host, which wraps multiple workers into a single
startable unit exposing the same API.

## Getting started

Install this package with:

```
go get -u go.tickamp.dev/lifecycle
```

The [HTTP example](https://github.com/tickamp/lifecycle/tree/master/examples/http) provides a good place to get started with simple use-cases. You can fund additional documentation in the package [Godoc](https://goreportcard.com/report/go.tickamp.dev/lifecycle).

## Building

This package is built with [Bazel](https://bazel.build). You can build it with:

```
bazel build //...
```

Standard go commands should be usable as well.

---

Released under the [MIT License](https://github.com/tickamp/lifecycle/blob/master/LICENSE).
