//go:build windows

package ui

import (
	"fmt"
	"math"
	"os"
	"unsafe"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"github.com/lxn/win"
)

type windowsTray struct {
	mw                 *walk.MainWindow
	notifyIcon         *walk.NotifyIcon
	pauseAction        *walk.Action
	paused             bool
	flyoutDlg          *walk.Dialog
	flyoutLabel        *walk.Label
	settingsWindowOpen bool
}

func NewTray(options TrayOptions, callbacks TrayCallbacks) (Tray, error) {
	mw, err := walk.NewMainWindow()
	if err != nil {
		return nil, fmt.Errorf("create main window: %w", err)
	}
	mw.SetVisible(false)

	notifyIcon, err := walk.NewNotifyIcon(mw)
	if err != nil {
		mw.Dispose()
		return nil, fmt.Errorf("create tray icon: %w", err)
	}

	// Load icon from embedded resource (ID 1)
	if icon, err := walk.NewIconFromResourceId(1); err == nil {
		_ = notifyIcon.SetIcon(icon)
	} else if exePath, err := os.Executable(); err == nil {
		// Fallback: try to extract from executable file
		if icon, err := walk.NewIconExtractedFromFileWithSize(exePath, 0, 16); err == nil {
			_ = notifyIcon.SetIcon(icon)
		}
	}

	if notifyIcon.Icon() == nil {
		// Final fallback: system info icon
		if icon := walk.IconInformation(); icon != nil {
			_ = notifyIcon.SetIcon(icon)
		}
	}

	tooltip := options.Tooltip
	if tooltip == "" {
		tooltip = "LG UltraFine Auto Brightness"
	}
	_ = notifyIcon.SetToolTip(tooltip)

	if err := notifyIcon.SetVisible(true); err != nil {
		notifyIcon.Dispose()
		mw.Dispose()
		return nil, fmt.Errorf("show tray icon: %w", err)
	}

	pauseAction := walk.NewAction()
	_ = pauseAction.SetText("Pause")
	restartAction := walk.NewAction()
	_ = restartAction.SetText("Restart")
	quitAction := walk.NewAction()
	_ = quitAction.SetText("Quit")

	notifyIcon.ContextMenu().Actions().Add(pauseAction)
	notifyIcon.ContextMenu().Actions().Add(restartAction)
	notifyIcon.ContextMenu().Actions().Add(walk.NewSeparatorAction())
	notifyIcon.ContextMenu().Actions().Add(quitAction)

	tray := &windowsTray{
		mw:          mw,
		notifyIcon:  notifyIcon,
		pauseAction: pauseAction,
	}

	notifyIcon.MouseUp().Attach(func(x, y int, button walk.MouseButton) {
		if button != walk.LeftButton {
			return
		}
		anchorX, anchorY := getCursorScreenPosition()
		if anchorX == 0 && anchorY == 0 {
			anchorX, anchorY = x, y
		}
		if callbacks.OnSettingsRequested != nil {
			callbacks.OnSettingsRequested(anchorX, anchorY)
		}
	})

	pauseAction.Triggered().Attach(func() {
		tray.paused = !tray.paused
		if tray.paused {
			_ = tray.pauseAction.SetText("Resume")
		} else {
			_ = tray.pauseAction.SetText("Pause")
		}
		if callbacks.OnPauseToggled != nil {
			callbacks.OnPauseToggled(tray.paused)
		}
	})

	restartAction.Triggered().Attach(func() {
		if callbacks.OnRestart != nil {
			callbacks.OnRestart()
		}
	})

	quitAction.Triggered().Attach(func() {
		if callbacks.OnQuit != nil {
			callbacks.OnQuit()
		}
	})

	return tray, nil
}

func (t *windowsTray) SetTooltip(text string) {
	t.mw.Synchronize(func() {
		_ = t.notifyIcon.SetToolTip(text)
	})
}

func (t *windowsTray) ShowSettings(data FlyoutData, onApply func(gamma, minBrightness float64)) {
	t.showSettingsFlyout(data, onApply)
}

func (t *windowsTray) UpdateBrightness(level float64) {
	t.mw.Synchronize(func() {
		if !t.settingsWindowOpen || t.flyoutLabel == nil {
			return
		}
		_ = t.flyoutLabel.SetText(fmt.Sprintf("Current Brightness: %.0f%%", level))
	})
}

func (t *windowsTray) Run() {
	t.mw.Run()
}

func (t *windowsTray) Close() {
	t.mw.Synchronize(func() {
		if t.notifyIcon != nil {
			_ = t.notifyIcon.Dispose()
		}
		if t.mw != nil {
			t.mw.Dispose()
		}
		walk.App().Exit(0)
	})
}

