package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/boatkit-io/tugboat/pkg/service"
	"github.com/sirupsen/logrus"
)

type Activity struct {
	invoked bool
}

func (*Activity) Name() string {
	return "activity"
}

func (a *Activity) Run(_ context.Context) error {
	a.invoked = true
	return nil
}

func (*Activity) Shutdown(_ context.Context) error {
	return nil
}

func (*Activity) Kill() error {
	return nil
}

func TestServiceRun(t *testing.T) {
	one, two := &Activity{}, &Activity{}
	runner := service.NewRunner(logrus.StandardLogger(), time.Millisecond*500, one, two)
	runner.Run(context.Background())

	if !one.invoked || !two.invoked {
		t.Error("expected activity to have been invoked, was not")
	}
}
