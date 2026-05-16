# Vision Package

The `vision` package provides automatic vision model switching for HelixCode, enabling seamless detection of image content in user input and automatic switching to vision-capable models when needed.

## Overview

When users include images in their conversations (screenshots, diagrams, photos), the system needs to automatically switch to a model that supports vision capabilities. This package handles the detection, model selection, and switching workflow transparently.

## Architecture

```
vision/
├── config.go       # Configuration types and defaults
├── detector.go     # Image detection logic
├── registry.go     # Model registry and capabilities
├── switcher.go     # Switch management and coordination
├── doc.go          # Package documentation
└── vision_test.go  # Comprehensive tests
```

### Core Components

- **ImageDetector**: Detects images in user input using various methods (MIME, extension, base64, URL, content inspection)
- **ModelRegistry**: Maintains registry of available models and their capabilities
- **CapabilityChecker**: Queries model capabilities and finds best vision models
- **VisionSwitchManager**: Coordinates detection, model selection, and switching
- **SwitchController**: Manages switch state, history, and reversion

## Key Types and Interfaces

### Image Detection

```go
type DetectionMethod string

const (
    DetectByMIME      DetectionMethod = "mime"      // Check MIME type
    DetectByExtension DetectionMethod = "extension" // Check file extension
    DetectByBase64    DetectionMethod = "base64"    // Detect base64 data URIs
    DetectByContent   DetectionMethod = "content"   // Magic number inspection
    DetectByURL       DetectionMethod = "url"       // Image URL patterns
)
```

### Image Sources

```go
type ImageSource string

const (
    SourceFile      ImageSource = "file"
    SourceURL       ImageSource = "url"
    SourceBase64    ImageSource = "base64"
    SourceClipboard ImageSource = "clipboard"
)
```

### Switch Modes

```go
type SwitchMode string

const (
    SwitchOnce    SwitchMode = "once"    // Single request only
    SwitchSession SwitchMode = "session" // Current session
    SwitchPersist SwitchMode = "persist" // Permanent change
)
```

### Model Definition

```go
type Model struct {
    ID           string
    Name         string
    Provider     string
    Capabilities *Capabilities
    Metadata     *ModelMetadata
}

type Capabilities struct {
    SupportsVision   bool
    SupportsAudio    bool
    SupportsVideo    bool
    MaxImageSize     int64
    MaxImages        int
    SupportedFormats []string
    ContextWindow    int
    OutputTokens     int
    FunctionCalling  bool
    StreamingSupport bool
}
```

## Usage Examples

### Basic Usage with Auto-Detection

```go
// Create registry with default vision models
registry := vision.NewModelRegistry()

// Configure auto-switch behavior
config := vision.DefaultConfig()
config.SwitchMode = vision.SwitchSession
config.RequireConfirm = true

// Create switch manager
manager, err := vision.NewVisionSwitchManager(config, registry)
if err != nil {
    log.Fatal(err)
}

// Set current model
manager.SetCurrentModel("gpt-3.5-turbo")

// Process user input with image
input := &vision.Input{
    Files: []*vision.File{
        {
            Path:     "screenshot.png",
            MIMEType: "image/png",
        },
    },
}

result, err := manager.ProcessInput(context.Background(), input)
if err != nil {
    log.Fatal(err)
}

if result.SwitchPerformed {
    fmt.Printf("Switched from %s to %s\n",
        result.FromModel.Name, result.ToModel.Name)
}
```

### Detecting Images in Text

```go
config := vision.DefaultDetectionConfig()
detector := vision.NewImageDetector(config)

// Detect base64 images
input := &vision.Input{
    Text: "Here's an image: data:image/png;base64,iVBORw0KGgo...",
}

result, err := detector.Detect(context.Background(), input)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Found %d images\n", result.ImageCount)
```

### Detecting Image URLs

```go
input := &vision.Input{
    Text: "Check this: https://example.com/photo.jpg",
}

result, err := detector.Detect(ctx, input)
// result.HasImages == true
// result.Images[0].Source == vision.SourceURL
```

### Content Inspection (Magic Numbers)

```go
inspector := vision.NewContentInspector()

data := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A} // PNG header
reader := bytes.NewReader(data)

result, err := inspector.InspectContent(reader)
if err == nil && result.IsImage {
    fmt.Printf("Detected format: %s\n", result.Format) // "png"
}
```

### Custom Model Registration

```go
registry := vision.NewModelRegistry()

customModel := &vision.Model{
    ID:       "custom-vision-model",
    Name:     "Custom Vision Model",
    Provider: "custom",
    Capabilities: &vision.Capabilities{
        SupportsVision:   true,
        MaxImageSize:     10 * 1024 * 1024,
        MaxImages:        5,
        SupportedFormats: []string{"jpg", "png"},
        ContextWindow:    32000,
    },
}

err := registry.Register(customModel)
if err != nil {
    log.Fatal(err)
}
```

### Finding Vision Models

```go
checker := vision.NewCapabilityChecker(registry)

// Find best model with preferences
preferences := &vision.ModelPreferences{
    PreferredModels: []string{"claude-3-5-sonnet-20241022", "gpt-4o"},
    Provider:        "anthropic",
}

model, err := checker.FindBestVisionModel(ctx, preferences)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Best vision model: %s\n", model.Name)
```

## Configuration Options

### Main Configuration

```go
type Config struct {
    // Detection settings
    EnableAutoDetect  bool              // Enable automatic image detection
    DetectionMethods  []DetectionMethod // Methods to use for detection
    ContentInspection bool              // Enable deep content inspection

    // Switch behavior
    SwitchMode     SwitchMode // once, session, or persist
    RequireConfirm bool       // Require user confirmation
    FallbackModel  string     // Model to use if vision unavailable
    AllowDowngrade bool       // Allow switching to lower-tier model

    // Model preferences
    PreferredVisionModel string   // Default vision model
    ModelPriority        []string // Priority order for model selection
    ProviderPreference   []string // Priority order for providers

    // Revert settings
    AutoRevert     bool          // Auto-revert to original model
    RevertDelay    time.Duration // Delay before auto-revert
    KeepForSession bool          // Keep switch for entire session
}
```

