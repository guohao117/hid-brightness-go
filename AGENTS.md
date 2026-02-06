# Agent Guidelines: ultrafine-brightness

This document provides instructions and standards for agentic coding tools operating in this repository.

## Project Overview
`ultrafine-brightness` is a cross-platform utility (currently targeting Windows, with macOS/Linux support planned) that provides automatic brightness adjustment for LG UltraFine monitors. It utilizes ambient light sensors (ALS) from the monitor or the system. 

The current Windows implementation uses a C++/WinRT bridge. Future macOS and Linux support will leverage `hidapi` (e.g., via `github.com/karalabe/hid`) for HID communication.

## Build and Development Commands

### Platform-Specific Requirements
- **Windows:**
  - Go 1.25+, CMake 3.15+, Visual Studio 2022 (with C++ WinRT support).
  - Uses `internal/bridge/winrt_bridge.cpp` for HID and Sensor APIs.
- **macOS/Linux (Planned):**
  - Will utilize `hidapi` and platform-specific ALS APIs.

### Core Commands
- **Build Release App (Windows - CMake):**
  ```bash
  cmake -B build
  cmake --build build --config Release
  ```
  **IMPORTANT:** Always use CMake to build the release application. This ensures both the C++ `winrt_bridge.dll` and the Go executables are built and placed correctly.

- **Build (Go Only - Debug/Temporary):**
  ```bash
  # Windows: Requires winrt_bridge.dll in path
  go build -ldflags "-H=windowsgui" -o bin/Release/ultrafine-brightness.exe ./cmd/hid-brightness
  ```
  *Note: Only use `go build` directly for debugging or building temporary programs. Do not use it for release builds.*

- **Run Tests:**
  ```bash
  go test ./...
  go test -v ./pkg/hid
  ```

- **Run Single Test:**
  ```bash
  go test -v -run TestName ./pkg/path/to/package
  ```

- **Linting & Formatting:**
  ```bash
  go fmt ./...
  go vet ./...
  ```

## Code Style & Architecture

### Go Style Guidelines
1.  **Formatting:** Always use `gofmt`.
2.  **Imports:** Group standard library imports first, then a blank line, then third-party imports.
3.  **Naming:**
    - Use `PascalCase` for exported identifiers.
    - Use `camelCase` for private identifiers.
4.  **Error Handling:**
    - Handle errors immediately: `if err != nil { return err }`.
    - Use `fmt.Errorf` with `%w` for wrapping.
5.  **Concurrency:**
    - Use `sync.Mutex` to protect hardware access.
    - Hardware operations should be thread-safe.
6.  **Context:**
    - Always accept `context.Context` for hardware I/O or long-running tasks.
7.  **Platform Specifics:**
    - Use build tags: `//go:build windows`, `//go:build darwin`, `//go:build linux`.
    - Keep platform-specific logic in `*_windows.go`, `*_darwin.go`, or `*_linux.go`.
    - Shared interfaces (e.g., `pkg/hid/hid.go`, `pkg/als/als.go`) must remain platform-agnostic.

### Windows C++ (WinRT Bridge) Guidelines
1.  **Standard:** C++17 with WinRT support.
2.  **Interface:** Use `extern "C"` for functions exported in `winrt_bridge.h`.
3.  **Error Handling:** Wrap everything in `try...catch (...)` and return error codes (e.g., `-1`).
4.  **Memory:** Go manages the lifecycle of objects via `winrt_*_close` functions.

### Project Structure
- `cmd/`: Application entry points.
- `internal/bridge/`: Windows-specific C++ WinRT bridge.
- `pkg/als/`: Ambient Light Sensor abstraction and platform implementations.
- `pkg/hid/`: HID device abstraction and LG-specific logic.
- `bin/`: Compiled binaries.

## Agent Specific Rules
- **Build Process:** ALWAYS use `cmake` to build the release application. `go build` should ONLY be used for debugging or building temporary programs.
- **Cross-Platform Readiness:** When adding features, ensure the core logic in `pkg/` remains decoupled from platform-specific implementations.
- **HIDAPI Transition:** For non-Windows platforms, prefer `hidapi` compatible libraries. Avoid adding new C++ bridges if a Go library can achieve the same result.
- **Hardware Simulation:** Use mocks/interfaces for testing instead of real hardware.
- **Bridge Policy:** Only use C++ for functionality that cannot be reasonably implemented in Go (e.g., complex WinRT APIs).
- **Safety:** Use `unsafe.Pointer` only when interacting with C/C++ bridges, ensuring memory is correctly pinned or managed.

## UI/UX Standards (Systray)
- Runs as a background task with a tray icon using `github.com/getlantern/systray`.
- Maintain the menu structure: Status, Brightness level, Pause, Restart, and Quit.
- Provide visual feedback for status changes (e.g., "Disconnected").

