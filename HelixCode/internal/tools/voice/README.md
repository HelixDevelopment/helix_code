# Voice Package

The `voice` package provides voice-to-code functionality for HelixCode, enabling users to provide input through speech. It integrates audio capture, device management, and speech-to-text transcription using OpenAI's Whisper API.

## Overview

This package enables:
- Audio recording from microphone with configurable sample rates and formats
- Device selection and enumeration across platforms (macOS, Linux, Windows)
- Whisper transcription integration with OpenAI API
- Language support with auto-detection or explicit specification
- Volume level detection and real-time monitoring
- WAV/MP3 format support
- Silence detection for automatic recording termination
- Mock audio support for testing without hardware

## Key Types

### VoiceInputManager

The main orchestrator for voice input operations.

```go
type VoiceInputManager struct {
    recorder    *AudioRecorder
    devices     *DeviceManager
    transcriber *Transcriber
    config      *VoiceConfig
}
```

### VoiceConfig

Configuration for voice input operations.

```go
type VoiceConfig struct {
    // Audio settings
    SampleRate int         // Default: 16000 Hz (Whisper requirement)
    Channels   int         // Default: 1 (mono)
    BitDepth   int         // Default: 16
    Format     AudioFormat // WAV or MP3

    // Recording settings
    MaxDuration      time.Duration // Default: 5 minutes
    SilenceTimeout   time.Duration // Default: 2 seconds
    SilenceThreshold float64       // Default: -40.0 dB

    // Transcription settings
    WhisperModel string  // Default: "whisper-1"
    Language     string  // Optional, auto-detect if empty
    Prompt       string  // Optional context for transcription
    Temperature  float64 // Default: 0.0 (deterministic)

    // Device settings
    DefaultDevice string // Device ID or name
    AutoSelect    bool   // Auto-select default device

    // Output settings
    OutputDirectory string // Default: ~/.helix/voice/recordings
    APIKey          string // OpenAI API key (required for transcription)
    BaseURL         string // Optional custom Whisper API endpoint
}
```

### AudioRecorder

Handles microphone input and recording.

```go
type AudioRecorder struct {
    device          *AudioDevice
    config          *AudioConfig
    recording       bool
    currentFile     string
    levelMonitor    *LevelMonitor
    silenceDetector *SilenceDetector
    samples         []int16
    mu              sync.RWMutex
    mockMode        bool
}

type AudioConfig struct {
    SampleRate      int
    Channels        int
    BitDepth        int
    Format          AudioFormat
    OutputDirectory string
}
```

### AudioDevice

Represents an audio input device.

```go
type AudioDevice struct {
    ID          string
    Name        string
    Driver      string   // CoreAudio, ALSA, WASAPI, Mock
    IsDefault   bool
    IsAvailable bool
    SampleRates []int
    Channels    int
    BitDepths   []int
}
```

### AudioLevels

Real-time audio level information.

```go
type AudioLevels struct {
    Peak      float64   // Peak level in dB
    RMS       float64   // RMS level in dB
    IsSilent  bool      // Whether current audio is silent
    Timestamp time.Time // Timestamp of measurement
}
```

### Transcriber

Handles Whisper API transcription.

```go
type Transcriber struct {
    config     *TranscriptionConfig
    httpClient *http.Client
    mu         sync.RWMutex
}

type TranscriptionConfig struct {
    APIKey      string
    Model       string  // whisper-1
    Language    string  // ISO 639-1 code
    Prompt      string  // Context prompt
    Temperature float64
    BaseURL     string
    Timeout     time.Duration
}

type TranscriptionResult struct {
    Text       string
    Language   string
    Duration   time.Duration
    Confidence float64
    Segments   []TranscriptionSegment
}

type TranscriptionSegment struct {
    ID         int
    Start      float64
    End        float64
    Text       string
    Confidence float64
}
```

### LevelMonitor

Tracks real-time audio levels.

```go
type LevelMonitor struct {
    buffer      []float64
    bufferSize  int
    windowSize  time.Duration
    updateRate  time.Duration
    currentPeak float64
    currentRMS  float64
    mu          sync.RWMutex
}
```

### SilenceDetector

Identifies periods of silence.

```go
type SilenceDetector struct {
    threshold    float64
    minDuration  time.Duration
    silenceStart time.Time
    isSilent     bool
    mu           sync.RWMutex
}
```

## Usage Examples

### Basic Voice Input

