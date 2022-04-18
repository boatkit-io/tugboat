// Package service contains various server utilities.
package service

import (
	"context"
	"errors"
	"sync"
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

	// erroredSignal is closed whenever any Activity.Run has returned a non-nil error.
	erroredSignal chan struct{}

	// Since you cannot detect whether or not a channel is closed without reading
	// from it, this is set to true when errored has been closed. The reason this
	// is all necessary is because we need the errored channel in the runActivity
	// function so when it is closed it automatically sends on the select it's used
	// in, cancelling the wrapped Run context and telling all of the other Activitys
	// to stop running.
	errored bool
}

// NewRunner returns a newly configured runner.
func NewRunner(log *logrus.Logger, shutdownTimeout, killTimeout time.Duration) *Runner {
	return &Runner{
		shutdownTimeout: shutdownTimeout,
		killTimeout:     killTimeout,
		wg:              &sync.WaitGroup{},
		log:             log,
		erroredSignal:   make(chan struct{}),
	}
}

// RegisterActivities registers any number of activities on the receiver to be ran using
// Run.
func (r *Runner) RegisterActivities(activities ...Activity) {
	if len(activities) > 0 {
		r.activities = append(r.activities, activities...)
	}
}

// Run runs all of the provided activities. Run is a blocking function and returns an exit
// code.
func (r *Runner) Run(ctx context.Context) int {
	for i := range r.activities {
		activity := r.activities[i]

		r.wg.Add(1) // Done is deferred in *Runner.runActivity.
		go r.runActivity(ctx, activity)
	}

	// Block until all activities are done.
	r.wg.Wait()

	if r.errored {
		return 1
	}
	return 0
}

// runActivity runs a single activity. It is expected that *Runner.wg.Add(1) is called
// before calling this function.
func (r *Runner) runActivity(ctx context.Context, activity Activity) {
	defer r.wg.Done() // LIFO ensures this will be called last.

	alog := r.log.WithField("name", activity.Name())

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	runReturned := make(chan struct{})
	go func() {
		defer close(runReturned)
		if err := activity.Run(runCtx); err != nil {
			alog.WithError(err).Error("run activity")

			// If the errored channel hasn't already been closed and the error returned
			// wasn't a context.DeadlineExceeded and wasn't a context.Canceled then we
			// need to close the signaling channel to shut down all activities.
			if !r.errored &&
				!errors.Is(err, context.DeadlineExceeded) &&
				!errors.Is(err, context.Canceled) {
				r.errored = true
				close(r.erroredSignal)
			}
		}
	}()

	// Block until the main context has been canceled or run has returned.
	select {
	case <-ctx.Done():
	case <-runReturned:
	case <-r.erroredSignal:
		cancel()
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
