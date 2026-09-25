package canbus

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/brutella/can"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vishvananda/netlink"
)

type alreadyClosedCANReadWriteCloser struct{}

func (alreadyClosedCANReadWriteCloser) Read(_ []byte) (int, error) {
	return 0, io.EOF
}

func (alreadyClosedCANReadWriteCloser) ReadFrame(_ *can.Frame) error {
	return io.EOF
}

func (alreadyClosedCANReadWriteCloser) Write(b []byte) (int, error) {
	return len(b), nil
}

func (alreadyClosedCANReadWriteCloser) WriteFrame(can.Frame) error {
	return nil
}

func (alreadyClosedCANReadWriteCloser) Close() error {
	return os.ErrClosed
}

func TestSocketCANCloseIgnoresAlreadyClosedBus(t *testing.T) {
	c := &SocketCANChannel{
		bus:        can.NewBus(alreadyClosedCANReadWriteCloser{}),
		busHandler: can.NewHandler(func(can.Frame) {}),
	}

	require.NoError(t, c.Close())
	require.NoError(t, c.Close())
}

func TestSocketCANWriteAfterCloseReturnsError(t *testing.T) {
	c := &SocketCANChannel{}

	require.NoError(t, c.Close())
	require.ErrorContains(t, c.WriteFrame(can.Frame{}), "canbus channel is closed")
}

func TestSocketCANStartAfterCloseReturnsError(t *testing.T) {
	c := &SocketCANChannel{}

	require.NoError(t, c.Close())
	require.ErrorContains(t, c.Start(context.Background()), "SocketCAN channel is closed")
}

func TestSocketCANCloseSerializesWithStartup(t *testing.T) {
	c := &SocketCANChannel{}
	c.startMu.Lock()
	closeDone := make(chan error, 1)
	go func() { closeDone <- c.Close() }()

	select {
	case <-closeDone:
		t.Fatal("Close returned while startup still owned the lifecycle lock")
	case <-time.After(20 * time.Millisecond):
	}
	c.startMu.Unlock()
	require.NoError(t, <-closeDone)
	require.True(t, c.isClosed())
}

func TestSocketCANStartReturnsMissingInterfaceError(t *testing.T) {
	channel := NewSocketCANChannel(logrus.New(), SocketCANChannelOptions{
		InterfaceName: "boatkit-test-interface-that-does-not-exist",
		BitRate:       250000,
	})

	err := channel.Start(context.Background())

	require.ErrorContains(t, err, "no link found")
}

func TestSocketCANBounceReasonDetectsBusOffWhileAdministrativelyUp(t *testing.T) {
	link := &netlink.Can{
		LinkAttrs: netlink.LinkAttrs{Flags: net.FlagUp, OperState: netlink.OperDown},
		BitRate:   250000,
		State:     netlink.CAN_STATE_BUS_OFF,
		RestartMs: 1000,
	}
	options := SocketCANChannelOptions{BitRate: 250000, RestartMilliseconds: 1000}

	assert.True(t, socketCANLinkIsUp(link))
	assert.Equal(t, "interface is bus-off", socketCANBounceReason(link, options))
}

func TestSocketCANBounceReasonDetectsMissingAutomaticRestart(t *testing.T) {
	link := &netlink.Can{
		LinkAttrs: netlink.LinkAttrs{Flags: net.FlagUp},
		BitRate:   250000,
		State:     netlink.CAN_STATE_ERROR_ACTIVE,
	}
	options := SocketCANChannelOptions{BitRate: 250000, RestartMilliseconds: 1000}

	assert.Equal(t, "restart delay is 0 ms, expected 1000 ms", socketCANBounceReason(link, options))
}

func TestSocketCANLinkUpArgsIncludeAutomaticRestart(t *testing.T) {
	options := SocketCANChannelOptions{
		InterfaceName:       "can0",
		BitRate:             250000,
		RestartMilliseconds: 1000,
	}

	assert.Equal(t, []string{
		"ip", "link", "set", "can0", "up", "type", "can",
		"bitrate", "250000", "restart-ms", "1000",
	}, socketCANLinkUpArgs(options))
}

