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

		if e, a := 0, runner.Run(ctx); e != a {
			t.Errorf("expected exit code returned from run to be %d, got %d", e, a)
		}

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

		if e, a := 0, runner.Run(ctx); e != a {
			t.Errorf("expected exit code returned from run to be %d, got %d", e, a)
		}

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

		if e, a := 1, runner.Run(ctx); e != a {
			t.Errorf("expected exit code returned from run to be %d, got %d", e, a)
		}

		errActivity.validate(t, false)
		longLived.validate(t, true)
	})

	t.Run("OneErrorOneLongLivedButHangs", func(t *testing.T) {
		t.Parallel()

		errActivity := NewActivity("error", func(_ context.Context) error {
			return errors.New("error")
		})

		longLived := NewActivity("longLived", func(_ context.Context) error {
			halt := make(chan struct{})
			<-halt // never return
			return nil
		})

		// Make a context that will die after a second.
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		t.Cleanup(cancel)

		runner := service.NewRunner(logrus.StandardLogger(), timeout, timeout)
		runner.RegisterActivities(errActivity, longLived)

		if e, a := 1, runner.Run(ctx); e != a {
			t.Errorf("expected exit code returned from run to be %d, got %d", e, a)
		}

		errActivity.validate(t, false)
		longLived.validate(t, false)
	})

	t.Run("OneShortLivedOneLongLived", func(t *testing.T) {
		t.Parallel()

		shortLived := NewActivity("error", nil)
		longLived := NewActivity("longLived", func(_ context.Context) error {
			return nil
		})

		// Make a context that will die after a second.
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		t.Cleanup(cancel)

		runner := service.NewRunner(logrus.StandardLogger(), timeout, timeout)
		runner.RegisterActivities(shortLived, longLived)

		if e, a := 0, runner.Run(ctx); e != a {
			t.Errorf("expected exit code returned from run to be %d, got %d", e, a)
		}

		shortLived.validate(t, false)
		longLived.validate(t, false)
	})
}

type fastActivity struct {
	name string
}

func (a *fastActivity) Name() string {
	return a.name
}

func (a *fastActivity) Run(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

func (*fastActivity) Shutdown(context.Context) error {
	return nil
}

func (*fastActivity) Kill() error {
	return nil
}

func TestServiceRunDoesNotWaitForKillTimeoutWhenKillReturns(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	runner := service.NewRunner(logrus.StandardLogger(), time.Second, 2*time.Second)
	runner.RegisterActivities(&fastActivity{name: "fast"})

	start := time.Now()
	if e, a := 0, runner.Run(ctx); e != a {
		t.Errorf("expected exit code returned from run to be %d, got %d", e, a)
	}

	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("runner waited for kill timeout after Kill returned; elapsed=%s", elapsed)
	}
}

type controlledActivity struct {
	name            string
	runStarted      chan struct{}
	runRelease      chan struct{}
	runReturned     chan struct{}
	shutdownStarted chan struct{}
	killStarted     chan struct{}
	killRelease     chan struct{}
	killReturned    chan struct{}
}

func newControlledActivity(name string) *controlledActivity {
	return &controlledActivity{
		name:            name,
		runStarted:      make(chan struct{}),
		runRelease:      make(chan struct{}),
		runReturned:     make(chan struct{}),
		shutdownStarted: make(chan struct{}),
		killStarted:     make(chan struct{}),
		killRelease:     make(chan struct{}),
		killReturned:    make(chan struct{}),
	}
}

func (a *controlledActivity) Name() string {
	return a.name
}

func (a *controlledActivity) Run(context.Context) error {
	defer close(a.runReturned)
	close(a.runStarted)
	<-a.runRelease
	return nil
}

func (a *controlledActivity) Shutdown(context.Context) error {
	close(a.shutdownStarted)
	return nil
}

func (a *controlledActivity) Kill() error {
	defer close(a.killReturned)
	close(a.killStarted)
	<-a.killRelease
	return nil
}

func TestServiceRunWaitsForBothKillAndRunToReturn(t *testing.T) {
	t.Run("KillReturnsBeforeRun", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		activity := newControlledActivity("kill-before-run")
		runner := service.NewRunner(logrus.StandardLogger(), time.Second, time.Second)
		runner.RegisterActivities(activity)

		done := runRunnerAsync(ctx, runner)
		<-activity.runStarted
		cancel()
		<-activity.shutdownStarted
		<-activity.killStarted

		close(activity.killRelease)
		<-activity.killReturned
		assertRunnerNotDone(t, done, "runner returned before Run returned")

		close(activity.runRelease)
		assertRunnerExitCode(t, done, 0)
	})

	t.Run("RunReturnsBeforeKill", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		activity := newControlledActivity("run-before-kill")
		runner := service.NewRunner(logrus.StandardLogger(), time.Second, time.Second)
		runner.RegisterActivities(activity)

		done := runRunnerAsync(ctx, runner)
		<-activity.runStarted
		cancel()
		<-activity.shutdownStarted
		<-activity.killStarted

		close(activity.runRelease)
		<-activity.runReturned
		assertRunnerNotDone(t, done, "runner returned before Kill returned")

		close(activity.killRelease)
		assertRunnerExitCode(t, done, 0)
	})
}

func runRunnerAsync(ctx context.Context, runner *service.Runner) <-chan int {
	done := make(chan int, 1)
	go func() {
		done <- runner.Run(ctx)
	}()
	return done
}

func assertRunnerNotDone(t *testing.T, done <-chan int, message string) {
	t.Helper()

	select {
	case exitCode := <-done:
		t.Fatalf("%s, exitCode=%d", message, exitCode)
	case <-time.After(50 * time.Millisecond):
	}
}

func assertRunnerExitCode(t *testing.T, done <-chan int, expected int) {
	t.Helper()

	select {
	case actual := <-done:
		if expected != actual {
			t.Fatalf("expected exit code returned from run to be %d, got %d", expected, actual)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("runner did not return")
	}
}
