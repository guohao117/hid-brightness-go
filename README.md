# ultrafine-brightness

A cross-platform utility for automatic brightness adjustment of LG UltraFine monitors, using ambient light sensors (ALS) from the monitor or system.

## Features
- Automatic brightness adjustment for LG UltraFine monitors
- Uses monitor or system ambient light sensors
- Windows support (C++/WinRT bridge)
- Planned macOS/Linux support (via hidapi)
- System tray UI with status, brightness, pause, restart, and quit

## Build Instructions
### Windows
1. **Requirements:**
   - Go 1.25+
   - CMake 3.15+
   - Visual Studio 2022 (with C++ WinRT support)
2. **Build Release:**
   ```sh
   cmake -B build
   cmake --build build --config Release
   ```
   This builds both the C++ `winrt_bridge.dll` and Go executables.
3. **Run Tests:**
   ```sh
   go test ./...
   go test -v ./pkg/hid
   ```

### Debug/Temporary Build (Go only)
```sh
# Requires winrt_bridge.dll in path
 go build -ldflags "-H=windowsgui" -o bin/Release/ultrafine-brightness.exe ./cmd/hid-brightness
```

## Project Structure
- `cmd/` — Application entry points
- `internal/bridge/` — Windows-specific C++ WinRT bridge
- `pkg/als/` — Ambient Light Sensor abstraction
- `pkg/hid/` — HID device abstraction and LG-specific logic
- `bin/` — Compiled binaries

## Contributing
- Keep platform-agnostic logic in `pkg/`
- Use mocks/interfaces for hardware testing
- Use C++ only for WinRT APIs not easily accessible from Go

## License
MIT
