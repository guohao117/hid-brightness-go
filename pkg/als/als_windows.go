//go:build windows
// +build windows

package als

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"sync"
	"syscall"
	"unsafe"
)

var (
	vidRegexp = regexp.MustCompile(`[vV][iI][dD]_([0-9a-fA-F]{4})`)
	pidRegexp = regexp.MustCompile(`[pP][iI][dD]_([0-9a-fA-F]{4})`)
)

var (
	modWinrtBridge   = syscall.NewLazyDLL("winrt_bridge.dll")
	procInitialize   = modWinrtBridge.NewProc("winrt_als_initialize")
	procOpenDefault  = modWinrtBridge.NewProc("winrt_als_open_default")
	procGetLux       = modWinrtBridge.NewProc("winrt_als_get_lux")
	procSetThreshold = modWinrtBridge.NewProc("winrt_als_set_report_threshold")
	procSubscribe    = modWinrtBridge.NewProc("winrt_als_subscribe_reading_changed")
	procUnsubscribe  = modWinrtBridge.NewProc("winrt_als_unsubscribe_reading_changed")
	procEnumerate    = modWinrtBridge.NewProc("winrt_als_enumerate")
	procGetCount     = modWinrtBridge.NewProc("winrt_als_get_device_count")
	procGetProps     = modWinrtBridge.NewProc("winrt_als_get_device_properties")
	procFreeEnum     = modWinrtBridge.NewProc("winrt_als_free_enumeration")
	procOpenById     = modWinrtBridge.NewProc("winrt_als_open_by_id")
	procClose        = modWinrtBridge.NewProc("winrt_als_close")
	procUninitialize = modWinrtBridge.NewProc("winrt_als_uninitialize")

	initOnce sync.Once
	initErr  error

	// registry for event callbacks
	subRegistry = make(map[uintptr]chan<- float64)
	subMu       sync.Mutex
	nextSubID   uintptr = 1
)

func registerSubscription(ch chan<- float64) uintptr {
	subMu.Lock()
	defer subMu.Unlock()
	id := nextSubID
	nextSubID++
	subRegistry[id] = ch
	return id
}

func unregisterSubscription(id uintptr) {
	subMu.Lock()
	defer subMu.Unlock()
	delete(subRegistry, id)
}

func alsCallback(id uintptr, luxBits uint64) uintptr {
	subMu.Lock()
	ch, ok := subRegistry[id]
	subMu.Unlock()

	if ok {
		lux := math.Float64frombits(luxBits)
		select {
		case ch <- lux:
		default:
			// Buffer full, skip
		}
	}
	return 0
}

var globalAlsCallback = syscall.NewCallback(alsCallback)

// ensureWinRTInitialized ensures that winrt_als_initialize is called exactly once.
func ensureWinRTInitialized() error {
	initOnce.Do(func() {
		ret, _, _ := procInitialize.Call()
		if int32(ret) != 0 {
			initErr = fmt.Errorf("failed to initialize WinRT apartment")
		}
	})
	return initErr
}

type alsWindows struct {
	handle uintptr
	mu     sync.Mutex
}

type double float64

func (a *alsWindows) GetLux(ctx context.Context) (float64, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.handle == 0 {
		return 0, fmt.Errorf("sensor is closed")
	}

	var lux double
	ret, _, _ := procGetLux.Call(a.handle, uintptr(unsafe.Pointer(&lux)))
	if int32(ret) != 0 {
		return 0, fmt.Errorf("failed to get lux reading from WinRT")
	}

	return float64(lux), nil
}

