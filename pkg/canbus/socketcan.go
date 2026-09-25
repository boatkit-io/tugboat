package canbus

import (
	"context"
	stderrors "errors"
	"fmt"
	"net"
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
	RestartMilliseconds  uint32
	ForceBounceInterface bool
	MessageHandler       can.HandlerFunc
}

// SocketCANChannel represents a single canbus channel for sending/receiving CAN frames
type SocketCANChannel struct {
	options SocketCANChannelOptions

	bus        *can.Bus
	busHandler can.Handler

	log *logrus.Logger

	startMu sync.Mutex
	mu      sync.Mutex
	closed  bool
}

type socketCANCommandRunner func(context.Context, []string) ([]byte, error)

type socketCANCommandError struct {
	err    error
	stderr string
}

func (e *socketCANCommandError) Error() string {
	return e.err.Error()
}

func (e *socketCANCommandError) Unwrap() error {
	return e.err
}

// NewSocketCANChannel returns a Channel object based on SocketCAN and the given options.  ChannelOptions are required settings.
func NewSocketCANChannel(log *logrus.Logger, options SocketCANChannelOptions) *SocketCANChannel {
	c := SocketCANChannel{
		options: options,
		log:     log,
	}

	return &c
}

// Start synchronously opens the CAN bus channel. This will also, as needed,
// use netlink to start the channel and set the bitrate.
func (c *SocketCANChannel) Start(ctx context.Context) error {
	c.startMu.Lock()
	defer c.startMu.Unlock()

	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return stderrors.New("SocketCAN channel is closed")
	}
	if c.bus != nil {
		c.mu.Unlock()
		return nil
	}
	c.mu.Unlock()

	// Referencing https://github.com/angelodlfrtr/go-can/blob/master/transports/socketcan.go

	// Use netlink to make sure the interface is up
	link, err := netlink.LinkByName(c.options.InterfaceName)
	if err != nil {
		return fmt.Errorf("no link found for %v: %w", c.options.InterfaceName, err)
	}

	if link.Type() != "can" && link.Type() != "vcan" {
		return fmt.Errorf("invalid linktype %q", link.Type())
	}

	var canLink *netlink.Can
	bounceReason := ""
	if link.Type() == "vcan" {
		if link.Attrs().OperState == netlink.OperDown {
			c.log.WithField("canName", c.options.InterfaceName).Info("vcan link is down, bringing up link")
			cmd := exec.CommandContext(ctx, "ip", "link", "set", c.options.InterfaceName, "up") // #nosec G204 -- interface name is argv only.
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
		goto linkReady
	}

	canLink = link.(*netlink.Can)

	bounceReason = socketCANBounceReason(canLink, c.options)
	if bounceReason != "" && socketCANLinkIsUp(canLink) {
		c.log.WithField("reason", bounceReason).Info("Bringing down SocketCAN interface")
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

	if !socketCANLinkIsUp(canLink) {
		c.log.WithField("canName", c.options.InterfaceName).WithField("bitRate", c.options.BitRate).Info("Link is down, bringing up link")

		if err := c.bringUpSocketCANLink(ctx, runSocketCANCommand); err != nil {
			return err
		}
	}

linkReady:

	if c.isClosed() {
		return stderrors.New("SocketCAN channel is closed")
	}

	// Open the brutella can bus
	bus, err := can.NewBusForInterfaceWithName(c.options.InterfaceName)
	if err != nil {
		return err
	}

	var busHandler can.Handler
	if c.options.MessageHandler != nil {
		busHandler = can.NewHandler(c.options.MessageHandler)
		bus.Subscribe(busHandler)
	}

	c.mu.Lock()
	closed := c.closed
	if !closed {
		c.bus = bus
		c.busHandler = busHandler
	}
	c.mu.Unlock()
	if closed {
		if busHandler != nil {
			bus.Unsubscribe(busHandler)
		}
		if err := bus.Disconnect(); err != nil && !isClosedCANBusError(err) {
			return pkgerrors.Wrap(err, "close underlying bus connection")
		}
		return stderrors.New("SocketCAN channel is closed")
	}

	c.log.WithField("interfaceName", c.options.InterfaceName).
		Info("Opened SocketCAN")

	return nil
}

func socketCANLinkIsUp(canLink *netlink.Can) bool {
	return canLink.Attrs().Flags&net.FlagUp != 0
}

func socketCANBounceReason(canLink *netlink.Can, options SocketCANChannelOptions) string {
	if canLink.State == netlink.CAN_STATE_BUS_OFF {
		return "interface is bus-off"
	}
	if canLink.BitRate != uint32(options.BitRate) {
		return fmt.Sprintf("bitrate is %d, expected %d", canLink.BitRate, options.BitRate)
	}
	if options.RestartMilliseconds > 0 && canLink.RestartMs != options.RestartMilliseconds {
		return fmt.Sprintf("restart delay is %d ms, expected %d ms", canLink.RestartMs, options.RestartMilliseconds)
	}
	if options.ForceBounceInterface {
		return "interface bounce was requested"
	}
	return ""
}

func socketCANLinkUpArgs(options SocketCANChannelOptions) []string {
	args := []string{
		"ip", "link", "set", options.InterfaceName, "up", "type", "can",
		"bitrate", strconv.Itoa(options.BitRate),
	}
	if options.RestartMilliseconds > 0 {
		args = append(args, "restart-ms", strconv.FormatUint(uint64(options.RestartMilliseconds), 10))
	}
	return args
}

func (c *SocketCANChannel) bringUpSocketCANLink(ctx context.Context, run socketCANCommandRunner) error {
	options := c.options
	args := socketCANLinkUpArgs(options)
	output, err := run(ctx, args)
	if err == nil {
		return nil
	}

	if options.RestartMilliseconds > 0 && socketCANAutomaticRestartUnsupported(err) {
		c.log.WithFields(logrus.Fields{
			"canName":             options.InterfaceName,
			"restartMilliseconds": options.RestartMilliseconds,
		}).Warn("SocketCAN interface does not support kernel-managed bus-off restart; bringing it up without a restart delay")
		options.RestartMilliseconds = 0
		args = socketCANLinkUpArgs(options)
		output, err = run(ctx, args)
		if err == nil {
			return nil
		}
	}

	logSocketCANCommandError(c.log, args, output, err, "Ip link set up failed")
	return err
}

func runSocketCANCommand(ctx context.Context, args []string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, args[0], args[1:]...) // #nosec G204 -- interface name is argv only.
	output, err := cmd.Output()
	if err == nil {
		return output, nil
	}

	var exitErr *exec.ExitError
	if stderrors.As(err, &exitErr) {
		return output, &socketCANCommandError{err: err, stderr: string(exitErr.Stderr)}
	}
	return output, err
}

