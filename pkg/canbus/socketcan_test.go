package canbus

import (
	"io"
	"os"
	"testing"

	"github.com/brutella/can"
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
