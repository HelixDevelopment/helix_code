# Hardware Package

The `hardware` package provides hardware detection and capability profiling for the HelixCode platform. It detects system hardware capabilities including CPU, GPU, memory, and platform information to determine optimal configurations for running LLM models and to provide hardware-aware compilation flags for llama.cpp integration.

## Overview

This package handles:
- Comprehensive hardware detection (CPU, GPU, memory, platform)
- Optimal LLM model size recommendations based on available resources
- Hardware compatibility checking for specific model sizes
- Platform-specific compilation flag generation for llama.cpp
- Support for NVIDIA CUDA and Apple Metal acceleration

## Architecture

The hardware system provides two main detection approaches:

1. **Detector** - Comprehensive hardware detection with platform-specific methods
2. **HardwareDetector** - Simplified profile generation using Go runtime

Both approaches provide similar information but with different levels of detail and platform-specific optimizations.

## Key Types

### HardwareProfile

Simplified view of system capabilities:

```go
type HardwareProfile struct {
    CPU     CPUInfo     `json:"cpu"`
    GPU     *GPUInfo    `json:"gpu,omitempty"`
    Memory  MemoryInfo  `json:"memory"`
    OS      OSInfo      `json:"os"`
    Network NetworkInfo `json:"network"`
}
```

### HardwareInfo

Comprehensive hardware information structure:

```go
type HardwareInfo struct {
    CPU      CPUInfo      `json:"cpu"`
    GPU      GPUInfo      `json:"gpu"`
    Memory   MemoryInfo   `json:"memory"`
    Platform PlatformInfo `json:"platform"`
}
```

### CPUInfo

Detailed CPU specifications:

```go
type CPUInfo struct {
    Architecture string `json:"architecture"`
    Vendor       string `json:"vendor"`
    Model        string `json:"model"`
    Cores        int    `json:"cores"`
    Threads      int    `json:"threads"`
    HasAVX       bool   `json:"has_avx"`
    HasAVX2      bool   `json:"has_avx2"`
    HasNEON      bool   `json:"has_neon"`
    Arch         string `json:"arch"`
    Frequency    int    `json:"frequency_mhz"`
    CacheSize    int    `json:"cache_size_kb"`
}
```

### GPUInfo

GPU specifications:

```go
type GPUInfo struct {
    Name              string  `json:"name"`
    Type              GPUType `json:"type"`
    Vendor            string  `json:"vendor"`
    Model             string  `json:"model"`
    VRAM              string  `json:"vram"`
    ComputeCapability float64 `json:"compute_capability"`
    CUDAVersion       string  `json:"cuda_version,omitempty"`
    SupportsCUDA      bool    `json:"supports_cuda"`
    SupportsMetal     bool    `json:"supports_metal"`
}
```

### MemoryInfo

Memory specifications:

```go
type MemoryInfo struct {
    Total     int64  `json:"total_bytes"`
    Available int64  `json:"available_bytes"`
    SwapTotal int64  `json:"swap_total_bytes"`
    TotalRAM  string `json:"total_ram"`
}
```

### PlatformInfo

Platform-specific information:

```go
type PlatformInfo struct {
    OS           string `json:"os"`
    Architecture string `json:"architecture"`
    Hostname     string `json:"hostname"`
}
```

### OSInfo

Operating system information:

```go
type OSInfo struct {
    Name    string `json:"name"`
    Version string `json:"version"`
    Arch    string `json:"arch"`
}
```

### NetworkInfo

Network connectivity information:

```go
type NetworkInfo struct {
    HasInternet bool          `json:"has_internet"`
    Latency     time.Duration `json:"latency"`
    Bandwidth   int64         `json:"bandwidth_bps"`
}
```

## Type Constants

### GPUType

Supported GPU types:

```go
const (
    GPUTypeNVIDIA GPUType = "nvidia"
    GPUTypeAMD    GPUType = "amd"
    GPUTypeApple  GPUType = "apple"
    GPUTypeIntel  GPUType = "intel"
)
```

### OSType

Supported operating systems:

```go
const (
    OSTypeLinux   OSType = "linux"
    OSTypeMacOS   OSType = "macos"
    OSTypeWindows OSType = "windows"
)
```

### Arch

Supported architectures:

