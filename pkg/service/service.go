// Package service contains various server utilities.
package service

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

// Activity represents a service activity to run.
type Activity interface {
	Run(context.Context) error
	Close(context.Context) error
}

// Run runs all of the provided types that implement the Activity
// interface.
func Run(ctx context.Context, activities ...Activity) error {
	g, ctx := errgroup.WithContext(ctx)

	for i := range activities {
		activity := activities[i]
		g.Go(func() error {
			defer activity.Close(ctx)
			return activity.Run(ctx)
		})
	}

	if err := g.Wait(); err != nil {
		return errors.Wrap(err, "run service activities")
	}

	return nil
}

// Shutdown is a helper type that implements the Activity interface for services to
// use for graceful shutdown. Use the NewShutdown factory function to generate an
// instance of this.
type Shutdown struct {
	// mainCancel is passed in to NewShutdown and it should be the cancelFunc that
	// communicates to everything that its time to stop (gracefully).
	mainCancel context.CancelFunc
}

// NewShutdown returns a new Shutdown type that implements the Activity interface to
// facilitate graceful shutdown for services. The context.cancelFunc passed to this
// should come from the context that all other Activitys the service is using has passed
// to them.
func NewShutdown(mainCancel context.CancelFunc) *Shutdown {
	return &Shutdown{
		mainCancel: mainCancel,
	}
}

// Run blocks until an os.Interrupt or syscall.SIGTERM signal is recieved, or the context
// is canceled.
func (s *Shutdown) Run(ctx context.Context) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	select {
	case <-c:
		signal.Reset(os.Interrupt, syscall.SIGTERM)
		s.mainCancel()
	case <-ctx.Done():
		// even though 99% of the time this will belong to the context passed
		// to shutdown, this doesn't hurt anything.
		s.mainCancel()
	}
}

// Close closes the Shutdown Activity.
func (*Shutdown) Close(_ context.Context) error {
	return nil
}
