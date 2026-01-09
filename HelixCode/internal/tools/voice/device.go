package voice

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// AudioDevice represents an audio input device
type AudioDevice struct {
	ID          string // System device ID
	Name        string // Human-readable name
	IsDefault   bool   // Whether this is the system default
	SampleRates []int  // Supported sample rates
	Channels    int    // Number of channels
	IsAvailable bool   // Current availability status
	Driver      string // Audio driver (CoreAudio, ALSA, etc.)
}

// DeviceManager handles audio device enumeration and selection
type DeviceManager struct {
	devices         []AudioDevice
	activeDevice    *AudioDevice
	refreshInterval time.Duration
	mu              sync.RWMutex
}

// NewDeviceManager creates a new device manager
func NewDeviceManager() (*DeviceManager, error) {
	dm := &DeviceManager{
		devices:         []AudioDevice{},
		refreshInterval: 30 * time.Second,
	}

	// Initialize device list
	if err := dm.RefreshDevices(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to initialize devices: %w", err)
	}

	return dm, nil
}

// ListDevices enumerates all available audio input devices
func (d *DeviceManager) ListDevices(ctx context.Context) ([]AudioDevice, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if len(d.devices) == 0 {
		return nil, ErrNoDevicesFound
	}

	// Return a copy to prevent external modifications
	devices := make([]AudioDevice, len(d.devices))
	copy(devices, d.devices)

	return devices, nil
}

// GetDevice retrieves a specific device by ID
func (d *DeviceManager) GetDevice(deviceID string) (*AudioDevice, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	for _, device := range d.devices {
		if device.ID == deviceID {
			deviceCopy := device
			return &deviceCopy, nil
		}
	}

	return nil, ErrDeviceNotFound
}

// GetDefaultDevice returns the system default input device
func (d *DeviceManager) GetDefaultDevice() (*AudioDevice, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	for _, device := range d.devices {
		if device.IsDefault {
			deviceCopy := device
			return &deviceCopy, nil
		}
	}

	// If no default found, return first available
	if len(d.devices) > 0 {
		deviceCopy := d.devices[0]
		return &deviceCopy, nil
	}

	return nil, ErrNoDevicesFound
}

// SelectDevice sets the active device for recording
func (d *DeviceManager) SelectDevice(deviceID string) error {
	device, err := d.GetDevice(deviceID)
	if err != nil {
		return err
	}

	if !device.IsAvailable {
		return ErrDeviceUnavailable
	}

	d.mu.Lock()
	d.activeDevice = device
	d.mu.Unlock()

	return nil
}

// GetActiveDevice returns the currently selected device
func (d *DeviceManager) GetActiveDevice() *AudioDevice {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if d.activeDevice != nil {
		deviceCopy := *d.activeDevice
		return &deviceCopy
	}

	return nil
}

// RefreshDevices updates the device list
func (d *DeviceManager) RefreshDevices(ctx context.Context) error {
	devices, err := d.enumerateDevices()
	if err != nil {
		return err
	}

	d.mu.Lock()
	d.devices = devices
	d.mu.Unlock()

	return nil
}

// ValidateDevice checks if a device is available and properly configured
func (d *DeviceManager) ValidateDevice(device *AudioDevice) error {
	if device == nil {
		return fmt.Errorf("device is nil")
	}

	if !device.IsAvailable {
		return ErrDeviceUnavailable
	}

	if device.Channels < 1 {
		return fmt.Errorf("device has no input channels")
	}

	if len(device.SampleRates) == 0 {
		return fmt.Errorf("device has no supported sample rates")
	}

	return nil
}

// enumerateDevices performs platform-specific device enumeration
func (d *DeviceManager) enumerateDevices() ([]AudioDevice, error) {
	// Platform-specific implementation
	switch runtime.GOOS {
	case "darwin":
		return d.enumerateMacOSDevices()
	case "linux":
		return d.enumerateLinuxDevices()
	case "windows":
		return d.enumerateWindowsDevices()
	default:
		// Return mock device for unsupported platforms
		return d.enumerateMockDevices()
	}
}

// enumerateMacOSDevices enumerates devices on macOS using system_profiler
func (d *DeviceManager) enumerateMacOSDevices() ([]AudioDevice, error) {
	devices := []AudioDevice{}

	// Try to use system_profiler to get audio devices
	cmd := exec.Command("system_profiler", "SPAudioDataType", "-detailLevel", "mini")
	output, err := cmd.Output()
	if err == nil {
		devices = d.parseMacOSAudioDevices(string(output))
	}

	// If no devices found via system_profiler, try using the audio device list command
	if len(devices) == 0 {
		// Fallback: use SwitchAudioSource if available
		cmd = exec.Command("SwitchAudioSource", "-a", "-t", "input")
		output, err = cmd.Output()
		if err == nil {
			for _, line := range strings.Split(string(output), "\n") {
				line = strings.TrimSpace(line)
				if line != "" {
					devices = append(devices, AudioDevice{
						ID:          strings.ReplaceAll(line, " ", "_"),
						Name:        line,
						IsDefault:   len(devices) == 0, // First device is default
						SampleRates: []int{8000, 16000, 44100, 48000},
						Channels:    1,
						IsAvailable: true,
						Driver:      "CoreAudio",
					})
				}
			}
		}
	}

	// If still no devices, return default
	if len(devices) == 0 {
		devices = append(devices, AudioDevice{
			ID:          "default",
			Name:        "Built-in Microphone",
			IsDefault:   true,
			SampleRates: []int{8000, 16000, 44100, 48000},
			Channels:    1,
			IsAvailable: true,
			Driver:      "CoreAudio",
		})
	}

	return devices, nil
}

