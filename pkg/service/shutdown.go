package service

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

// Ensure that Shutdown implements the Activity interface.
var _ Activity = &Shutdown{}

// Shutdown is a helper type that implements the Activity interface for services to
// use for graceful shutdown. Use the NewShutdown factory function to generate an
// instance of this.
type Shutdown struct {
	// mainCancel is passed in to NewShutdown and it should be the cancelFunc that
	// communicates to everything that its time to stop (gracefully).
	mainCancel context.CancelFunc
	done       chan struct{}
	doneOnce   sync.Once
}

// NewShutdown returns a new Shutdown type that implements the Activity interface to
// facilitate graceful shutdown for services. The context.cancelFunc passed to this
// should come from the context that all other Activitys the service is using has passed
// to them.
func NewShutdown(mainCancel context.CancelFunc) *Shutdown {
	return &Shutdown{
		mainCancel: mainCancel,
		done:       make(chan struct{}),
	}
}

// Name returns the name of the Shutdown Activity.
func (*Shutdown) Name() string {
	return "shutdown"
}

// Run blocks until an os.Interrupt or syscall.SIGTERM signal is received, or the context
// is canceled.
func (s *Shutdown) Run(ctx context.Context) error {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(c)

	select {
	case <-c:
		s.mainCancel()
	case <-ctx.Done():
		// even though 99% of the time this will belong to the context passed
		// to shutdown, this doesn't hurt anything.
		s.mainCancel()
	}

	<-s.done
	return nil
}

// Shutdown is a no-op, but it implements the interface necessary for Activity.
func (*Shutdown) Shutdown(_ context.Context) error {
	return nil
}

// Kill releases Run after shutdown has started.
func (s *Shutdown) Kill() error {
	s.doneOnce.Do(func() {
		close(s.done)
	})
	return nil
}
