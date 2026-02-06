//go:build windows

package main

import (
	"context"
	"fmt"
	"github.com/plissken/ultrafine-brightness/internal/config"
	"github.com/plissken/ultrafine-brightness/internal/ui"
	"github.com/plissken/ultrafine-brightness/pkg/als"
	"github.com/plissken/ultrafine-brightness/pkg/hid"
	"io"
	"log"
	"math"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"time"
)

func init() {
	logPath := filepath.Join(os.TempDir(), "ultrafine-brightness.log")
	if logFile, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644); err == nil {
		log.SetOutput(io.MultiWriter(os.Stdout, logFile))
		log.Printf("logging to %s", logPath)
	}
}

var (
	appConfig *config.Config
)

type AutoBrightness struct {
	config            *config.Config
	configUpdate      chan *config.Config
	sensor            als.ALS
	display           *hid.Device
	currentBrightness float64
	targetBrightness  float64
	lastLux           float64
	mu                sync.Mutex
	paused            bool
	onUpdate          func(float64)
}

const (
	defaultBrightnessStep = 0.1
	luxLowThreshold       = 0.1
)

func NewAutoBrightness(cfg *config.Config, updateChan chan *config.Config, sensor als.ALS, display *hid.Device, onUpdate func(float64)) *AutoBrightness {
	var initial float64
	if display != nil {
		initial, _ = display.GetBrightness(context.Background())
	}
	if onUpdate != nil {
		onUpdate(initial)
	}
	return &AutoBrightness{
		config:            cfg,
		configUpdate:      updateChan,
		sensor:            sensor,
		display:           display,
		currentBrightness: initial,
		targetBrightness:  initial,
		onUpdate:          onUpdate,
	}
}

func (ab *AutoBrightness) MapLuxToBrightness(lux float64) float64 {
	if lux <= luxLowThreshold {
		return ab.config.MinBrightness
	}
	clampedLux := math.Max(ab.config.MinLux, math.Min(lux, ab.config.MaxLux))
	perceived := math.Pow(clampedLux, ab.config.Alpha) / math.Pow(ab.config.MaxLux, ab.config.Alpha)
	brightness := math.Pow(perceived, 1.0/ab.config.Gamma) * 100.0
	if brightness < ab.config.MinBrightness {
		brightness = ab.config.MinBrightness
	}
	return brightness
}

func (ab *AutoBrightness) Run(ctx context.Context) {
	readings, err := ab.sensor.SubscribeLux(ctx, ab.config.TargetThreshold)
	if err != nil {
		log.Printf("Failed to subscribe: %v", err)
		return
	}
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case newCfg := <-ab.configUpdate:
			ab.mu.Lock()
			ab.config = newCfg
			if !ab.paused {
				ab.targetBrightness = ab.MapLuxToBrightness(ab.lastLux)
			}
			ab.mu.Unlock()
			log.Printf("Config updated: Gamma=%.1f, MinBrightness=%.1f", newCfg.Gamma, newCfg.MinBrightness)

		case lux, ok := <-readings:
			if !ok {
				return
			}
			ab.mu.Lock()
			ab.lastLux = lux
			if !ab.paused {
				ab.targetBrightness = ab.MapLuxToBrightness(lux)
			}
			ab.mu.Unlock()

		case <-ticker.C:
			ab.mu.Lock()
			if !ab.paused && math.Abs(ab.currentBrightness-ab.targetBrightness) > defaultBrightnessStep {
				step := ab.config.SlewRate * defaultBrightnessStep
				diff := ab.targetBrightness - ab.currentBrightness
				if math.Abs(diff) < step {
					ab.currentBrightness = ab.targetBrightness
				} else if diff > 0 {
					ab.currentBrightness += step
				} else {
					ab.currentBrightness -= step
				}

				if err := ab.display.SetBrightness(ctx, ab.currentBrightness); err != nil {
					log.Printf("Failed to set brightness: %v", err)
				}
				if ab.onUpdate != nil {
					ab.onUpdate(ab.currentBrightness)
				}
			}
			ab.mu.Unlock()

		case <-ctx.Done():
			return
		}
	}
}

func (ab *AutoBrightness) SetPaused(paused bool) {
	ab.mu.Lock()
	defer ab.mu.Unlock()
	ab.paused = paused
}

