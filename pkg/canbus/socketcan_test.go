package canbus

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/brutella/can"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSocketCANVCan0Integration tests the complete SocketCAN functionality with vcan0
func TestSocketCANVCan0Integration(t *testing.T) {
	// Check if vcan0 interface exists and is up
	cmd := exec.Command("ip", "link", "show", "vcan0")
	output, err := cmd.Output()
	if err != nil {
		t.Skip("Skipping vcan test - vcan0 interface not available")
	}

	outputStr := string(output)
	if !strings.Contains(outputStr, "UP") {
		t.Skip("Skipping vcan test - vcan0 interface not up")
	}

	// Create logger
	log := logrus.New()
	log.SetLevel(logrus.InfoLevel)

	// Create SocketCAN channel (write-only, no message handler)
	options := SocketCANChannelOptions{
		InterfaceName:        "vcan0",
		BitRate:              250000, // Not used for vcan but required
		ForceBounceInterface: false,
		MessageHandler:       nil, // No handler - write-only test
	}

	channel := NewSocketCANChannel(log, options)

	// Test 1: Run the channel in a goroutine
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	errChan := make(chan error, 1)
	go func() {
		errChan <- channel.Run(ctx)
	}()

	// Wait a bit for the channel to start
	time.Sleep(100 * time.Millisecond)

	// Test 2: Write a frame
	testFrame := can.Frame{
		ID:     0x123,
		Length: 8,
		Data:   [8]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08},
	}

	err = channel.WriteFrame(testFrame)
	require.NoError(t, err, "Failed to write frame")

	// Test 3: Write multiple frames
	for i := 0; i < 5; i++ {
		frame := can.Frame{
			ID:     uint32(0x200 + i),
			Length: 4,
			Data:   [8]byte{byte(i), byte(i + 1), byte(i + 2), byte(i + 3)},
		}
		err = channel.WriteFrame(frame)
		require.NoError(t, err, "Failed to write frame %d", i)
	}

	// Wait for all frames to be processed
	time.Sleep(200 * time.Millisecond)

	// Test 4: Cancel context and close the channel
	cancel()

	// Wait for the channel to stop
	select {
	case err := <-errChan:
		fmt.Printf("Channel stopped with error: %v\n", err)
	case <-time.After(2 * time.Second):
		fmt.Println("Channel did not stop within timeout")
	}

	err = channel.Close()
	require.NoError(t, err, "Failed to close channel")
}

// TestSocketCANVCan0ErrorHandling tests error conditions
func TestSocketCANVCan0ErrorHandling(t *testing.T) {
	// Skip if not running as root
	if !isRoot() {
		t.Skip("Skipping vcan test - requires root privileges")
	}

	log := logrus.New()
	log.SetLevel(logrus.InfoLevel)

	// Test 1: Try to use non-existent interface
	options := SocketCANChannelOptions{
		InterfaceName:        "vcan999",
		BitRate:              250000,
		ForceBounceInterface: false,
		MessageHandler:       func(frame can.Frame) {},
	}

	channel := NewSocketCANChannel(log, options)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := channel.Run(ctx)
	assert.Error(t, err, "Expected error when using non-existent interface")

	// Test 2: Try to write to closed channel
	options.InterfaceName = "vcan0"
	channel2 := NewSocketCANChannel(log, options)

	// Don't run the channel, just try to write
	testFrame := can.Frame{ID: 0x123, Length: 1, Data: [8]byte{0x01}}
	err = channel2.WriteFrame(testFrame)
	assert.Error(t, err, "Expected error when writing to uninitialized channel")
}

// TestSocketCANVCan0InterfaceSetup tests the interface setup process
func TestSocketCANVCan0InterfaceSetup(t *testing.T) {
	// Skip if not running as root
	if !isRoot() {
		t.Skip("Skipping vcan test - requires root privileges")
	}

	// Test interface setup
	err := setupVCanInterface("vcan0")
	require.NoError(t, err, "Failed to setup vcan0")

	// Verify interface exists and is up
	cmd := exec.Command("ip", "link", "show", "vcan0")
	output, err := cmd.Output()
	require.NoError(t, err, "Failed to check vcan0 status")

	outputStr := string(output)
	assert.Contains(t, outputStr, "vcan0", "vcan0 interface not found")
	assert.Contains(t, outputStr, "UP", "vcan0 interface not up")

	// Cleanup
	cleanupVCanInterface("vcan0")
}