// parseMacOSAudioDevices parses system_profiler output for audio devices
func (d *DeviceManager) parseMacOSAudioDevices(output string) []AudioDevice {
	var devices []AudioDevice
	var currentDevice *AudioDevice
	isInput := false

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)

		// Check for input device indicator
		if strings.Contains(strings.ToLower(line), "input") {
			isInput = true
		} else if strings.Contains(strings.ToLower(line), "output") {
			isInput = false
		}

		// Look for device names (lines ending with ":")
		if strings.HasSuffix(line, ":") && !strings.HasPrefix(line, "_") {
			deviceName := strings.TrimSuffix(line, ":")
			if isInput && len(deviceName) > 0 {
				if currentDevice != nil {
					devices = append(devices, *currentDevice)
				}
				currentDevice = &AudioDevice{
					ID:          strings.ReplaceAll(deviceName, " ", "_"),
					Name:        deviceName,
					IsDefault:   len(devices) == 0,
					SampleRates: []int{8000, 16000, 44100, 48000},
					Channels:    1,
					IsAvailable: true,
					Driver:      "CoreAudio",
				}
			}
		}
	}

	if currentDevice != nil {
		devices = append(devices, *currentDevice)
	}

	return devices
}

// enumerateLinuxDevices enumerates devices on Linux using ALSA/PulseAudio
func (d *DeviceManager) enumerateLinuxDevices() ([]AudioDevice, error) {
	devices := []AudioDevice{}

	// Try PulseAudio first (more common on modern distros)
	pulseDevices := d.enumeratePulseAudioDevices()
	if len(pulseDevices) > 0 {
		return pulseDevices, nil
	}

	// Fallback to ALSA
	alsaDevices := d.enumerateALSADevices()
	if len(alsaDevices) > 0 {
		return alsaDevices, nil
	}

	// Default fallback device
	devices = append(devices, AudioDevice{
		ID:          "default",
		Name:        "Default Audio Device",
		IsDefault:   true,
		SampleRates: []int{8000, 16000, 44100, 48000},
		Channels:    1,
		IsAvailable: true,
		Driver:      "ALSA",
	})

	return devices, nil
}

// enumeratePulseAudioDevices uses pactl to list PulseAudio sources
func (d *DeviceManager) enumeratePulseAudioDevices() []AudioDevice {
	var devices []AudioDevice

	cmd := exec.Command("pactl", "list", "sources", "short")
	output, err := cmd.Output()
	if err != nil {
		return devices
	}

	// Parse output: index name module sample_spec channel_map state
	// Example: 0 alsa_input.pci-0000_00_1f.3.analog-stereo module-alsa-card s16le 2ch 44100Hz SUSPENDED
	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 5 {
			// Skip monitor sources (they're output monitors, not real inputs)
			if strings.Contains(fields[1], ".monitor") {
				continue
			}

			deviceID := fields[1]
			deviceName := d.formatPulseAudioDeviceName(deviceID)

			// Parse sample rate from sample_spec
			sampleRate := 44100
			if len(fields) >= 4 {
				if rate, err := parseSampleRate(fields[3]); err == nil {
					sampleRate = rate
				}
			}

			// Parse channels
			channels := 1
			if len(fields) >= 4 {
				if ch, err := parseChannels(fields[3]); err == nil {
					channels = ch
				}
			}

			isDefault := len(devices) == 0 || strings.Contains(deviceID, "default")
			isAvailable := strings.ToUpper(fields[len(fields)-1]) != "SUSPENDED"

			devices = append(devices, AudioDevice{
				ID:          deviceID,
				Name:        deviceName,
				IsDefault:   isDefault,
				SampleRates: []int{8000, 16000, 22050, 44100, 48000, sampleRate},
				Channels:    channels,
				IsAvailable: isAvailable,
				Driver:      "PulseAudio",
			})
		}
	}

	return devices
}

// formatPulseAudioDeviceName converts a PulseAudio device ID to a readable name
func (d *DeviceManager) formatPulseAudioDeviceName(deviceID string) string {
	// Remove common prefixes
	name := deviceID
	name = strings.TrimPrefix(name, "alsa_input.")
	name = strings.TrimPrefix(name, "alsa_output.")

	// Replace underscores and dots with spaces
	name = strings.ReplaceAll(name, "_", " ")
	name = strings.ReplaceAll(name, ".", " ")

	// Capitalize first letter of each word
	words := strings.Fields(name)
	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(string(word[0])) + strings.ToLower(word[1:])
		}
	}

	return strings.Join(words, " ")
}

