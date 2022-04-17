// Package service contains various server utilities.
package service

import (
	"context"

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
		go func(activity Activity) {
			g.Go(func() error {
				defer activity.Close(ctx)
				return activity.Run(ctx)
			})
		}(activities[i])
	}

	if err := g.Wait(); err != nil {
		return errors.Wrap(err, "run service activities")
	}

	return nil
}