func (a *alsWindows) SubscribeLux(ctx context.Context, threshold float64) (<-chan float64, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.handle == 0 {
		return nil, fmt.Errorf("sensor is closed")
	}

	ch := make(chan float64, 1)

	// Set hardware threshold if provided
	if threshold > 0 {
		procSetThreshold.Call(a.handle, uintptr(math.Float64bits(threshold)))
	}

	subID := registerSubscription(ch)
	token, _, _ := procSubscribe.Call(a.handle, globalAlsCallback, subID)

	if token == 0 {
		unregisterSubscription(subID)
		return nil, fmt.Errorf("failed to subscribe to reading changed event")
	}

	handle := a.handle
	go func() {
		<-ctx.Done()
		procUnsubscribe.Call(handle, token)
		unregisterSubscription(subID)
		close(ch)
	}()

	return ch, nil
}

func (a *alsWindows) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.handle != 0 {
		procClose.Call(a.handle)
		a.handle = 0
	}
	return nil
}

type alsWindowsManager struct{}

func (m *alsWindowsManager) List(ctx context.Context) ([]ALSDeviceInfo, error) {
	ret, _, _ := procEnumerate.Call()
	h := uintptr(ret)
	if h == 0 {
		return []ALSDeviceInfo{}, nil
	}
	defer procFreeEnum.Call(h)

	countRet, _, _ := procGetCount.Call(h)
	count := int(countRet)

	devices := make([]ALSDeviceInfo, 0, count)
	for i := 0; i < count; i++ {
		idBuf := make([]byte, 1024)
		nameBuf := make([]byte, 1024)
		var vid, pid uint16
		procGetProps.Call(
			h,
			uintptr(i),
			uintptr(unsafe.Pointer(&idBuf[0])),
			uintptr(len(idBuf)),
			uintptr(unsafe.Pointer(&nameBuf[0])),
			uintptr(len(nameBuf)),
			uintptr(unsafe.Pointer(&vid)),
			uintptr(unsafe.Pointer(&pid)),
		)

		id := bytePtrToString(&idBuf[0])
		name := bytePtrToString(&nameBuf[0])

		// Fallback: Try to parse VID/PID from ID string if they weren't found in properties
		if vid == 0 {
			if matches := vidRegexp.FindStringSubmatch(id); len(matches) > 1 {
				if v, err := strconv.ParseUint(matches[1], 16, 16); err == nil {
					vid = uint16(v)
				}
			}
		}
		if pid == 0 {
			if matches := pidRegexp.FindStringSubmatch(id); len(matches) > 1 {
				if v, err := strconv.ParseUint(matches[1], 16, 16); err == nil {
					pid = uint16(v)
				}
			}
		}

		devices = append(devices, ALSDeviceInfo{
			VendorID:  vid,
			ProductID: pid,
			Path:      id,
			name:      name,
		})
	}

	return devices, nil
}

func (m *alsWindowsManager) Open(ctx context.Context, deviceInfo ALSDeviceInfo) (ALS, error) {
	pathPtr, err := syscall.BytePtrFromString(deviceInfo.Path)
	if err != nil {
		return nil, err
	}

	ret, _, _ := procOpenById.Call(uintptr(unsafe.Pointer(pathPtr)))
	h := uintptr(ret)
	if h == 0 {
		return nil, fmt.Errorf("failed to open light sensor: %s", deviceInfo.Path)
	}

	return &alsWindows{
		handle: h,
	}, nil
}

func (m *alsWindowsManager) Default(ctx context.Context) (ALS, error) {
	ret, _, _ := procOpenDefault.Call()
	h := uintptr(ret)
	if h == 0 {
		return nil, fmt.Errorf("no light sensor found on this system")
	}

	return &alsWindows{
		handle: h,
	}, nil
}

func (m *alsWindowsManager) Close() error {
	uninitialize()
	return nil
}

func uninitialize() {
	initOnce.Do(func() {}) // Ensure we don't uninit if never init, though initErr check is better
	if initErr == nil {
		procUninitialize.Call()
	}
}

func newManager() (ALSManager, error) {
	if err := ensureWinRTInitialized(); err != nil {
		return nil, err
	}
	return &alsWindowsManager{}, nil
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