func main() {
	var err error
	appConfig, err = config.LoadConfig(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	runWindowsApp()
}

func runWindowsApp() {
	restartChan := make(chan struct{}, 1)
	updateChan := make(chan *config.Config, 1)
	restartChan <- struct{}{} // Initial start

	ctx, cancel := context.WithCancel(context.Background())
	var currentCancel context.CancelFunc
	var mu sync.Mutex
	isPaused := false
	var currentBrightness float64
	var brightnessMu sync.RWMutex
	var tray ui.Tray

	setTooltip := func(text string) {
		if tray != nil {
			tray.SetTooltip(text)
		}
	}

	setRunningTooltip := func(prefix string) {
		brightnessMu.RLock()
		level := currentBrightness
		brightnessMu.RUnlock()
		setTooltip(fmt.Sprintf("LG UltraFine - %s - %.0f%%", prefix, level))
	}

	tray, err := ui.NewTray(ui.TrayOptions{
		Tooltip: "LG UltraFine Auto Brightness",
	}, ui.TrayCallbacks{
		OnSettingsRequested: func(anchorX, anchorY int) {
			brightnessMu.RLock()
			level := currentBrightness
			brightnessMu.RUnlock()
			tray.ShowSettings(ui.FlyoutData{
				CurrentBrightness: level,
				Gamma:             appConfig.Gamma,
				MinBrightness:     appConfig.MinBrightness,
				AnchorX:           anchorX,
				AnchorY:           anchorY,
			}, func(gamma, minBrightness float64) {
				cfg := *appConfig
				cfg.Gamma = gamma
				cfg.MinBrightness = minBrightness
				appConfig = &cfg
				if err := config.SaveConfig("config.txt", appConfig); err != nil {
					log.Printf("Failed to save config: %v", err)
				}
				select {
				case updateChan <- appConfig:
				default:
				}
			})
		},
		OnPauseToggled: func(paused bool) {
			mu.Lock()
			isPaused = paused
			if isPaused {
				setRunningTooltip("Paused")
			} else {
				setRunningTooltip("Running")
			}
			mu.Unlock()

			select {
			case restartChan <- struct{}{}:
			default:
			}
		},
		OnRestart: func() {
			log.Println("Manual restart triggered...")
			select {
			case restartChan <- struct{}{}:
			default:
			}
		},
		OnQuit: func() {
			cancel()
			tray.Close()
		},
	})
	if err != nil {
		log.Fatalf("Failed to initialize tray UI: %v", err)
	}
	defer tray.Close()

	// Main controller lifecycle
	go func() {
		var display *hid.Device
		var sensor als.ALS
		var alsManager als.ALSManager

		for {
			select {
			case <-ctx.Done():
				// Final cleanup
				if display != nil {
					display.Close()
				}
				if sensor != nil {
					sensor.Close()
				}
				if alsManager != nil {
					alsManager.Close()
				}
				return
			case <-restartChan:
				// Cleanup previous run
				if currentCancel != nil {
					currentCancel()
				}
				if display != nil {
					display.Close()
					display = nil
				}
				if sensor != nil {
					sensor.Close()
					sensor = nil
				}
				if alsManager != nil {
					alsManager.Close()
					alsManager = nil
				}

				setTooltip("LG UltraFine - Searching for hardware...")

				// Re-initialize
				log.Println("Searching for HID devices...")
				devices, err := hid.FindBrightnessDevices()
				if err != nil || len(devices) == 0 {
					log.Println("No monitor found. Please check connection and click Restart.")
					setTooltip("LG UltraFine - Disconnected")
					continue
				}

				id := ""
				for _, d := range devices {
					if d.UsagePage == hid.UsagePageStandardMonitor {
						id = d.ID
						break
					}
				}
				if id == "" {
					id = devices[0].ID
				}

				display, err = hid.Open(id)
				if err != nil {
					log.Printf("Failed to open HID device: %v", err)
					setTooltip("LG UltraFine - Open Display Failed")
					continue
				}

				alsManager, err = als.NewManager()
				if err != nil {
					log.Printf("Failed to initialize ALS manager: %v", err)
					display.Close()
					display = nil
					setTooltip("LG UltraFine - ALS Init Failed")
					continue
				}

				runCtx, runCancel := context.WithCancel(ctx)
				currentCancel = runCancel

				sensor, err = alsManager.Default(runCtx)
				if err != nil {
					log.Printf("Failed to open light sensor: %v", err)
					display.Close()
					display = nil
					alsManager.Close()
					alsManager = nil
					setTooltip("LG UltraFine - Sensor Not Found")
					continue
				}

				updateUI := func(level float64) {
					brightnessMu.Lock()
					currentBrightness = level
					brightnessMu.Unlock()
					mu.Lock()
					pausedNow := isPaused
					mu.Unlock()
					if pausedNow {
						setRunningTooltip("Paused")
					} else {
						setRunningTooltip("Running")
					}
					if tray != nil {
						tray.UpdateBrightness(level)
					}
				}

				controller := NewAutoBrightness(appConfig, updateChan, sensor, display, updateUI)

				// Apply current pause state
				mu.Lock()
				controller.SetPaused(isPaused)
				if isPaused {
					setRunningTooltip("Paused")
				} else {
					setRunningTooltip("Running")
				}
				mu.Unlock()

				go controller.Run(runCtx)
				log.Println("Controller started successfully")
			}
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	go func() {
		<-sigChan
		cancel()
		tray.Close()
	}()

	tray.Run()
}
