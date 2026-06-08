package canbus

import (
	"context"
	stderrors "errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"

	"github.com/brutella/can"
	pkgerrors "github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
)

// SocketCANChannelOptions is a type that contains required options on a SocketCANChannel.
type SocketCANChannelOptions struct {
	InterfaceName        string
	BitRate              int
	ForceBounceInterface bool
	MessageHandler       can.HandlerFunc
}

// SocketCANChannel represents a single canbus channel for sending/receiving CAN frames
type SocketCANChannel struct {
	options SocketCANChannelOptions

	bus        *can.Bus
	busHandler can.Handler

	log *logrus.Logger

	mu     sync.Mutex
	closed bool
}

// NewSocketCANChannel returns a Channel object based on SocketCAN and the given options.  ChannelOptions are required settings.
func NewSocketCANChannel(log *logrus.Logger, options SocketCANChannelOptions) *SocketCANChannel {
	c := SocketCANChannel{
		options: options,
		log:     log,
	}

	return &c
}

// Run opens the canbus channel and starts listening.  This will also, as needed, use netlink to actually call into the OS
// to start the channel and/or set the bitrate, as needed.
func (c *SocketCANChannel) Run(ctx context.Context) error {
	// Referencing https://github.com/angelodlfrtr/go-can/blob/master/transports/socketcan.go
	if c.isClosed() {
		return nil
	}

	// Use netlink to make sure the interface is up
	link, err := netlink.LinkByName(c.options.InterfaceName)
	if err != nil {
		return fmt.Errorf("no link found for %v: %w", c.options.InterfaceName, err)
	}

	if link.Type() != "can" {
		return fmt.Errorf("invalid linktype %q", link.Type())
	}

	canLink := link.(*netlink.Can)

	if canLink.Attrs().OperState == netlink.OperUp {
		bounce := false
		if canLink.BitRate != uint32(c.options.BitRate) {
			c.log.WithField("bitRate", canLink.BitRate).Info("Channel currently has wrong bitrate, bringing down")
			bounce = true
		} else if c.options.ForceBounceInterface {
			c.log.Info("Bouncing channel")
			bounce = true
		}

		if bounce {
			cmd := exec.CommandContext(ctx, "ip", "link", "set", c.options.InterfaceName, "down") // #nosec G204 -- interface name is argv only.
			if output, err := cmd.Output(); err != nil {
				logBase := c.log.WithField("cmd", strings.Join(cmd.Args, " ")).WithField("output", string(output))
				var exitErr *exec.ExitError
				if stderrors.As(err, &exitErr) {
					logBase = logBase.WithField("stderr", string(exitErr.Stderr))
				}
				logBase.Error("Ip link set down failed")
				return err
			}

			// Re-fetch info
			link, err = netlink.LinkByName(c.options.InterfaceName)
			if err != nil {
				return fmt.Errorf("no link found for %v: %w", c.options.InterfaceName, err)
			}

			canLink = link.(*netlink.Can)
		}
	}

	if canLink.Attrs().OperState == netlink.OperDown {
		c.log.WithField("canName", c.options.InterfaceName).WithField("bitRate", c.options.BitRate).Info("Link is down, bringing up link")

		// ip link set can1 up type can bitrate 250000
		args := []string{"ip", "link", "set", c.options.InterfaceName, "up", "type", "can", "bitrate", strconv.Itoa(c.options.BitRate)}
		cmd := exec.CommandContext(ctx, args[0], args[1:]...) // #nosec G204 -- interface name is argv only.
		if output, err := cmd.Output(); err != nil {
			logBase := c.log.WithField("cmd", strings.Join(cmd.Args, " ")).WithField("output", string(output))
			var exitErr *exec.ExitError
			if stderrors.As(err, &exitErr) {
				logBase = logBase.WithField("stderr", string(exitErr.Stderr))
			}
			logBase.Error("Ip link set up failed")
			return err
		}
	}

	if c.isClosed() {
		return nil
	}

	// Open the brutella can bus
	bus, err := can.NewBusForInterfaceWithName(c.options.InterfaceName)
	if err != nil {
		return err
	}

	busHandler := can.NewHandler(c.options.MessageHandler)
	bus.Subscribe(busHandler)

	c.mu.Lock()
	closed := c.closed
	if !closed {
		c.bus = bus
		c.busHandler = busHandler
	}
	c.mu.Unlock()
	if closed {
		bus.Unsubscribe(busHandler)
		if err := bus.Disconnect(); err != nil && !isClosedCANBusError(err) {
			return pkgerrors.Wrap(err, "close underlying bus connection")
		}
		return nil
	}

	c.log.WithField("interfaceName", c.options.InterfaceName).
		Info("Opened SocketCAN and listening")

	// Start listening for messages
	if err := bus.ConnectAndPublish(); err != nil {
		if c.isClosed() && isClosedCANBusError(err) {
			return nil
		}
		return err
	}

	return nil
}

// Close shuts down the channel
func (c *SocketCANChannel) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	bus := c.bus
	busHandler := c.busHandler
	c.mu.Unlock()

	if bus == nil {
		return nil
	}

	if busHandler != nil {
		bus.Unsubscribe(busHandler)
	}
	if err := bus.Disconnect(); err != nil && !isClosedCANBusError(err) {
		return pkgerrors.Wrap(err, "close underlying bus connection")
	}

	return nil
}

// WriteFrame will send a CAN frame to the channel
func (c *SocketCANChannel) WriteFrame(frame can.Frame) error {
	c.mu.Lock()
	bus := c.bus
	closed := c.closed
	c.mu.Unlock()

	if closed || bus == nil {
		return stderrors.New("canbus channel is closed")
	}

	return bus.Publish(frame)
}

func (c *SocketCANChannel) isClosed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.closed
}

func isClosedCANBusError(err error) bool {
	if err == nil {
		return false
	}

	errMessage := err.Error()
	return strings.Contains(errMessage, "file already closed") ||
		strings.Contains(errMessage, "use of closed network connection")
}
