package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/plissken/ultrafine-brightness/pkg/hid"
	"os"
	"time"
)

func main() {
	listAllCmd := flag.Bool("all", false, "List ALL HID devices")
	listCmd := flag.Bool("list", false, "List available HID brightness devices")
	setCmd := flag.Int("set", -1, "Set brightness (0-100)")
	idCmd := flag.String("id", "", "Specify device ID to open (optional if only one device found)")

	flag.Parse()

	if !*listCmd && !*listAllCmd && *setCmd == -1 {
		fmt.Println("Usage:")
		fmt.Println("  test-hid -list")
		fmt.Println("  test-hid -all")
		fmt.Println("  test-hid -set <0-100> [-id <device_id>]")
		os.Exit(1)
	}

	var devices []hid.DeviceInfo
	var err error

	if *listAllCmd {
		devices, err = hid.Enumerate(0, 0)
	} else {
		devices, err = hid.FindBrightnessDevices()
	}

	if err != nil {
		fmt.Printf("Error searching for devices: %v\n", err)
		os.Exit(1)
	}

	if len(devices) == 0 {
		fmt.Println("No HID brightness devices found (Usage Page 0x59).")
		return
	}

	if *listCmd || *listAllCmd {
		fmt.Printf("Found %d HID device(s):\n", len(devices))
		for i, d := range devices {
			fmt.Printf("[%d] Name: %s\n    ID: %s\n    VID: 0x%04X, PID: 0x%04X, UsagePage: 0x%04X, UsageId: 0x%04X\n",
				i, d.Name, d.ID, d.Vid, d.Pid, d.UsagePage, d.UsageId)
		}
		return
	}

	if *setCmd != -1 {
		if *setCmd < 0 || *setCmd > 100 {
			fmt.Println("Brightness must be between 0 and 100.")
			os.Exit(1)
		}

		targetID := *idCmd
		if targetID == "" {
			targetID = devices[0].ID
			fmt.Printf("No ID specified, using first device: %s\n", devices[0].Name)
		}

		dev, err := hid.Open(targetID)
		if err != nil {
			fmt.Printf("Failed to open device: %v\n", err)
			os.Exit(1)
		}
		defer dev.Close()

		// Get current brightness first
		current, err := dev.GetBrightness(context.Background())
		if err == nil {
			fmt.Printf("Current brightness: %.1f%%\n", current)
		}

		fmt.Printf("Setting brightness to %d%%...\n", *setCmd)
		err = dev.SetBrightness(context.Background(), float64(*setCmd))
		if err != nil {
			fmt.Printf("Error setting brightness: %v\n", err)
			os.Exit(1)
		}

		// Small delay for device to apply
		time.Sleep(150 * time.Millisecond)

		// Read back
		actual, err := dev.GetBrightness(context.Background())
		if err == nil {
			fmt.Printf("Actual brightness level: %.1f%%\n", actual)
		}
		fmt.Println("Done.")
	}
}