func TestSocketCANLinkUpFallsBackWhenAutomaticRestartIsUnsupported(t *testing.T) {
	var logOutput bytes.Buffer
	log := logrus.New()
	log.SetOutput(&logOutput)
	channel := NewSocketCANChannel(log, SocketCANChannelOptions{
		InterfaceName:       "can1",
		BitRate:             250000,
		RestartMilliseconds: 1000,
	})
	var commands [][]string
	run := func(_ context.Context, args []string) ([]byte, error) {
		commands = append(commands, append([]string(nil), args...))
		if len(commands) == 1 {
			return nil, &socketCANCommandError{
				err:    errors.New("exit status 2"),
				stderr: "Error: Device doesn't support restart from Bus Off.\n",
			}
		}
		return nil, nil
	}

	require.NoError(t, channel.bringUpSocketCANLink(context.Background(), run))
	assert.Equal(t, [][]string{
		{"ip", "link", "set", "can1", "up", "type", "can", "bitrate", "250000", "restart-ms", "1000"},
		{"ip", "link", "set", "can1", "up", "type", "can", "bitrate", "250000"},
	}, commands)
	assert.Contains(t, logOutput.String(), "does not support kernel-managed bus-off restart")
}

func TestSocketCANLinkUpDoesNotHideOtherErrors(t *testing.T) {
	log := logrus.New()
	log.SetOutput(io.Discard)
	channel := NewSocketCANChannel(log, SocketCANChannelOptions{
		InterfaceName:       "can1",
		BitRate:             250000,
		RestartMilliseconds: 1000,
	})
	wantErr := &socketCANCommandError{
		err:    errors.New("exit status 2"),
		stderr: "RTNETLINK answers: Operation not permitted\n",
	}
	commandCount := 0
	run := func(_ context.Context, _ []string) ([]byte, error) {
		commandCount++
		return nil, wantErr
	}

	err := channel.bringUpSocketCANLink(context.Background(), run)

	assert.ErrorIs(t, err, wantErr)
	assert.Equal(t, 1, commandCount)
}

func TestSocketCANChannelVCan0WriteFrame(t *testing.T) {
	requireVCan0(t)

	log := logrus.New()
	log.SetLevel(logrus.InfoLevel)

	channel := NewSocketCANChannel(log, SocketCANChannelOptions{
		InterfaceName:        "vcan0",
		BitRate:              250000,
		ForceBounceInterface: false,
		MessageHandler:       func(can.Frame) {},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, channel.Start(ctx))

	errCh := make(chan error, 1)
	go func() {
		errCh <- channel.Run(ctx)
	}()
	t.Cleanup(func() {
		cancel()
		assert.NoError(t, channel.Close())
	})

	testFrame := can.Frame{
		ID:     0x123,
		Length: 4,
		Data:   [8]byte{0x01, 0x02, 0x03, 0x04},
	}
	assert.NoError(t, channel.WriteFrame(testFrame))
}

func TestSocketCANChannelVCan0AllowsNilMessageHandler(t *testing.T) {
	requireVCan0(t)

	log := logrus.New()
	log.SetLevel(logrus.InfoLevel)

	channel := NewSocketCANChannel(log, SocketCANChannelOptions{
		InterfaceName:        "vcan0",
		BitRate:              250000,
		ForceBounceInterface: false,
		MessageHandler:       nil,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, channel.Start(ctx))

	errCh := make(chan error, 1)
	go func() {
		errCh <- channel.Run(ctx)
	}()
	t.Cleanup(func() {
		cancel()
		assert.NoError(t, channel.Close())
	})

	testFrame := can.Frame{
		ID:     0x124,
		Length: 1,
		Data:   [8]byte{0x01},
	}
	assert.NoError(t, channel.WriteFrame(testFrame))
}

func requireVCan0(t *testing.T) {
	t.Helper()

	output, err := exec.Command("ip", "link", "show", "vcan0").CombinedOutput()
	if err != nil {
		t.Skipf("vcan0 is not available: %v: %s", err, string(output))
	}
	if !strings.Contains(string(output), "UP") {
		t.Skipf("vcan0 is not up: %s", string(output))
	}
}