### Default Configuration

```go
config := vision.DefaultConfig()
// EnableAutoDetect:      true
// DetectionMethods:      [mime, extension, base64]
// SwitchMode:            session
// RequireConfirm:        true
// FallbackModel:         "claude-3-5-sonnet-20241022"
// PreferredVisionModel:  "claude-3-5-sonnet-20241022"
// ModelPriority:         ["claude-3-5-sonnet-20241022", "gpt-4o", "gemini-2.0-flash"]
// ProviderPreference:    ["anthropic", "openai", "google"]
// AutoRevert:            false
// RevertDelay:           5 minutes
// KeepForSession:        true
```

### Detection Configuration

```go
type DetectionConfig struct {
    Methods          []DetectionMethod // Detection methods to use
    SupportedFormats []string          // Supported image formats
    MaxFileSize      int64             // Maximum file size (default 10MB)
    InspectContent   bool              // Use magic number detection
    URLPatterns      []string          // URL patterns to match
}
```

## Default Vision-Capable Models

The package includes these models out-of-the-box:

| Model | Provider | Max Images | Max Size | Context |
|-------|----------|------------|----------|---------|
| claude-3-5-sonnet-20241022 | Anthropic | 20 | 10MB | 200K |
| gpt-4o | OpenAI | 10 | 20MB | 128K |
| gpt-4-turbo | OpenAI | 10 | 20MB | 128K |
| gpt-4-vision-preview | OpenAI | 10 | 20MB | 128K |
| gemini-2.0-flash | Google | 16 | 4MB | 1M |
| gemini-pro-vision | Google | 16 | 4MB | 32K |
| qwen-vl-plus | Qwen | 8 | 10MB | 32K |
| qwen-vl-max | Qwen | 8 | 10MB | 32K |

## Switch Modes Explained

### Once Mode

```go
config.SwitchMode = vision.SwitchOnce
```

- Switches for a single request only
- Automatically reverts after request completes
- No persistent state
- Best for: One-off image analysis

### Session Mode

```go
config.SwitchMode = vision.SwitchSession
```

- Switches for the current session
- All subsequent requests use the vision model
- Reverts when session ends
- Best for: Image-heavy conversations

### Persist Mode

```go
config.SwitchMode = vision.SwitchPersist
```

- Permanently switches to vision model
- Saves preference to configuration
- All future sessions start with vision model
- Best for: Users who regularly work with images

## Image Format Detection

### By Magic Numbers

```go
var ImageSignatures = map[string][]byte{
    "png":  {0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A},
    "jpg":  {0xFF, 0xD8, 0xFF},
    "gif":  {0x47, 0x49, 0x46, 0x38},
    "webp": {0x52, 0x49, 0x46, 0x46},
    "bmp":  {0x42, 0x4D},
    "tiff": {0x49, 0x49, 0x2A, 0x00},
}
```

### Supported Formats

Default supported formats: `jpg`, `jpeg`, `png`, `gif`, `webp`, `bmp`

## Switch History Tracking

```go
// Get switch history
history := manager.GetSwitchHistory()
for _, event := range history {
    fmt.Printf("%s: %s -> %s (reason: %s)\n",
        event.Timestamp,
        event.FromModel.Name,
        event.ToModel.Name,
        event.Reason)
}

// Check if switch is active
if manager.IsSwitchActive() {
    // Currently using a vision model
}

// Manually revert to original model
err := manager.RevertSwitch(ctx)
```

## Error Handling

Common errors:

```go
// Detection errors
ErrNoImagesDetected  // No images found in input
ErrInvalidFormat     // Unsupported image format

// Model errors
ErrModelNotFound     // Requested model not in registry
ErrNoVisionSupport   // Model does not support vision
ErrNoVisionModels    // No vision-capable models available

// Switch errors
ErrSwitchFailed      // Model switch operation failed
ErrSwitchDenied      // User denied switch confirmation
```

## Thread Safety

All components are thread-safe:

- `ModelRegistry` uses `sync.RWMutex` for concurrent access
- `VisionSwitchManager` protects state with mutexes
- `SwitchController` safely tracks switch history
- `SwitchHistory` has its own mutex for event access

## Performance

Detection is optimized for speed:

- MIME and extension checks: < 1ms
- Base64 detection: Uses compiled regex
- Content inspection: Reads only first 16 bytes
- Results returned immediately without buffering

## Validation

Configuration validation ensures correctness:

```go
err := config.Validate()
if err != nil {
    // Handle validation error:
    // - Invalid switch mode
    // - No detection methods
    // - No fallback model
    // - Negative revert delay
}
```

## Testing

```bash
cd HelixCode
go test -v ./internal/llm/vision

# Run specific tests
go test -v ./internal/llm/vision -run TestImageDetectionByMIME
go test -v ./internal/llm/vision -run TestAutoSwitch
go test -v ./internal/llm/vision -run TestSwitchModes

# Benchmarks
go test -bench=. ./internal/llm/vision
```

## Best Practices

1. **Use session mode** for most applications - balances convenience and control
2. **Enable confirmation** for critical workflows to prevent unexpected switches
3. **Set appropriate model priority** based on your use case (cost vs. capability)
4. **Handle switch events** to inform users when model changes occur
5. **Configure fallback model** to ensure graceful degradation
6. **Monitor switch history** to understand usage patterns
7. **Test with various image types** to ensure detection works correctly