```go
package main

import (
    "context"
    "fmt"
    "os"
    "time"

    "dev.helix.code/internal/tools/voice"
)

func main() {
    config := &voice.VoiceConfig{
        SampleRate:      16000,
        Channels:        1,
        Format:          voice.FormatWAV,
        SilenceTimeout:  2 * time.Second,
        APIKey:          os.Getenv("OPENAI_API_KEY"),
        AutoSelect:      true,
    }

    manager, err := voice.NewVoiceInputManager(config)
    if err != nil {
        panic(err)
    }

    ctx := context.Background()

    // Start recording
    if err := manager.StartRecording(ctx); err != nil {
        panic(err)
    }

    fmt.Println("Recording... Speak now!")

    // Wait for user to speak
    time.Sleep(5 * time.Second)

    // Stop and get the audio file
    audioPath, err := manager.StopRecording(ctx)
    if err != nil {
        panic(err)
    }

    fmt.Printf("Audio saved to: %s\n", audioPath)

    // Transcribe the recording
    text, err := manager.TranscribeRecording(ctx, audioPath)
    if err != nil {
        panic(err)
    }

    fmt.Printf("Transcribed: %s\n", text)
}
```

### One-Shot Recording and Transcription

```go
// Record and transcribe in a single operation
text, err := manager.RecordAndTranscribe(ctx)
if err != nil {
    panic(err)
}
fmt.Printf("You said: %s\n", text)

// This method automatically stops recording after detecting
// silence for the configured duration (default: 2 seconds)
```

### Device Management

```go
// List all available devices
devices, err := manager.ListDevices(ctx)
if err != nil {
    panic(err)
}

for _, device := range devices {
    fmt.Printf("Device: %s (%s)\n", device.Name, device.ID)
    fmt.Printf("  Driver: %s\n", device.Driver)
    fmt.Printf("  Default: %v, Available: %v\n", device.IsDefault, device.IsAvailable)
    fmt.Printf("  Sample rates: %v\n", device.SampleRates)
    fmt.Printf("  Channels: %d\n", device.Channels)
}

// Select a specific device
if err := manager.SelectDevice(ctx, "device-id"); err != nil {
    panic(err)
}

// Get currently active device
activeDevice := manager.GetActiveDevice()
fmt.Printf("Active device: %s\n", activeDevice.Name)
```

### Audio Level Monitoring

```go
// Start recording
if err := manager.StartRecording(ctx); err != nil {
    panic(err)
}

// Monitor audio levels in real-time
ticker := time.NewTicker(100 * time.Millisecond)
defer ticker.Stop()

timeout := time.After(10 * time.Second)

for {
    select {
    case <-timeout:
        manager.StopRecording(ctx)
        return
    case <-ticker.C:
        levels := manager.GetAudioLevels()
        fmt.Printf("\rPeak: %6.1f dB | RMS: %6.1f dB | Silent: %v",
            levels.Peak, levels.RMS, levels.IsSilent)
    }
}
```

### Custom Transcription Options

```go
// Create transcriber with custom options
transcriber, err := voice.NewTranscriber(&voice.TranscriptionConfig{
    APIKey:      os.Getenv("OPENAI_API_KEY"),
    Model:       "whisper-1",
    Language:    "en",                    // Force English
    Prompt:      "Programming context",   // Guide transcription
    Temperature: 0.0,                     // Deterministic output
    Timeout:     30 * time.Second,
})

// Transcribe file directly
result, err := transcriber.TranscribeFile(ctx, "/path/to/audio.wav")

fmt.Printf("Text: %s\n", result.Text)
fmt.Printf("Language: %s\n", result.Language)
fmt.Printf("Duration: %v\n", result.Duration)
fmt.Printf("Confidence: %.2f\n", result.Confidence)

// Access segments for detailed timing
for _, seg := range result.Segments {
    fmt.Printf("[%.2f - %.2f] %s\n", seg.Start, seg.End, seg.Text)
}
```

### Silence Detection

```go
// Configure silence detection
config := &voice.VoiceConfig{
    SilenceThreshold: -35.0,         // More sensitive (default: -40.0 dB)
    SilenceTimeout:   3 * time.Second, // Longer silence required
}

manager, _ := voice.NewVoiceInputManager(config)

// Start recording with auto-stop on silence
text, err := manager.RecordAndTranscribe(ctx)
// Recording automatically stops after 3 seconds of silence
```

### Mock Audio for Testing

```go
// Create a mock transcriber for testing
mockTranscriber := voice.NewMockTranscriber(&voice.TranscriptionConfig{
    APIKey: "test-key",
    Model:  "whisper-1",
})

// Set custom responses for specific files
mockTranscriber.SetMockResponse("/path/to/test.wav", "This is a test transcription")

// Set default response for any file
mockTranscriber.SetDefaultResponse("Default transcription response")

// Use the mock transcriber
result, err := mockTranscriber.TranscribeFile(ctx, "/path/to/test.wav")
fmt.Printf("Mock result: %s\n", result.Text)

// Mock device manager is created automatically when no hardware is available
// This allows tests to run in CI/CD environments
```