func socketCANAutomaticRestartUnsupported(err error) bool {
	var commandErr *socketCANCommandError
	if !stderrors.As(err, &commandErr) {
		return false
	}
	return strings.Contains(strings.ToLower(commandErr.stderr), "support restart from bus off")
}

func logSocketCANCommandError(log *logrus.Logger, args []string, output []byte, err error, message string) {
	logBase := log.WithField("cmd", strings.Join(args, " ")).WithField("output", string(output))
	var commandErr *socketCANCommandError
	if stderrors.As(err, &commandErr) {
		logBase = logBase.WithField("stderr", commandErr.stderr)
	}
	logBase.Error(message)
}

// Run starts listening after synchronously opening the CAN bus channel.
func (c *SocketCANChannel) Run(ctx context.Context) error {
	if err := c.Start(ctx); err != nil {
		return err
	}

	c.mu.Lock()
	bus := c.bus
	closed := c.closed
	c.mu.Unlock()
	if closed || bus == nil {
		return nil
	}

	c.log.WithField("interfaceName", c.options.InterfaceName).
		Info("Listening on SocketCAN")

	// Start listening for messages
	err := bus.ConnectAndPublish()
	if c.isClosed() && isClosedCANBusError(err) {
		return nil
	}
	return err
}

var _ Interface = (*SocketCANChannel)(nil)

// Close shuts down the channel
func (c *SocketCANChannel) Close() error {
	c.startMu.Lock()
	defer c.startMu.Unlock()

	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	bus := c.bus
	busHandler := c.busHandler
	c.bus = nil
	c.busHandler = nil
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
