package main

import (
	"context"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"time"

	"bl_wm_esc/client"
	"bl_wm_esc/tui"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	readFlag := flag.Bool("read", false, "Connect to ESC, read settings, print them, and exit")
	testToggleFlag := flag.Bool("test-toggle", false, "Connect to ESC, toggle Motor Direction, write settings, reconnect, and verify")
	telemetryFlag := flag.Bool("telemetry", false, "Connect to ESC, query telemetry, print it, and exit")
	flag.Parse()

	escClient := client.NewESCClient()

	if *readFlag {
		runRead(escClient)
		return
	}

	if *testToggleFlag {
		runTestToggle(escClient)
		return
	}

	if *telemetryFlag {
		runTelemetry(escClient)
		return
	}

	// Initialize TUI Bubble Tea model
	m := tui.NewModel(escClient)

	// Create and run the Bubble Tea program
	p := tea.NewProgram(m, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v\n", err)
		os.Exit(1)
	}
}

func runRead(c *client.ESCClient) {
	fmt.Println("Connecting to ESC...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.Connect(ctx); err != nil {
		fmt.Printf("Failed to connect: %v\n", err)
		return
	}
	defer c.Close()

	fmt.Printf("ESC: %s | Firmware: %s | Active Profile: %d\n", c.ModelName(), c.Firmware(), c.ActiveProfile())
	
	fmt.Println("Reading settings...")
	data, err := c.ReadSettings(c.ActiveProfile())
	if err != nil {
		fmt.Printf("Failed to read settings: %v\n", err)
		return
	}

	fmt.Printf("Raw settings hex (%d bytes):\n  %s\n", len(data), hex.EncodeToString(data))
	
	// Print motor direction byte (offset 11 -> real offset 12)
	if len(data) >= 50 {
		dirVal := data[12]
		fmt.Printf("Current Motor Direction byte (index 12): %d (Hex: 0x%02x)\n", dirVal, dirVal)
	}
}

func runTestToggle(c *client.ESCClient) {
	fmt.Println("=== Connecting to ESC ===")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := c.Connect(ctx); err != nil {
		cancel()
		fmt.Printf("Failed to connect: %v\n", err)
		return
	}
	cancel()

	fmt.Printf("ESC: %s | Firmware: %s | Active Profile: %d\n", c.ModelName(), c.Firmware(), c.ActiveProfile())
	
	fmt.Println("Reading current settings...")
	data, err := c.ReadSettings(c.ActiveProfile())
	if err != nil {
		c.Close()
		fmt.Printf("Failed to read settings: %v\n", err)
		return
	}

	if len(data) < 50 {
		c.Close()
		fmt.Printf("Settings payload too short: %d bytes\n", len(data))
		return
	}

	oldHex := hex.EncodeToString(data)
	oldDir := data[12]
	fmt.Printf("Original Payload hex: %s\n", oldHex)
	fmt.Printf("Original Motor Direction byte (index 12): %d\n", oldDir)

	// Toggle motor direction: if 0, set to 1; if anything else, set to 0
	var newDir byte = 0
	if oldDir == 0 {
		newDir = 1
	}
	data[12] = newDir
	fmt.Printf("Toggling Motor Direction byte to: %d\n", newDir)

	newHex := hex.EncodeToString(data)
	fmt.Printf("Writing settings payload hex: %s\n", newHex)
	c.Close() // Close connection to reset state

	// Reconnect and write
	fmt.Println("Reconnecting for write...")
	c2 := client.NewESCClient()
	ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
	if err := c2.Connect(ctx2); err != nil {
		cancel2()
		fmt.Printf("Failed to reconnect for write: %v\n", err)
		return
	}
	cancel2()

	err = c2.WriteSettings(c2.ActiveProfile(), data)
	c2.Close()
	if err != nil {
		fmt.Printf("WriteSettings failed: %v\n", err)
	} else {
		fmt.Println("WriteSettings + Commit succeeded!")
	}

	// Wait for ESC to reboot and WLAN connection to recover
	fmt.Println("Waiting 5 seconds for ESC reboot and WLAN reconnection...")
	time.Sleep(5 * time.Second)

	// Reconnect to verify
	fmt.Println("Reconnecting to verify settings...")
	c3 := client.NewESCClient()
	
	// We'll retry connecting for up to 15 seconds because Wi-Fi reconnection takes time
	connected := false
	for i := 0; i < 5; i++ {
		ctx3, cancel3 := context.WithTimeout(context.Background(), 3*time.Second)
		err = c3.Connect(ctx3)
		cancel3()
		if err == nil {
			connected = true
			break
		}
		fmt.Printf("  Connection attempt %d/5 failed: %v, retrying in 2 seconds...\n", i+1, err)
		time.Sleep(2 * time.Second)
	}

	if !connected {
		fmt.Println("Failed to reconnect to ESC for verification.")
		return
	}
	defer c3.Close()

	fmt.Println("Reading setup after reboot...")
	dataAfter, err := c3.ReadSettings(c3.ActiveProfile())
	if err != nil {
		fmt.Printf("Failed to read settings after reboot: %v\n", err)
		return
	}

	afterHex := hex.EncodeToString(dataAfter)
	afterDir := dataAfter[12]
	fmt.Printf("Payload hex after reboot: %s\n", afterHex)
	fmt.Printf("Motor Direction byte (index 12) after reboot: %d\n", afterDir)

	if afterDir == newDir {
		fmt.Println("SUCCESS: Motor Direction byte persisted successfully!")
	} else {
		fmt.Println("FAILURE: Motor Direction byte did not change (it reverted to original value).")
	}
}

func runTelemetry(c *client.ESCClient) {
	fmt.Println("Connecting to ESC...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.Connect(ctx); err != nil {
		fmt.Printf("Failed to connect: %v\n", err)
		return
	}
	defer c.Close()

	fmt.Printf("ESC: %s | Firmware: %s | Active Profile: %d\n", c.ModelName(), c.Firmware(), c.ActiveProfile())
	
	fmt.Println("Querying telemetry...")
	t, err := c.QueryTelemetry()
	if err != nil {
		fmt.Printf("Failed to query telemetry: %v\n", err)
		return
	}

	fmt.Println("=== Live Telemetry ===")
	fmt.Printf("Voltage:    %.2f V (Min: %.2f V)\n", t.Voltage, t.VoltageMin)
	fmt.Printf("Current:    %.1f A (Max: %.1f A)\n", t.Current, t.CurrentMax)
	fmt.Printf("ESC Temp:   %d °C (Max: %d °C)\n", t.TempESC, t.TempESCMax)
	fmt.Printf("Motor Temp: %d °C (Max: %d °C)\n", t.TempMotor, t.TempMotorMax)
	fmt.Printf("RPM:        %d rpm (Max: %d rpm)\n", t.RPM, t.RPMMax)
	fmt.Printf("Throttle:   %.1f %%\n", t.Throttle)
	fmt.Printf("Run Time:   %.1f s\n", t.RunTime)
}

