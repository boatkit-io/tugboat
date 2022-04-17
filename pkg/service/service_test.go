package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/boatkit-io/tugboat/pkg/service"
	"github.com/sirupsen/logrus"
)

type Activity struct {
	invoked  bool
	shutdown bool
	killed   bool
}

func (*Activity) Name() string {
	return "activity"
}

func (a *Activity) Run(_ context.Context) error {
	a.invoked = true
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

func TestServiceRun(t *testing.T) {
	one, two := &Activity{}, &Activity{}
	timeout := time.Millisecond * 500 // using same timeout for kill and shutdown

	// Make a context that will die after a second.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	t.Cleanup(cancel)

	runner := service.NewRunner(logrus.StandardLogger(), timeout, timeout, one, two)
	runner.Run(ctx)

	if !one.invoked || !two.invoked {
		t.Error("expected all activities to have been invoked, were not")
	}

	if !one.shutdown || !two.shutdown {
		t.Error("expected all activities to have been shutdown, were not")
	}

	if !one.killed || !two.killed {
		t.Error("expected all activities to have been killed, were not")
	}
}