```go
const (
    ArchX86_64 Arch = "x86_64"
    ArchARM64  Arch = "arm64"
    ArchARM32  Arch = "arm32"
)
```

## Usage Examples

### Basic Hardware Detection

```go
import "dev.helix.code/internal/hardware"

detector := hardware.NewDetector()
info, err := detector.Detect()
if err != nil {
    log.Printf("Detection error: %v", err)
}

log.Printf("CPU: %s (%d cores)", info.CPU.Model, info.CPU.Cores)
log.Printf("GPU: %s (%s VRAM)", info.GPU.Model, info.GPU.VRAM)
log.Printf("Platform: %s/%s", info.Platform.OS, info.Platform.Architecture)
```

### Using HardwareDetector for Profiles

```go
detector := hardware.NewHardwareDetector()
profile := detector.GetProfile()

log.Printf("CPU Cores: %d", profile.CPU.Cores)
log.Printf("CPU Threads: %d", profile.CPU.Threads)
log.Printf("Memory: %d bytes", profile.Memory.Total)
log.Printf("OS: %s (%s)", profile.OS.Name, profile.OS.Arch)
```

### Using Default Profile

```go
profile := hardware.DefaultProfile()

log.Printf("System: %s on %s", profile.OS.Name, profile.OS.Arch)
log.Printf("CPU: %d cores", profile.CPU.Cores)
if profile.GPU != nil {
    log.Printf("GPU: %s", profile.GPU.Name)
}
```

### Determining Optimal Model Size

```go
detector := hardware.NewDetector()
detector.Detect()

modelSize := detector.GetOptimalModelSize()
log.Printf("Recommended model size: %s", modelSize)
// Returns: "3B", "7B", "13B", "34B", or "70B"
```

### Checking Model Compatibility

```go
detector := hardware.NewDetector()
detector.Detect()

if detector.CanRunModel("13B") {
    log.Println("System can run 13B parameter models")
} else {
    optimal := detector.GetOptimalModelSize()
    log.Printf("System limited to %s models", optimal)
}
```

### Getting Compilation Flags

```go
detector := hardware.NewDetector()
detector.Detect()

flags := detector.GetCompilationFlags()
// May return: ["-DGGML_USE_CUBLAS"] for NVIDIA
// Or: ["-DGGML_USE_METAL"] for Apple Silicon

log.Printf("Compilation flags: %v", flags)
```

## Model Size Recommendations

The package recommends LLM model sizes based on total available memory (VRAM + RAM/2):

| Total Memory | Recommended Size |
|-------------|------------------|
| 32GB+       | 70B parameters   |
| 16GB+       | 34B parameters   |
| 8GB+        | 13B parameters   |
| 4GB+        | 7B parameters    |
| <4GB        | 3B parameters    |

## Platform-Specific Detection

### Linux Detection

On Linux systems, the package uses:

- **CPU Detection**: Parses `/proc/cpuinfo` for model name, vendor, and features
- **GPU Detection**: Uses `nvidia-smi` for NVIDIA GPUs
- **Memory Detection**: Standard methods (defaults to 16GB if unavailable)

```go
// Linux CPU detection reads /proc/cpuinfo
// Lines parsed:
// - "model name" -> CPU model
// - "vendor_id" -> CPU vendor
```

### macOS Detection

On macOS systems, the package uses:

- **CPU Detection**: `sysctl -n machdep.cpu.brand_string` and `machdep.cpu.vendor`
- **GPU Detection**: `system_profiler SPDisplaysDataType` for GPU model and VRAM
- **Metal Support**: Automatically enabled for Apple Silicon

```go
// macOS GPU detection
cmd := exec.Command("system_profiler", "SPDisplaysDataType")
// Parses "Chipset Model:" and "VRAM" lines
```

### Windows Detection

On Windows systems, the package uses:

- Generic detection using Go runtime
- Falls back to default/unknown values for detailed information

## GPU Acceleration Support

### NVIDIA CUDA

Detected when `nvidia-smi` is available:

```go
if info.GPU.SupportsCUDA {
    // CUDA acceleration available
    // Compilation flag: -DGGML_USE_CUBLAS
}
```

### Apple Metal

Automatically detected on macOS:

```go
if info.GPU.SupportsMetal {
    // Metal acceleration available
    // Compilation flag: -DGGML_USE_METAL
}
```

## Configuration