### Recording with Specific Format

```go
// Record as WAV (default)
configWAV := &voice.VoiceConfig{
    Format:     voice.FormatWAV,
    SampleRate: 16000,
    BitDepth:   16,
    Channels:   1,
}

// Record as MP3 (requires encoder)
configMP3 := &voice.VoiceConfig{
    Format:     voice.FormatMP3,
    SampleRate: 44100,
    BitDepth:   16,
    Channels:   2,
}
```

### Custom Output Directory

```go
config := &voice.VoiceConfig{
    OutputDirectory: "/custom/path/to/recordings",
    // Files will be named: recording_20060102_150405.wav
}

manager, _ := voice.NewVoiceInputManager(config)

// Recording will be saved to the custom directory
audioPath, _ := manager.StopRecording(ctx)
// e.g., /custom/path/to/recordings/recording_20240115_143022.wav
```

## Configuration Options

### VoiceConfig

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `SampleRate` | int | 16000 | Audio sample rate in Hz |
| `Channels` | int | 1 | Number of audio channels |
| `BitDepth` | int | 16 | Bits per sample |
| `Format` | AudioFormat | WAV | Output format (WAV/MP3) |
| `MaxDuration` | time.Duration | 5m | Maximum recording duration |
| `SilenceTimeout` | time.Duration | 2s | Silence duration to auto-stop |
| `SilenceThreshold` | float64 | -40.0 | Silence threshold in dB |
| `WhisperModel` | string | whisper-1 | Whisper model name |
| `Language` | string | "" | ISO 639-1 language code |
| `Prompt` | string | "" | Context prompt for transcription |
| `Temperature` | float64 | 0.0 | Transcription temperature |
| `DefaultDevice` | string | "" | Preferred device ID/name |
| `AutoSelect` | bool | true | Auto-select default device |
| `OutputDirectory` | string | ~/.helix/voice/recordings | Recording output path |
| `APIKey` | string | "" | OpenAI API key |
| `BaseURL` | string | "" | Custom API endpoint |

## Platform Support

The package supports audio capture on multiple platforms:

| Platform | Driver | Description |
|----------|--------|-------------|
| macOS | CoreAudio | Native CoreAudio API |
| Linux | ALSA/PulseAudio | ALSA with PulseAudio support |
| Windows | WASAPI | Windows Audio Session API |
| Other | Mock | Mock driver for testing |

Platform-specific audio APIs are used when available, with automatic fallback to mock implementations for unsupported platforms or testing.

## Security Considerations

1. **API Key Protection**: Store the OpenAI API key securely. Never commit it to version control.

2. **Audio Privacy**: Recordings may contain sensitive information. Ensure proper access controls on the output directory.

3. **Network Security**: Transcription requests are sent over HTTPS to OpenAI's servers.

4. **File Cleanup**: Implement a cleanup policy for audio recordings to prevent accumulation of sensitive data.

5. **File Size Limits**: Whisper API rejects files larger than 25MB. The package validates file sizes before upload.

## Error Types

```go
var (
    ErrNoDevicesFound      = errors.New("no audio input devices available")
    ErrDeviceNotFound      = errors.New("specified device does not exist")
    ErrAlreadyRecording    = errors.New("recording already in progress")
    ErrNotRecording        = errors.New("no recording to stop")
    ErrTranscriptionFailed = errors.New("Whisper API transcription failed")
    ErrInvalidAPIKey       = errors.New("OpenAI API key is invalid or missing")
    ErrFileTooLarge        = errors.New("audio file exceeds 25MB Whisper API limit")
)
```

All errors can be checked using `errors.Is()`:

```go
if errors.Is(err, voice.ErrAlreadyRecording) {
    // Handle duplicate recording attempt
}
```

## Performance Considerations

- Audio is buffered in memory during recording
- Level monitoring runs in a separate goroutine with configurable update rate
- Whisper API calls have a 30-second timeout by default
- Audio files larger than 25MB are rejected before upload
- Mock audio generates sine waves at 440Hz (A4 note) for testing
- Sample rate of 16000 Hz is optimal for Whisper API

## Best Practices

1. **Use 16kHz sample rate** for optimal Whisper transcription quality and file size.

2. **Enable silence detection** to automatically stop recording when the user stops speaking.

3. **Set appropriate timeouts** to prevent indefinitely long recordings.

4. **Provide context prompts** when transcribing domain-specific content (e.g., "Programming terms: function, variable, class").

5. **Use language hints** if you know the expected language to improve accuracy.

6. **Clean up recordings** periodically to manage disk space.

7. **Handle device unavailability** gracefully with fallback options.
