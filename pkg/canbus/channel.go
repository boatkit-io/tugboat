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

// Channel represents a single canbus channel for sending/receiving CAN frames
type Channel struct {
	bitRate        int32
	ChannelOptions ChannelOptions

	bus        *can.Bus
	busHandler can.Handler

	log *logrus.Logger
}

// NewChannel returns a Channel object based on the given options.  ChannelOptions are required settings, and then you can optionally add
// more ChannelOption objects for various optional options.
func NewChannel(log *logrus.Logger, ChannelOptions ChannelOptions, opts ...ChannelOption) *Channel {
	c := Channel{
		ChannelOptions: ChannelOptions,
		log:            log,
	}

	// Apply defaults
	c.bitRate = DefaultBitRate

	// Apply functional options.
	for i := range opts {
		opts[i](&c)
	}

	return &c
}

// Run opens the canbus channel and starts listening.  This will also, as needed, use netlink to actually call into the OS
// to start the channel and/or set the bitrate, as needed.
func (c *Channel) Run(ctx context.Context) error {
	// Referencing https://github.com/angelodlfrtr/go-can/blob/master/transports/socketcan.go

	// Use netlink to make sure the interface is up
	link, err := netlink.LinkByName(c.ChannelOptions.CanInterfaceName)
	if err != nil {
		return fmt.Errorf("no link found for %v: %v", c.ChannelOptions.CanInterfaceName, err)
	}

	if link.Type() != "can" {
		return fmt.Errorf("invalid linktype %q", link.Type())
	}

	canLink := link.(*netlink.Can)

	if canLink.Attrs().OperState == netlink.OperUp {
		if canLink.BitRate != uint32(c.bitRate) {
			c.log.WithField("bitRate", canLink.BitRate).Info("Channel currentl has wrong bitrate, bringing down")

			cmd := exec.CommandContext(ctx, "ip", "link", "set", c.ChannelOptions.CanInterfaceName, "down")
			if output, err := cmd.Output(); err != nil {
				logBase := c.log.WithField("cmd", strings.Join(cmd.Args, " ")).WithField("output", string(output))
				if errCast, worked := err.(*exec.ExitError); !worked {
					logBase = logBase.WithField("stderr", string(errCast.Stderr))
				}
				logBase.Error("Ip link set down failed")
				return err
			}

			// Re-fetch info
			link, err = netlink.LinkByName(c.ChannelOptions.CanInterfaceName)
			if err != nil {
				return fmt.Errorf("no link found for %v: %v", c.ChannelOptions.CanInterfaceName, err)
			}

			canLink = link.(*netlink.Can)
		}
	}

	if canLink.Attrs().OperState == netlink.OperDown {
		c.log.WithField("canName", c.ChannelOptions.CanInterfaceName).WithField("bitRate", c.bitRate).Info("Link is down, bringing up link")

		// ip link set can1 up type can bitrate 250000
		cmd := exec.CommandContext(ctx, "ip", "link", "set", c.ChannelOptions.CanInterfaceName, "up", "type", "can", "bitrate", strconv.Itoa(int(c.bitRate)))
		if output, err := cmd.Output(); err != nil {
			logBase := c.log.WithField("cmd", strings.Join(cmd.Args, " ")).WithField("output", string(output))
			if errCast, worked := err.(*exec.ExitError); !worked {
				logBase = logBase.WithField("stderr", string(errCast.Stderr))
			}
			logBase.Error("Ip link set up failed")
			return err
		}

		// TODO(ddr): Someday figure out how the hell to make the netlink stuff work

		// fmt.Printf("CanAttrs: %+v\n", *canLink)
		// canLink.BitRate = 250000

		// if err := netlink.LinkModify(canLink); err != nil {
		// 	return err
		// }

		// fmt.Println("Modified link")

		// if err := netlink.LinkSetUp(canLink); err != nil {
		// 	return err
		// }
	}

	// Open the brutella can bus
	bus, err := can.NewBusForInterfaceWithName(c.ChannelOptions.CanInterfaceName)
	if err != nil {
		return err
	}

	c.bus = bus
	c.busHandler = can.NewHandler(c.ChannelOptions.MessageHandler)
	c.bus.Subscribe(c.busHandler)

	c.log.WithField("canName", c.ChannelOptions.CanInterfaceName).
		Info("opened connection and listening")

	// Start listening for messages
	return bus.ConnectAndPublish()
}

// Close shuts down the channel
func (c *Channel) Close() error {
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
func (c *Channel) WriteFrame(frame can.Frame) error {
	return c.bus.Publish(frame)
}