### YAML Configuration

```yaml
hardware:
  enabled: true
  auto_detect: true
  serial:
    default_baud: 9600
  gpio:
    platform: "raspberry_pi"
```

## Error Handling

Detection methods log warnings but do not fail the overall detection. Individual detection failures result in default or unknown values:

```go
info, err := detector.Detect()
// err is typically nil even if some detection fails

if info.CPU.Model == "Unknown" {
    log.Println("CPU model detection failed")
}

if info.GPU.Model == "Unknown" {
    log.Println("GPU not detected or detection failed")
}
```

## Best Practices

### 1. Handle Detection Gracefully

Always check for unknown values after detection:

```go
info, _ := detector.Detect()
if info.CPU.Model == "Unknown" {
    // Fall back to default behavior
}
```

### 2. Cache Detection Results

Hardware doesn't change during runtime, so cache results:

```go
var cachedInfo *hardware.HardwareInfo
var once sync.Once

func getHardwareInfo() *hardware.HardwareInfo {
    once.Do(func() {
        detector := hardware.NewDetector()
        cachedInfo, _ = detector.Detect()
    })
    return cachedInfo
}
```

### 3. Use Model Size Recommendations

Let the package determine appropriate model sizes:

```go
modelSize := detector.GetOptimalModelSize()
// Use modelSize for model selection rather than hardcoding
```

### 4. Check Compatibility Before Loading Models

Always verify hardware compatibility:

```go
if !detector.CanRunModel(requestedSize) {
    return fmt.Errorf("insufficient hardware for %s model", requestedSize)
}
```

## Integration Patterns

### With LLM Provider Selection

```go
func selectProvider(detector *hardware.Detector) llm.Provider {
    info, _ := detector.Detect()

    if info.GPU.SupportsCUDA || info.GPU.SupportsMetal {
        // Use GPU-accelerated provider
        return llm.NewOllamaProvider(config)
    }

    // Fall back to API-based provider
    return llm.NewOpenAIProvider(config)
}
```

### With Model Download

```go
func downloadOptimalModel(detector *hardware.Detector) error {
    modelSize := detector.GetOptimalModelSize()

    modelMap := map[string]string{
        "3B":  "phi-2",
        "7B":  "llama-2-7b",
        "13B": "llama-2-13b",
        "34B": "codellama-34b",
        "70B": "llama-2-70b",
    }

    return downloadModel(modelMap[modelSize])
}
```

### With Build Configuration

```go
func configureBuild(detector *hardware.Detector) *BuildConfig {
    flags := detector.GetCompilationFlags()

    config := &BuildConfig{
        Flags: flags,
    }

    if contains(flags, "-DGGML_USE_CUBLAS") {
        config.GPUBackend = "cuda"
    } else if contains(flags, "-DGGML_USE_METAL") {
        config.GPUBackend = "metal"
    }

    return config
}
```

## Supported Hardware

### CPUs

- Intel x86_64 processors
- AMD x86_64 processors
- Apple Silicon (M1, M2, M3 series)
- ARM64 processors

### GPUs

- NVIDIA GPUs with CUDA support
- Apple Silicon integrated GPUs (Metal)
- AMD GPUs (detection only)
- Intel integrated GPUs (detection only)

## Thread Safety

The `Detector` is **not** thread-safe. Create separate instances for concurrent use or synchronize access externally:

```go
var mu sync.Mutex
var detector *hardware.Detector

func detectHardware() (*hardware.HardwareInfo, error) {
    mu.Lock()
    defer mu.Unlock()

    if detector == nil {
        detector = hardware.NewDetector()
    }
    return detector.Detect()
}
```

## Testing

```bash
# Run all hardware tests
go test -v ./internal/hardware/...

# Run with coverage
go test -cover ./internal/hardware/...

# Run specific test
go test -v ./internal/hardware -run TestHardwareDetector
```

## Notes

- Detection uses platform-specific commands that may require appropriate permissions
- NVIDIA detection requires `nvidia-smi` to be available in PATH
- macOS detection uses `sysctl` and `system_profiler` commands
- Memory detection defaults to 16GB if unable to detect actual values
- CPU core count uses `runtime.NumCPU()` as a reliable baseline
- GPU detection may return empty/unknown values if no GPU is available
- Metal support is assumed true for all macOS systems
