package als

import (
	"context"
	"fmt"
)

type NotImplementedError struct{}

func (e *NotImplementedError) Error() string {
	return "ALS not implemented on this platform"
}

var NotImplemented = &NotImplementedError{}

type ALSDeviceInfo struct {
	VendorID  uint16
	ProductID uint16
	Path      string
	name      string // Private field for platform-specific name
}

// String implements fmt.Stringer and returns a human-readable description of the device.
func (d ALSDeviceInfo) String() string {
	name := d.name
	if name == "" {
		name = "Unknown ALS"
	}
	return fmt.Sprintf("%s (VID: %04x, PID: %04x, Path: %s)", name, d.VendorID, d.ProductID, d.Path)
}

type ALS interface {
	GetLux(ctx context.Context) (float64, error)
	// SubscribeLux returns a channel that receives lux readings.
	// threshold specifies the minimum change in lux required to trigger an update.
	// If 0, the platform's default threshold is used.
	SubscribeLux(ctx context.Context, threshold float64) (<-chan float64, error)
	Close() error
}

type ALSManager interface {
	List(ctx context.Context) ([]ALSDeviceInfo, error)
	Open(ctx context.Context, deviceInfo ALSDeviceInfo) (ALS, error)
	Default(ctx context.Context) (ALS, error)
	Close() error
}

// NewManager returns a platform-specific ALSManager.
func NewManager() (ALSManager, error) {
	return newManager()
}

// Uninitialize performs global cleanup for the ALS package.
func Uninitialize() {
	uninitialize()
}
