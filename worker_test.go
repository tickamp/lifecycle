package lifecycle

import (
	"context"
	"errors"
	"os"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestWorkerShutdown(t *testing.T) {
	s := newTestWorker("worker", 0, time.Second, nil)
	assert.NoError(t, s.doStart())
	assert.Equal(t, Started, s.State())
	<-s.Ready()
	assert.NoError(t, s.Shutdown())
	assert.Equal(t, []State{Starting, Started, ShuttingDown, Stopped},
		s.ObserverEventSequence())
	<-s.Ready()
	<-s.Done()
}

func TestWorkerShutdownTimeout(t *testing.T) {
	s := newTestWorker("worker", 10*time.Second, 50*time.Millisecond, nil)
	assert.NoError(t, s.doStart())
	assert.Equal(t, Started, s.State())
	assert.NoError(t, s.Shutdown())
	assert.Equal(t,
		[]State{Starting, Started, ShuttingDown, Terminating, Stopped},
		s.ObserverEventSequence())
}

func TestWorkerTerminate(t *testing.T) {
	s := newTestWorker("worker", 0, time.Second, nil)
	assert.NoError(t, s.doStart())
	assert.Equal(t, Started, s.State())
	assert.NoError(t, s.Terminate())
	assert.Equal(t, []State{Starting, Started, Terminating, Stopped},
		s.ObserverEventSequence())
}

func TestWorkerSignal(t *testing.T) {
	s := newTestWorker("worker", 0, time.Second, nil)
	assert.NoError(t, s.doStart())
	assert.Equal(t, Started, s.State())
	syscall.Kill(syscall.Getpid(), syscall.SIGUSR2)
	assert.Equal(t, []State{Starting, Started, ShuttingDown, Stopped},
		s.ObserverEventSequence())
}

func TestWorkerSignalShutdownTimeout(t *testing.T) {
	s := newTestWorker("worker", 10*time.Second, 50*time.Millisecond, nil)
	assert.NoError(t, s.doStart())
	assert.Equal(t, Started, s.State())
	syscall.Kill(syscall.Getpid(), syscall.SIGUSR2)
	assert.Equal(t,
		[]State{Starting, Started, ShuttingDown, Terminating, Stopped},
		s.ObserverEventSequence())
}

func TestWorkerExitingNoError(t *testing.T) {
	s := newTestWorker("worker", 0, time.Second, nil)
	assert.NoError(t, s.doStart())
	assert.Equal(t, Started, s.State())
	s.interrupt(nil)
	assert.Equal(t, []State{Starting, Started, Stopped},
		s.ObserverEventSequence())
}

func TestWorkerExitingWithError(t *testing.T) {
	s := newTestWorker("worker", 0, time.Second, nil)
	assert.NoError(t, s.doStart())
	assert.Equal(t, Started, s.State())
	s.interrupt(errors.New("oops"))
	assert.Equal(t, []State{Starting, Started, Error},
		s.ObserverEventSequence())
}

func TestWorkerExitingWithIgnoredError(t *testing.T) {
	s := newTestWorker("worker", 0, time.Second, nil)
	assert.NoError(t, s.doStart())
	assert.Equal(t, Started, s.State())
	s.interrupt(errors.New("ignore"))
	assert.Equal(t, []State{Starting, Started, Stopped},
		s.ObserverEventSequence())
}

func TestReadinessProbe(t *testing.T) {
	s := newTestWorker("worker", 0, time.Second, func() <-chan error {
		ch := make(chan error)
		go func() {
			<-time.After(5 * time.Millisecond)
			close(ch)
		}()
		return ch
	})
	assert.NoError(t, s.doStart())
	assert.Equal(t, Started, s.State())
	assert.NoError(t, s.Shutdown())
	assert.Equal(t, []State{Starting, Started, ShuttingDown, Stopped},
		s.ObserverEventSequence())
}

func TestReadinessProbeError(t *testing.T) {
	s := newTestWorker("worker", 0, time.Second, func() <-chan error {
		ch := make(chan error)
		go func() {
			<-time.After(5 * time.Millisecond)
			ch <- errors.New("oops")
			close(ch)
		}()
		return ch
	})
	assert.EqualError(t, s.doStart(), "oops")
	assert.Equal(t, Error, s.State())
	assert.Equal(t, []State{Starting, Error}, s.ObserverEventSequence())
}

// Event observer
type eventObserver struct {
	events []Event
	ch     chan Event
	wg     sync.WaitGroup
}

func newEventObserver() *eventObserver {
	ch := make(chan Event)

	e := &eventObserver{
		ch: ch,
	}

	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		for event := range ch {
			e.events = append(e.events, event)
		}
	}()

	return e
}

func (e *eventObserver) ObserverChan() chan<- Event {
	return e.ch
}

func (e *eventObserver) ObserverEvents() []Event {
	e.wg.Wait()
	return e.events
}

func (e *eventObserver) ObserverEventSequence() []State {
	events := e.ObserverEvents()
	res := make([]State, len(events))
	for i, event := range events {
		res[i] = event.To
	}
	return res
}

type testWorker struct {
	*Worker
	*eventObserver
	interrupt func(err error)
}

func newTestWorker(name string, shutdownDelay time.Duration,
	shutdownTimeout time.Duration,
	readinessProbe func() <-chan error) *testWorker {
	ch := make(chan error, 1)

	var closeOnce sync.Once
	closeCh := func(err error) {
		closeOnce.Do(func() {
			if err != nil {
				ch <- err
			}
			close(ch)
		})
	}

	s := &testWorker{
		Worker: NewWorkerWithOptions(
			&Hooks{
				Name: name,
				Start: func(ctx context.Context) error {
					err := <-ch
					return err
				},
				Shutdown: func(ctx context.Context) error {
					<-time.After(shutdownDelay)
					closeCh(nil)
					return nil
				},
				Terminate: func(ctx context.Context) error {
					closeCh(nil)
					return nil
				},
				Error: func(event Event) error {
					if event.Error != nil && event.Error.Error() == "ignore" {
						return nil
					}
					return event.Error
				},
			},
			&ServiceOptions{
				ReadinessProbe:  readinessProbe,
				ShutdownTimeout: shutdownTimeout,
				Logger:          simpleLogger{},
				Signals:         []os.Signal{syscall.SIGUSR2},
			},
		),
		eventObserver: newEventObserver(),
		interrupt:     closeCh,
	}
	s.Observe(s.ObserverChan())

	return s
}

func (s *testWorker) doStart() error {
	ch := make(chan error)
	go func() {
		ch <- s.Start()
		close(ch)
	}()

	select {
	case err := <-ch:
		return err
	case <-time.After(50 * time.Millisecond):
	}

	return nil
}
