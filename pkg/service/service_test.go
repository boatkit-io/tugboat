package service_test

import (
	"context"
	"testing"

	"github.com/boatkit-io/tugboat/pkg/service"
)

type Activity struct {
	invoked bool
}

func (a *Activity) Run(_ context.Context) error {
	a.invoked = true
	return nil
}

func (a *Activity) Close(_ context.Context) error {
	return nil
}

func TestServiceRun(t *testing.T) {
	one, two := &Activity{}, &Activity{}

	activities := []service.Activity{one, two}
	if err := service.Run(context.Background(), activities...); err != nil {
		t.Errorf("error from running activities: %v", err)
	}

	if !one.invoked || !two.invoked {
		t.Error("expected activity to have been invoked, was not")
	}
}
