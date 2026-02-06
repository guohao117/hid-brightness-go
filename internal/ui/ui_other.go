//go:build !windows

package ui

import "fmt"

type noopTray struct{}

func NewTray(options TrayOptions, callbacks TrayCallbacks) (Tray, error) {
	_ = options
	_ = callbacks
	return &noopTray{}, fmt.Errorf("tray UI is not implemented on this platform")
}

func (n *noopTray) SetTooltip(text string) {
	_ = text
}

func (n *noopTray) ShowSettings(data FlyoutData, onApply func(gamma, minBrightness float64)) {
	_ = data
	_ = onApply
}

func (n *noopTray) UpdateBrightness(level float64) {}

func (n *noopTray) Run() {}

func (n *noopTray) Close() {}