// enumerateALSADevices uses arecord to list ALSA capture devices
func (d *DeviceManager) enumerateALSADevices() []AudioDevice {
	var devices []AudioDevice

	cmd := exec.Command("arecord", "-l")
	output, err := cmd.Output()
	if err != nil {
		return devices
	}

	// Parse output format:
	// card 0: PCH [HDA Intel PCH], device 0: ALC269VC Analog [ALC269VC Analog]
	//   Subdevices: 1/1
	cardRegex := regexp.MustCompile(`card (\d+): (\S+) \[([^\]]+)\], device (\d+): ([^\[]+) \[([^\]]+)\]`)

	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := scanner.Text()
		matches := cardRegex.FindStringSubmatch(line)
		if len(matches) >= 7 {
			cardNum := matches[1]
			deviceNum := matches[4]
			deviceName := strings.TrimSpace(matches[6])

			deviceID := fmt.Sprintf("hw:%s,%s", cardNum, deviceNum)

			devices = append(devices, AudioDevice{
				ID:          deviceID,
				Name:        deviceName,
				IsDefault:   len(devices) == 0,
				SampleRates: []int{8000, 16000, 22050, 44100, 48000},
				Channels:    2,
				IsAvailable: true,
				Driver:      "ALSA",
			})
		}
	}

	return devices
}

// parseSampleRate extracts sample rate from PulseAudio sample spec
func parseSampleRate(sampleSpec string) (int, error) {
	// Format: s16le 2ch 44100Hz
	rateRegex := regexp.MustCompile(`(\d+)Hz`)
	matches := rateRegex.FindStringSubmatch(sampleSpec)
	if len(matches) >= 2 {
		return strconv.Atoi(matches[1])
	}
	return 0, fmt.Errorf("no sample rate found")
}

// parseChannels extracts channel count from PulseAudio sample spec
func parseChannels(sampleSpec string) (int, error) {
	// Format: s16le 2ch 44100Hz
	chRegex := regexp.MustCompile(`(\d+)ch`)
	matches := chRegex.FindStringSubmatch(sampleSpec)
	if len(matches) >= 2 {
		return strconv.Atoi(matches[1])
	}
	return 0, fmt.Errorf("no channel count found")
}

// enumerateWindowsDevices enumerates devices on Windows using PowerShell
func (d *DeviceManager) enumerateWindowsDevices() ([]AudioDevice, error) {
	devices := []AudioDevice{}

	// Use PowerShell to query audio devices
	cmd := exec.Command("powershell", "-Command",
		"Get-WmiObject Win32_SoundDevice | Where-Object {$_.ConfigManagerErrorCode -eq 0} | Select-Object -Property Name,DeviceID | ConvertTo-Json")
	output, err := cmd.Output()
	if err == nil && len(output) > 0 {
		devices = d.parseWindowsAudioDevices(string(output))
	}

	// If no devices found, return default
	if len(devices) == 0 {
		devices = append(devices, AudioDevice{
			ID:          "default",
			Name:        "Default Audio Device",
			IsDefault:   true,
			SampleRates: []int{8000, 16000, 44100, 48000},
			Channels:    1,
			IsAvailable: true,
			Driver:      "WASAPI",
		})
	}

	return devices, nil
}

// parseWindowsAudioDevices parses PowerShell JSON output for audio devices
func (d *DeviceManager) parseWindowsAudioDevices(output string) []AudioDevice {
	var devices []AudioDevice

	// Simple parsing - look for Name and DeviceID in the output
	lines := strings.Split(output, "\n")
	var currentName, currentID string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "\"Name\"") {
			// Extract name value
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				name := strings.TrimSpace(parts[1])
				name = strings.Trim(name, "\",")
				currentName = name
			}
		} else if strings.Contains(line, "\"DeviceID\"") {
			// Extract device ID value
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				id := strings.TrimSpace(parts[1])
				id = strings.Trim(id, "\",")
				currentID = id
			}
		}

		// When we have both name and ID, create a device
		if currentName != "" && currentID != "" {
			devices = append(devices, AudioDevice{
				ID:          currentID,
				Name:        currentName,
				IsDefault:   len(devices) == 0,
				SampleRates: []int{8000, 16000, 44100, 48000},
				Channels:    2,
				IsAvailable: true,
				Driver:      "WASAPI",
			})
			currentName = ""
			currentID = ""
		}
	}

	return devices
}

// enumerateMockDevices returns mock devices for testing
func (d *DeviceManager) enumerateMockDevices() ([]AudioDevice, error) {
	return []AudioDevice{
		{
			ID:          "mock-default",
			Name:        "Mock Default Microphone",
			IsDefault:   true,
			SampleRates: []int{8000, 16000, 44100, 48000},
			Channels:    1,
			IsAvailable: true,
			Driver:      "Mock",
		},
		{
			ID:          "mock-usb",
			Name:        "Mock USB Microphone",
			IsDefault:   false,
			SampleRates: []int{16000, 44100, 48000},
			Channels:    1,
			IsAvailable: true,
			Driver:      "Mock",
		},
	}, nil
}
