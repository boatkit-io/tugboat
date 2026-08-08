package canbus

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"go.bug.st/serial"
)

type lifecycleSerialPort struct {
	closed atomic.Bool
}

func (*lifecycleSerialPort) SetMode(*serial.Mode) error                           { return nil }
func (*lifecycleSerialPort) Read([]byte) (int, error)                             { return 0, nil }
func (*lifecycleSerialPort) Write(p []byte) (int, error)                          { return len(p), nil }
func (*lifecycleSerialPort) Drain() error                                         { return nil }
func (*lifecycleSerialPort) ResetInputBuffer() error                              { return nil }
func (*lifecycleSerialPort) ResetOutputBuffer() error                             { return nil }
func (*lifecycleSerialPort) SetDTR(bool) error                                    { return nil }
func (*lifecycleSerialPort) SetRTS(bool) error                                    { return nil }
func (*lifecycleSerialPort) GetModemStatusBits() (*serial.ModemStatusBits, error) { return nil, nil }
func (*lifecycleSerialPort) SetReadTimeout(time.Duration) error                   { return nil }
func (p *lifecycleSerialPort) Close() error {
	p.closed.Store(true)
	return nil
}
func (*lifecycleSerialPort) Break(time.Duration) error { return nil }

func TestUSBCANStartReturnsMissingSerialPortError(t *testing.T) {
	channel := NewUSBCANChannel(logrus.New(), USBCANChannelOptions{
		SerialPortName: "/boatkit/test/serial-port-that-does-not-exist",
		SerialBaudRate: 2_000_000,
		BitRate:        250_000,
	})

	err := channel.Start(context.Background())

	require.Error(t, err)
}

func TestUSBCANStartCancellationDoesNotWaitForSerialOpen(t *testing.T) {
	openEntered := make(chan struct{})
	openRelease := make(chan struct{})
	port := &lifecycleSerialPort{}
	channel := NewUSBCANChannel(logrus.New(), USBCANChannelOptions{
		SerialPortName: "test-port",
		SerialBaudRate: 2_000_000,
		BitRate:        250_000,
	})
	channel.openPort = func(string, *serial.Mode) (serial.Port, error) {
		close(openEntered)
		<-openRelease
		return port, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	startDone := make(chan error, 1)
	go func() { startDone <- channel.Start(ctx) }()
	<-openEntered

	cancel()
	select {
	case err := <-startDone:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(time.Second):
		t.Fatal("Start did not return after context cancellation")
	}
	close(openRelease)
	require.Eventually(t, port.closed.Load, time.Second, time.Millisecond)
}

func TestUSBCANCloseInterruptsBlockedSerialOpen(t *testing.T) {
	openEntered := make(chan struct{})
	openRelease := make(chan struct{})
	port := &lifecycleSerialPort{}
	channel := NewUSBCANChannel(logrus.New(), USBCANChannelOptions{
		SerialPortName: "test-port",
		SerialBaudRate: 2_000_000,
		BitRate:        250_000,
	})
	channel.openPort = func(string, *serial.Mode) (serial.Port, error) {
		close(openEntered)
		<-openRelease
		return port, nil
	}
	startDone := make(chan error, 1)
	go func() { startDone <- channel.Start(context.Background()) }()
	<-openEntered

	require.NoError(t, channel.Close())
	select {
	case err := <-startDone:
		require.ErrorContains(t, err, "closed")
	case <-time.After(time.Second):
		t.Fatal("Start did not return after Close")
	}
	close(openRelease)
	require.Eventually(t, port.closed.Load, time.Second, time.Millisecond)
}
