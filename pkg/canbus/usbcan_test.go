package canbus

import (
	"context"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func TestUSBCANStartReturnsMissingSerialPortError(t *testing.T) {
	channel := NewUSBCANChannel(logrus.New(), USBCANChannelOptions{
		SerialPortName: "/boatkit/test/serial-port-that-does-not-exist",
		SerialBaudRate: 2_000_000,
		BitRate:        250_000,
	})

	err := channel.Start(context.Background())

	require.Error(t, err)
}