// TestSocketCANVCan0WriteOnly tests just the write functionality for use with external candump
func TestSocketCANVCan0WriteOnly(t *testing.T) {
	// Check if vcan0 interface exists and is up
	cmd := exec.Command("ip", "link", "show", "vcan0")
	output, err := cmd.Output()
	if err != nil {
		t.Skip("Skipping vcan test - vcan0 interface not available")
	}

	outputStr := string(output)
	if !strings.Contains(outputStr, "UP") {
		t.Skip("Skipping vcan test - vcan0 interface not up")
	}

	log := logrus.New()
	log.SetLevel(logrus.InfoLevel)

	// Create channel with no message handler (write-only)
	options := SocketCANChannelOptions{
		InterfaceName:        "vcan0",
		BitRate:              250000,
		ForceBounceInterface: false,
		MessageHandler:       nil, // No handler
	}

	channel := NewSocketCANChannel(log, options)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Start the channel in a goroutine
	errChan := make(chan error, 1)
	go func() {
		errChan <- channel.Run(ctx)
	}()

	// Wait for the channel to start
	time.Sleep(200 * time.Millisecond)

	// Write a single frame
	testFrame := can.Frame{
		ID:     0x123,
		Length: 4,
		Data:   [8]byte{0x01, 0x02, 0x03, 0x04},
	}

	err = channel.WriteFrame(testFrame)
	require.NoError(t, err, "Failed to write frame")

	// Cancel context to stop the channel
	cancel()

	// Wait for the channel to stop
	select {
	case err := <-errChan:
		fmt.Printf("Channel stopped with error: %v\n", err)
	case <-time.After(2 * time.Second):
		fmt.Println("Channel did not stop within timeout")
	}

	// Close channel
	err = channel.Close()
	require.NoError(t, err, "Failed to close write-only channel")

}

// TestSocketCANVCan0MultipleExtendedFrames tests writing multiple extended CAN frames
func TestSocketCANVCan0MultipleExtendedFrames(t *testing.T) {
	// Check if vcan0 interface exists and is up
	cmd := exec.Command("ip", "link", "show", "vcan0")
	output, err := cmd.Output()
	if err != nil {
		t.Skip("Skipping vcan test - vcan0 interface not available")
	}

	outputStr := string(output)
	if !strings.Contains(outputStr, "UP") {
		t.Skip("Skipping vcan test - vcan0 interface not up")
	}

	// Create logger
	log := logrus.New()
	log.SetLevel(logrus.InfoLevel)

	// Create SocketCAN channel (write-only, no message handler)
	options := SocketCANChannelOptions{
		InterfaceName:        "vcan0",
		BitRate:              250000, // Not used for vcan but required
		ForceBounceInterface: false,
		MessageHandler:       nil, // No handler - write-only test
	}

	channel := NewSocketCANChannel(log, options)

	// Run the channel in a goroutine
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	errChan := make(chan error, 1)
	go func() {
		errChan <- channel.Run(ctx)
	}()

	// Wait for the channel to start
	time.Sleep(100 * time.Millisecond)

	// Test data: Multiple extended frames with 29-bit IDs
	// Set bit 31 (0x80000000) to indicate extended frame format
	extendedFrames := []can.Frame{
		{
			ID:     0x18FF1234 | 0x80000000, // Extended ID (29-bit) with EFF flag
			Length: 8,
			Data:   [8]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08},
		},
		{
			ID:     0x1AFF5678 | 0x80000000, // Extended ID (29-bit) with EFF flag
			Length: 6,
			Data:   [8]byte{0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x00, 0x00},
		},
		{
			ID:     0x1BFF9ABC | 0x80000000, // Extended ID (29-bit) with EFF flag
			Length: 4,
			Data:   [8]byte{0xAA, 0xBB, 0xCC, 0xDD, 0x00, 0x00, 0x00, 0x00},
		},
		{
			ID:     0x1CFFDEF0 | 0x80000000, // Extended ID (29-bit) with EFF flag
			Length: 8,
			Data:   [8]byte{0xFF, 0xEE, 0xDD, 0xCC, 0xBB, 0xAA, 0x99, 0x88},
		},
		{
			ID:     0x1DFF1234 | 0x80000000, // Extended ID (29-bit) with EFF flag
			Length: 2,
			Data:   [8]byte{0x77, 0x66, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
		},
	}

	// Write all extended frames
	for i, frame := range extendedFrames {
		err = channel.WriteFrame(frame)
		require.NoError(t, err, "Failed to write extended frame %d", i)

		// Small delay between frames
		time.Sleep(10 * time.Millisecond)
	}

	// Wait for all frames to be processed
	time.Sleep(200 * time.Millisecond)

	// Cancel context and close the channel
	cancel()

	// Wait for the channel to stop
	select {
	case err := <-errChan:
		fmt.Printf("Channel stopped with error: %v\n", err)
	case <-time.After(2 * time.Second):
		fmt.Println("Channel did not stop within timeout")
	}

	err = channel.Close()
	require.NoError(t, err, "Failed to close channel")
}

// Helper functions

func isRoot() bool {
	cmd := exec.Command("id", "-u")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return string(output) == "0\n"
}

func setupVCanInterface(iface string) error {
	// Remove interface if it exists
	exec.Command("ip", "link", "del", iface).Run()

	// Create vcan interface
	cmd := exec.Command("ip", "link", "add", "dev", iface, "type", "vcan")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create %s: %v, output: %s", iface, err, string(output))
	}

	// Bring interface up
	cmd = exec.Command("ip", "link", "set", iface, "up")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to bring up %s: %v, output: %s", iface, err, string(output))
	}

	return nil
}

func cleanupVCanInterface(iface string) error {
	cmd := exec.Command("ip", "link", "del", iface)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to delete %s: %v, output: %s", iface, err, string(output))
	}
	return nil
}
