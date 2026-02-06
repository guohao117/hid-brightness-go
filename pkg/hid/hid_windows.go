//go:build windows
// +build windows

package hid

import (
	"context"
	"fmt"
	"sync"
	"syscall"
	"unsafe"
)

var (
	modWinrtBridge      = syscall.NewLazyDLL("winrt_bridge.dll")
	procHidInitialize   = modWinrtBridge.NewProc("winrt_hid_initialize")
	procHidEnumerate    = modWinrtBridge.NewProc("winrt_hid_enumerate")
	procHidGetCount     = modWinrtBridge.NewProc("winrt_hid_get_device_count")
	procHidGetInfo      = modWinrtBridge.NewProc("winrt_hid_get_device_info")
	procHidFreeEnum     = modWinrtBridge.NewProc("winrt_hid_free_enumeration")
	procHidOpen         = modWinrtBridge.NewProc("winrt_hid_open")
	procHidGetFeature   = modWinrtBridge.NewProc("winrt_hid_get_feature_report")
	procHidSendFeature  = modWinrtBridge.NewProc("winrt_hid_send_feature_report")
	procHidClose        = modWinrtBridge.NewProc("winrt_hid_close")
	procHidUninitialize = modWinrtBridge.NewProc("winrt_hid_uninitialize")

	initOnce sync.Once
	initErr  error
)

func ensureWinRTInitialized() error {
	initOnce.Do(func() {
		ret, _, _ := procHidInitialize.Call()
		if int32(ret) != 0 {
			initErr = fmt.Errorf("failed to initialize WinRT apartment for HID")
		}
	})
	return initErr
}

// DeviceInfo represents information about a HID device.
type DeviceInfo struct {
	ID        string
	Name      string
	Vid       uint16
	Pid       uint16
	UsagePage uint16
	UsageId   uint16
}

// Enumerate searches for HID devices with the given usage page and usage ID.
// If usagePage is 0, it returns all HID devices.
func Enumerate(usagePage, usageId uint16) ([]DeviceInfo, error) {
	if err := ensureWinRTInitialized(); err != nil {
		return nil, err
	}

	handle, _, _ := procHidEnumerate.Call(uintptr(usagePage), uintptr(usageId))
	if handle == 0 {
		return nil, fmt.Errorf("failed to enumerate HID devices")
	}
	defer procHidFreeEnum.Call(handle)

	count, _, _ := procHidGetCount.Call(handle)
	devices := make([]DeviceInfo, int(count))

	for i := 0; i < int(count); i++ {
		var idBuf [512]byte
		var nameBuf [512]byte
		var vid, pid, up, ui uint16
		procHidGetInfo.Call(
			handle,
			uintptr(i),
			uintptr(unsafe.Pointer(&idBuf[0])),
			uintptr(len(idBuf)),
			uintptr(unsafe.Pointer(&nameBuf[0])),
			uintptr(len(nameBuf)),
			uintptr(unsafe.Pointer(&vid)),
			uintptr(unsafe.Pointer(&pid)),
			uintptr(unsafe.Pointer(&up)),
			uintptr(unsafe.Pointer(&ui)),
		)

		devices[i] = DeviceInfo{
			ID:        bytePtrToString(&idBuf[0]),
			Name:      bytePtrToString(&nameBuf[0]),
			Vid:       vid,
			Pid:       pid,
			UsagePage: up,
			UsageId:   ui,
		}
	}

	return devices, nil
}

func bytePtrToString(p *byte) string {
	if p == nil {
		return ""
	}
	var n int
	for ptr := unsafe.Pointer(p); *(*byte)(ptr) != 0; n++ {
		ptr = unsafe.Pointer(uintptr(ptr) + 1)
	}
	return string(unsafe.Slice(p, n))
}

// Device represents an opened HID device.
type Device struct {
	handle uintptr
	mu     sync.Mutex
}

// Open opens a HID device by its system ID.
func Open(deviceId string) (*Device, error) {
	if err := ensureWinRTInitialized(); err != nil {
		return nil, err
	}

	cId, err := syscall.BytePtrFromString(deviceId)
	if err != nil {
		return nil, err
	}

	handle, _, _ := procHidOpen.Call(uintptr(unsafe.Pointer(cId)))
	if handle == 0 {
		return nil, fmt.Errorf("failed to open HID device: %s", deviceId)
	}

	return &Device{handle: handle}, nil
}

