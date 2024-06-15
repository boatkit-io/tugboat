package main

import (
	"context"
	"fmt"

	"github.com/boatkit-io/tugboat/pkg/canbus"
	"github.com/brutella/can"
	"github.com/sirupsen/logrus"
)

func main() {
	ctx := context.Background()

	options := canbus.USBCANChannelOptions{
		SerialPortName: "/dev/tty.usbserial-210",
		SerialBaudRate: 2000000,
		BitRate:        250000,
		FrameHandler: func(frame can.Frame) {
			fmt.Printf("handled frame: %+v\n", frame)
		},
	}
	c := canbus.NewUSBCANChannel(logrus.StandardLogger(), options)
	if err := c.Run(ctx); err != nil {
		fmt.Printf("error: %v\n", err)
	}
}
