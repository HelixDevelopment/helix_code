# Hardware Package

The `hardware` package provides hardware automation and testing for the HelixCode platform.

## Overview

This package handles:
- Hardware device detection
- GPIO control (Raspberry Pi, Arduino)
- Serial communication
- Hardware test automation
- Device monitoring

## Key Types

### HardwareManager

```go
type HardwareManager struct {
    devices  map[string]Device
    serial   *SerialManager
    gpio     *GPIOManager
    config   *Config
}
```

### Device

```go
type Device interface {
    ID() string
    Type() DeviceType
    Connect(ctx context.Context) error
    Disconnect(ctx context.Context) error
    Status() Status
}
```

## Usage

### Detecting Devices

```go
import "dev.helix.code/internal/hardware"

manager := hardware.NewManager(config)
devices, err := manager.DetectDevices(ctx)
```

### Serial Communication

```go
// Open serial port
serial, err := manager.OpenSerial("/dev/ttyUSB0", 9600)

// Send data
err := serial.Write([]byte("AT\r\n"))

// Read response
data, err := serial.Read(1024)
```

### GPIO Control

```go
// Setup GPIO pin
err := manager.SetupPin(17, hardware.PinOutput)

// Write to pin
err := manager.WritePin(17, hardware.High)

// Read from pin
value, err := manager.ReadPin(18)
```

## Supported Hardware

- Raspberry Pi
- Arduino
- ESP32/ESP8266
- USB serial devices
- GPIO-capable boards

## Configuration

```yaml
hardware:
  enabled: true
  auto_detect: true
  serial:
    default_baud: 9600
  gpio:
    platform: "raspberry_pi"
```

## Testing

```bash
go test -v ./internal/hardware/...
```

## Notes

- Requires appropriate permissions for hardware access
- Use with caution on production systems
- Test thoroughly before hardware automation
