package hid

import "context"

// BrightnessDevice defines the interface for a device that can control brightness.
type BrightnessDevice interface {
	SetBrightness(ctx context.Context, level float64) error
	GetBrightness(ctx context.Context) (float64, error)
	Close() error
}

// Common HID Usages
const (
	UsagePageMonitor         = 0x59
	UsagePageStandardMonitor = 0x80
	UsagePageLGVendor        = 0xFF00
	UsageBrightness          = 0x01
)
