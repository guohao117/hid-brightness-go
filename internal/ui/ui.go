package ui

type FlyoutData struct {
	CurrentBrightness float64
	Gamma             float64
	MinBrightness     float64
	AnchorX           int
	AnchorY           int
}

type TrayCallbacks struct {
	OnSettingsRequested func(anchorX, anchorY int)
	OnPauseToggled      func(paused bool)
	OnRestart           func()
	OnQuit              func()
}

type TrayOptions struct {
	Tooltip string
}

type Tray interface {
	SetTooltip(text string)
	ShowSettings(data FlyoutData, onApply func(gamma, minBrightness float64))
	UpdateBrightness(level float64)
	Run()
	Close()
}
