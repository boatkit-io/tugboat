// Package service contains various server utilities.
package service

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
)

// Activity represents a service activity to run.
type Activity interface {
	Name() string
	Run(context.Context) error
	Shutdown(context.Context) error
	Kill() error
}

// Runner runs Activitys.
type Runner struct {
	activities      []Activity
	shutdownTimeout time.Duration
	killTimeout     time.Duration
	wg              *sync.WaitGroup
	log             *logrus.Logger
}

// NewRunner returns a newly configured runner.
func NewRunner(log *logrus.Logger, shutdownTimeout, killTimeout time.Duration) *Runner {
	return &Runner{
		shutdownTimeout: shutdownTimeout,
		killTimeout:     killTimeout,
		wg:              &sync.WaitGroup{},
		log:             log,
	}
}

// RegisterActivities registers any number of activities on the receiver to be ran using
// Run.
func (r *Runner) RegisterActivities(activities ...Activity) {
	if len(activities) > 0 {
		r.activities = append(r.activities, activities...)
	}
}

// Run runs all of the provided activities. Run is a blocking function.
func (r *Runner) Run(ctx context.Context) {
	for i := range r.activities {
		activity := r.activities[i]

		r.wg.Add(1) // Done is deferred in *Runner.runActivity.
		go r.runActivity(ctx, activity)
	}

	// Block until all activities are done.
	r.wg.Wait()
}

// runActivity runs a single activity. It is expected that *Runner.wg.Add(1) is called
// before calling this function.
func (r *Runner) runActivity(ctx context.Context, activity Activity) {
	defer r.wg.Done() // LIFO ensures this will be called last.

	alog := r.log.WithField("name", activity.Name())

	runReturned := make(chan struct{})
	go func() {
		defer close(runReturned)
		if err := activity.Run(ctx); err != nil {
			alog.WithError(err).Error("run activity")
		}
	}()

	// Block until the main context has been canceled or run has returned.
	select {
	case <-ctx.Done():
	case <-runReturned:
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), r.shutdownTimeout)
	defer cancel()

	shutdownReturned := make(chan struct{})
	go func() {
		defer close(shutdownReturned)
		if err := activity.Shutdown(shutdownCtx); err != nil {
			alog.WithError(err).Error("shutdown activity")
		}
	}()

	// Block until we've either hit our shutdown timeout or shutdown has returned.
	select {
	case <-shutdownCtx.Done():
	case <-shutdownReturned:
	}

	killCtx, cancel := context.WithTimeout(context.Background(), r.killTimeout)
	defer cancel()

	go func() {
		if err := activity.Kill(); err != nil {
			alog.WithError(err).Error("kill activity")
		}
	}()

	// Block until we've hit our kill timeout.
	<-killCtx.Done()
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

// Name returns the name of the Shutdown Activity.
func (*Shutdown) Name() string {
	return "shutdown"
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

// Shutdown is a no-op, but it implements the interface necessary for Activity.
func (*Shutdown) Shutdown(_ context.Context) error {
	return nil
}

// Kill is a no-op, but it implements the interface necessary for Activity.
func (*Shutdown) Kill() error {
	return nil
}
