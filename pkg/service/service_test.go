package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/boatkit-io/tugboat/pkg/service"
	"github.com/sirupsen/logrus"
)

type Activity struct {
	ran      bool
	shutdown bool
	killed   bool

	runContextCancelledByCaller bool

	name    string
	runFunc func(context.Context) error
}

func NewActivity(name string, runFunc func(context.Context) error) *Activity {
	return &Activity{
		name:    name,
		runFunc: runFunc,
	}
}

func (a *Activity) Name() string {
	return a.name
}

func (a *Activity) Run(ctx context.Context) error {
	a.ran = true

	if a.runFunc != nil {
		if err := a.runFunc(ctx); err != nil {
			return err
		}
	}

	if ctx.Err() == context.Canceled {
		a.runContextCancelledByCaller = true
	}

	return nil
}

func (a *Activity) Shutdown(_ context.Context) error {
	a.shutdown = true

	halt := make(chan struct{})
	<-halt

	return nil
}

func (a *Activity) Kill() error {
	a.killed = true
	return nil
}

func (a *Activity) validate(t *testing.T, runContextCanceledByCaller bool) {
	if !a.ran {
		t.Errorf("expected %q run function to be invoked, was not", a.Name())
	}

	if !a.shutdown {
		t.Errorf("expected %q shutdown function to be invoked, was not", a.Name())
	}

	if !a.killed {
		t.Errorf("expected %q kill function to be invoked, was not", a.Name())
	}

	if e, a := a.runContextCancelledByCaller, runContextCanceledByCaller; e != a {
		t.Errorf("expected value of %t for runContextCanceledByCaller, got %t", e, a)
	}
}

func TestServiceRun(t *testing.T) {
	timeout := time.Millisecond * 500 // using same timeout for kill and shutdown

	t.Run("AllRunSuccesfullyShortLived", func(t *testing.T) {
		t.Parallel()

		one, two := NewActivity("one", nil), NewActivity("two", nil)

		// Make a context that will die after a second.
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		t.Cleanup(cancel)

		runner := service.NewRunner(logrus.StandardLogger(), timeout, timeout)
		runner.RegisterActivities(one, two)
		runner.Run(ctx)

		one.validate(t, false)
		two.validate(t, false)
	})

	t.Run("AllRunSuccesfullyLongLived", func(t *testing.T) {
		t.Parallel()

		runFunc := func(ctx context.Context) error {
			<-ctx.Done()
			return nil
		}
		one, two := NewActivity("one", runFunc), NewActivity("two", runFunc)

		// Make a context that will die after a second.
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		t.Cleanup(cancel)

		runner := service.NewRunner(logrus.StandardLogger(), timeout, timeout)
		runner.RegisterActivities(one, two)
		runner.Run(ctx)

		// The cancel error is different than the one that happens when the timeout
		// hits for a context, hence false being passed as the second parameter.
		one.validate(t, false)
		two.validate(t, false)
	})

	t.Run("OneErrorOneLongLived", func(t *testing.T) {
		t.Parallel()

		errActivity := NewActivity("error", func(_ context.Context) error {
			return errors.New("error")
		})

		longLived := NewActivity("longLived", func(ctx context.Context) error {
			<-ctx.Done() // block until the context is cancelled by the
			return nil
		})

		// Make a context that will die after a second.
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		t.Cleanup(cancel)

		runner := service.NewRunner(logrus.StandardLogger(), timeout, timeout)
		runner.RegisterActivities(errActivity, longLived)
		runner.Run(ctx)

		errActivity.validate(t, false)
		longLived.validate(t, true)
	})

	t.Run("OneErrorOneLongLivedButHangs", func(t *testing.T) {
		t.Parallel()

		errActivity := NewActivity("error", func(_ context.Context) error {
			return errors.New("error")
		})

		longLived := NewActivity("longLived", func(ctx context.Context) error {
			halt := make(chan struct{})
			<-halt // never return
			return nil
		})

		// Make a context that will die after a second.
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		t.Cleanup(cancel)

		runner := service.NewRunner(logrus.StandardLogger(), timeout, timeout)
		runner.RegisterActivities(errActivity, longLived)
		runner.Run(ctx)

		errActivity.validate(t, false)
		longLived.validate(t, false)
	})

	t.Run("OneShortLivedOneLongLived", func(t *testing.T) {
		t.Parallel()

		shortLived := NewActivity("error", nil)
		longLived := NewActivity("longLived", func(ctx context.Context) error {
			return nil
		})

		// Make a context that will die after a second.
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		t.Cleanup(cancel)

		runner := service.NewRunner(logrus.StandardLogger(), timeout, timeout)
		runner.RegisterActivities(shortLived, longLived)
		runner.Run(ctx)

		shortLived.validate(t, false)
		longLived.validate(t, false)
	})
}
