// Package lifecycle provides simple service management primitives.
//
// A typical HTTP server could look like:
//
//     type MyHttpServer struct {
//         server http.Server
//     }
//
//     func NewHttpServer() *MyHttpServer {
//         mux := http.NewServeMux()
//            mux.HandleFunc("/", func(rw http.ResponseWriter, req *http.Request) {
//                rw.Write([]byte("Hello!"))
//            })
//            server := http.Server{Addr: ":8090", Handler: mux}
//            return &MyHttpServer{
//               server: server,
//            }
//     }
//
//     func (m *MyHttpServer) Start() error {
//         return m.server.ListenAndServe()
//     }
//
// This is pretty simple code for simple projects, but lacks powerful state
// management. To add signal handling, graceful shutdown with automatic
// termination after a certain delay or readiness probes, one would need to
// write boulerplate code.
//
// Using lifecycle, you can modify your code like:
//
//     type MyHttpServer struct {
//         *lifecycle.Worker
//         server http.Server
//     }
//
//     func NewHttpServer() *MyHttpServer {
//         mux := http.NewServeMux()
//         mux.HandleFunc("/", func(rw http.ResponseWriter, req *http.Request) {
//             rw.Write([]byte("Hello!"))
//         })
//         server := http.Server{Addr: ":8090", Handler: mux}
//         return &MyHttpServer{
//             Worker: lifecycle.NewWorker(&lifecycle.Hooks{
//                 Start:     lifecycle.DropContext(server.ListenAndServe),
//                 Shutdown:  server.Shutdown,
//                 Terminate: lifecycle.DropContext(server.Close),
//             }),
//             server: server,
//         }
//     }
//
//     // No need to add Start, Stop and other lifecycle controlling methods,
//     // which are inherited from Worker.
//
// Out of the box, this provides you with:
//
//     • Start, Stop and Terminate methods
//     • StartBackground, providing a non-blocking alternative
//     • Graceful termination timeout, terminating the service
//     • Error handling and filtering, for example to ignore http.ErrServerClosed
//     • Logging by providing your logger
//     • Readiness probes
//     • Observers to listen to the service state changes and errors
//
// This package also provides a Host, which wraps multiple workers into a single
// startable unit exposing the same API.
package lifecycle