// Close closes the HID device.
func (d *Device) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.handle != 0 {
		procHidClose.Call(d.handle)
		d.handle = 0
	}
	return nil
}

// GetFeatureReport reads a feature report from the device.
func (d *Device) GetFeatureReport(reportID uint16, buffer []byte) (int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.handle == 0 {
		return 0, fmt.Errorf("device is closed")
	}

	var bytesWritten uint32
	ret, _, _ := procHidGetFeature.Call(
		d.handle,
		uintptr(reportID),
		uintptr(unsafe.Pointer(&buffer[0])),
		uintptr(len(buffer)),
		uintptr(unsafe.Pointer(&bytesWritten)),
	)

	if int32(ret) != 0 {
		return 0, fmt.Errorf("failed to get feature report %d", reportID)
	}

	return int(bytesWritten), nil
}

// SendFeatureReport sends a feature report to the device.
func (d *Device) SendFeatureReport(reportID uint16, data []byte) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.handle == 0 {
		return fmt.Errorf("device is closed")
	}

	ret, _, _ := procHidSendFeature.Call(
		d.handle,
		uintptr(reportID),
		uintptr(unsafe.Pointer(&data[0])),
		uintptr(len(data)),
	)

	if int32(ret) != 0 {
		return fmt.Errorf("failed to send feature report %d", reportID)
	}

	return nil
}

// SetBrightness sets the brightness level (0-100).
// For LG UltraFine, it maps 0-100 to the range 0x0190 - 0xD2F0 (400 - 54000).
func (d *Device) SetBrightness(ctx context.Context, level float64) error {
	if level < 0 {
		level = 0
	}
	if level > 100 {
		level = 100
	}

	const (
		minVal = 0x0000 // Changed from 400 to 0 to be safe
		maxVal = 0xD2F0 // 54000
	)

	val := uint16((level / 100.0) * float64(maxVal))

	// LG UltraFine variants:
	// Some use 16-bit 0-54000 at offset 1/2.
	// Some use 8-bit 0-100 at offset 5.
	// We send both to be safe.
	data := make([]byte, 7)
	data[0] = 0 // Report ID
	data[1] = uint8(val & 0xFF)
	data[2] = uint8((val >> 8) & 0xFF)
	data[3] = 0
	data[4] = 0
	data[5] = uint8(level)
	data[6] = 0

	return d.SendFeatureReport(0, data)
}

// GetBrightness returns the current brightness level (0-100).
func (d *Device) GetBrightness(ctx context.Context) (float64, error) {
	// First check Monitor Page (0x80)
	buffer := make([]byte, 7)
	n, err := d.GetFeatureReport(0, buffer)
	if err != nil {
		return 0, err
	}
	if n < 3 {
		return 0, fmt.Errorf("insufficient data received: %d bytes", n)
	}

	// LG UltraFine: bytes are [ID, LSB, MSB, 0, 0, Val8, 0]
	// Try 16-bit first
	val16 := uint16(buffer[1]) | (uint16(buffer[2]) << 8)
	val8 := buffer[5]

	const (
		maxVal16 = 0xD2F0
	)

	var level float64
	if val16 > 0 {
		level = (float64(val16) / float64(maxVal16)) * 100.0
	} else if val8 > 0 {
		level = float64(val8)
	}

	return level, nil
}

// FindBrightnessDevices searches for devices matching the Monitor Control Usage Page (0x80)
// or LG specific Vendor Page (0xFF00).
func FindBrightnessDevices() ([]DeviceInfo, error) {
	// First try Monitor Page (standard)
	devices, err := Enumerate(UsagePageStandardMonitor, UsageBrightness)
	if err != nil {
		return nil, err
	}

	// Then try LG Vendor Page (specific to UltraFine)
	lgDevices, err := Enumerate(UsagePageLGVendor, UsageBrightness)
	if err == nil {
		devices = append(devices, lgDevices...)
	}

	return devices, nil
}