func (t *windowsTray) showSettingsFlyout(data FlyoutData, onApply func(gamma, minBrightness float64)) {
	if t.settingsWindowOpen {
		return
	}
	t.settingsWindowOpen = true

	gammaValue := int(math.Round(data.Gamma * 10))
	minBrightnessValue := int(math.Round(data.MinBrightness))

	var dlg *walk.Dialog
	var brightnessLabel *walk.Label
	var gammaLabel *walk.Label
	var minBrightnessLabel *walk.Label
	var gammaSlider *walk.Slider
	var minBrightnessSlider *walk.Slider

	dlgDef := Dialog{
		AssignTo:  &dlg,
		Title:     "Brightness Settings",
		FixedSize: true,
		MinSize:   Size{Width: 420, Height: 210},
		MaxSize:   Size{Width: 420, Height: 210},
		Layout:    VBox{Margins: Margins{Left: 12, Top: 12, Right: 12, Bottom: 12}, Spacing: 10},
		Children: []Widget{
			Label{AssignTo: &brightnessLabel, Text: fmt.Sprintf("Current Brightness: %.0f%%", data.CurrentBrightness)},
			Label{AssignTo: &gammaLabel, Text: fmt.Sprintf("Gamma: %.1f", data.Gamma)},
			Slider{
				AssignTo: &gammaSlider,
				MinValue: 10,
				MaxValue: 40,
				Value:    gammaValue,
				OnValueChanged: func() {
					gammaLabel.SetText(fmt.Sprintf("Gamma: %.1f", float64(gammaSlider.Value())/10.0))
				},
			},
			Label{AssignTo: &minBrightnessLabel, Text: fmt.Sprintf("Min Brightness: %.0f%%", data.MinBrightness)},
			Slider{
				AssignTo: &minBrightnessSlider,
				MinValue: 0,
				MaxValue: 100,
				Value:    minBrightnessValue,
				OnValueChanged: func() {
					minBrightnessLabel.SetText(fmt.Sprintf("Min Brightness: %d%%", minBrightnessSlider.Value()))
				},
			},
			Composite{
				Layout: HBox{Spacing: 8},
				Children: []Widget{
					HSpacer{},
					PushButton{
						Text: "Cancel",
						OnClicked: func() {
							dlg.Cancel()
						},
					},
					PushButton{
						Text: "Apply",
						OnClicked: func() {
							if onApply != nil {
								onApply(float64(gammaSlider.Value())/10.0, float64(minBrightnessSlider.Value()))
							}
							dlg.Accept()
						},
					},
				},
			},
		},
	}

	if err := dlgDef.Create(nil); err != nil {
		t.settingsWindowOpen = false
		return
	}

	// Store references for updates
	t.flyoutDlg = dlg
	t.flyoutLabel = brightnessLabel

	hwnd := uintptr(dlg.Handle())

	// Remove window decorations and make it a tool window
	h := win.HWND(hwnd)
	style := win.GetWindowLong(h, win.GWL_STYLE)
	style &^= win.WS_CAPTION | win.WS_THICKFRAME | win.WS_SYSMENU
	var wsPopup uint32 = win.WS_POPUP
	style |= int32(wsPopup) | win.WS_BORDER
	win.SetWindowLong(h, win.GWL_STYLE, style)

	exStyle := win.GetWindowLong(h, win.GWL_EXSTYLE)
	exStyle |= win.WS_EX_TOOLWINDOW
	win.SetWindowLong(h, win.GWL_EXSTYLE, exStyle)

	// Apply the style changes
	win.SetWindowPos(h, 0, 0, 0, 0, 0, win.SWP_FRAMECHANGED|win.SWP_NOMOVE|win.SWP_NOSIZE|win.SWP_NOZORDER)

	// Force a resize to trigger layout recalculation without decorations
	bounds := dlg.BoundsPixels()
	dlg.SetBoundsPixels(bounds)

	// Force layout recalculation and repaint after style changes
	dlg.RequestLayout()
	dlg.Invalidate()

	// Ensure the window is the foreground window so that clicking the taskbar
	// or other windows will properly trigger the Deactivating event.
	win.SetForegroundWindow(h)

	dlg.Deactivating().Attach(func() {
		dlg.Cancel()
	})

	dlg.Closing().Attach(func(canceled *bool, reason walk.CloseReason) {
		t.settingsWindowOpen = false
		t.flyoutDlg = nil
		t.flyoutLabel = nil
	})

	// Show the window first so it gets drawn and its actual size is calculated
	dlg.Show()

	// Now that it's shown and sized, position it correctly
	positionFlyoutWindow(hwnd, data.AnchorX, data.AnchorY)
}

func positionFlyoutWindow(hwnd uintptr, anchorX, anchorY int) {
	h := win.HWND(hwnd)
	if h == 0 {
		return
	}

	var winRect win.RECT
	if !win.GetWindowRect(h, &winRect) {
		return
	}

	// Calculate the required top-left position for the window
	// so that the bottom-right aligns with anchorX, anchorY.
	width := int(winRect.Right - winRect.Left)
	height := int(winRect.Bottom - winRect.Top)
	x := anchorX - width
	y := anchorY - height

	// Get work area to ensure the window stays on screen
	var wa win.RECT
	const spiGetWorkArea = 0x0030
	if win.SystemParametersInfo(spiGetWorkArea, 0, unsafe.Pointer(&wa), 0) {
		if x < int(wa.Left) {
			x = int(wa.Left)
		}
		if y < int(wa.Top) {
			y = int(wa.Top)
		}
		if x+width > int(wa.Right) {
			x = int(wa.Right) - width
		}
		if y+height > int(wa.Bottom) {
			y = int(wa.Bottom) - height
		}
	}

	const swpNoSize = 0x0001
	const swpShowWindow = 0x0040
	win.SetWindowPos(
		h,
		win.HWND_TOPMOST,
		int32(x),
		int32(y),
		0,
		0,
		swpNoSize|swpShowWindow,
	)
}

func getCursorScreenPosition() (int, int) {
	var p win.POINT
	if win.GetCursorPos(&p) {
		return int(p.X), int(p.Y)
	}
	return 0, 0
}
