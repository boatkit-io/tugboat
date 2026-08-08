package canbus

import (
	"context"

	"github.com/brutella/can"
)

// Interface is a basic interface for a CANbus implementation
type Interface interface {
	// Start synchronously completes startup before the long-running read loop begins.
	Start(ctx context.Context) error
	Run(ctx context.Context) error
	Close() error
	WriteFrame(frame can.Frame) error
}
