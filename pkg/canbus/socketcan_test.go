package canbus

import (
	"context"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/brutella/can"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
