package canbus

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/brutella/can"
	"github.com/pkg/errors"
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

	// Use netlink to make sure the interface is up
	link, err := netlink.LinkByName(c.options.InterfaceName)
	if err != nil {
		return fmt.Errorf("no link found for %v: %v", c.options.InterfaceName, err)
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
			cmd := exec.CommandContext(ctx, "ip", "link", "set", c.options.InterfaceName, "down")
			if output, err := cmd.Output(); err != nil {
				logBase := c.log.WithField("cmd", strings.Join(cmd.Args, " ")).WithField("output", string(output))
				if errCast, worked := err.(*exec.ExitError); worked {
					logBase = logBase.WithField("stderr", string(errCast.Stderr))
				}
				logBase.Error("Ip link set down failed")
				return err
			}

			// Re-fetch info
			link, err = netlink.LinkByName(c.options.InterfaceName)
			if err != nil {
				return fmt.Errorf("no link found for %v: %v", c.options.InterfaceName, err)
			}

			canLink = link.(*netlink.Can)
		}
	}

	if canLink.Attrs().OperState == netlink.OperDown {
		c.log.WithField("canName", c.options.InterfaceName).WithField("bitRate", c.options.BitRate).Info("Link is down, bringing up link")

		// ip link set can1 up type can bitrate 250000
		cmd := exec.CommandContext(ctx, "ip", "link", "set", c.options.InterfaceName, "up", "type", "can", "bitrate",
			strconv.Itoa(int(c.options.BitRate)))
		if output, err := cmd.Output(); err != nil {
			logBase := c.log.WithField("cmd", strings.Join(cmd.Args, " ")).WithField("output", string(output))
			if errCast, worked := err.(*exec.ExitError); worked {
				logBase = logBase.WithField("stderr", string(errCast.Stderr))
			}
			logBase.Error("Ip link set up failed")
			return err
		}
	}

	// Open the brutella can bus
	bus, err := can.NewBusForInterfaceWithName(c.options.InterfaceName)
	if err != nil {
		return err
	}

	c.bus = bus
	c.busHandler = can.NewHandler(c.options.MessageHandler)
	c.bus.Subscribe(c.busHandler)

	c.log.WithField("interfaceName", c.options.InterfaceName).
		Info("Opened SocketCAN and listening")

	// Start listening for messages
	return bus.ConnectAndPublish()
}

// Close shuts down the channel
func (c *SocketCANChannel) Close() error {
	if c.bus == nil {
		return nil
	}

	c.bus.Unsubscribe(c.busHandler)
	if err := c.bus.Disconnect(); err != nil {
		return errors.Wrap(err, "close underlying bus connection")
	}

	return nil
}

// WriteFrame will send a CAN frame to the channel
func (c *SocketCANChannel) WriteFrame(frame can.Frame) error {
	return c.bus.Publish(frame)
}
