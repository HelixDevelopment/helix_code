package hardware

import (
	"bufio"
	stdctx "context"
	"fmt"
	"log"
	"math"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

// HardwareInfo contains comprehensive hardware information
type HardwareInfo struct {
	CPU      CPUInfo      `json:"cpu"`
	GPU      GPUInfo      `json:"gpu"`
	Memory   MemoryInfo   `json:"memory"`
	Platform PlatformInfo `json:"platform"`
}

// PlatformInfo contains platform-specific information
type PlatformInfo struct {
	OS           string `json:"os"`
	Architecture string `json:"architecture"`
	Hostname     string `json:"hostname"`
}

// Detector handles hardware detection
type Detector struct {
	info *HardwareInfo
}

// NewDetector creates a new hardware detector
func NewDetector() *Detector {
	return &Detector{
		info: &HardwareInfo{},
	}
}

// Detect performs comprehensive hardware detection
func (d *Detector) Detect() (*HardwareInfo, error) {
	ctx := stdctx.Background()
	log.Println(tr(ctx, "internal_hardware_detection_starting", nil))

	// Detect CPU information
	if err := d.detectCPU(); err != nil {
		log.Print(tr(ctx, "internal_hardware_cpu_detection_failed", map[string]any{"Error": err}))
	}

	// Detect GPU information
	if err := d.detectGPU(); err != nil {
		log.Print(tr(ctx, "internal_hardware_gpu_detection_failed", map[string]any{"Error": err}))
	}

	// Detect memory information
	if err := d.detectMemory(); err != nil {
		log.Print(tr(ctx, "internal_hardware_memory_detection_failed", map[string]any{"Error": err}))
	}

	// Detect platform information
	if err := d.detectPlatform(); err != nil {
		log.Print(tr(ctx, "internal_hardware_platform_detection_failed", map[string]any{"Error": err}))
	}

	log.Println(tr(ctx, "internal_hardware_detection_completed", nil))
	return d.info, nil
}

// GetOptimalModelSize calculates the optimal model size for the hardware
func (d *Detector) GetOptimalModelSize() string {
	// Calculate based on available VRAM and RAM
	vramGB := d.parseMemorySize(d.info.GPU.VRAM)
	ramGB := d.parseMemorySize(d.info.Memory.TotalRAM)

	// Determine optimal model size based on available memory
	totalMemory := vramGB + (ramGB / 2) // Use half of RAM for model loading

	switch {
	case totalMemory >= 32:
		return "70B" // 70B parameter models
	case totalMemory >= 16:
		return "34B" // 34B parameter models
	case totalMemory >= 8:
		return "13B" // 13B parameter models
	case totalMemory >= 4:
		return "7B" // 7B parameter models
	default:
		return "3B" // 3B parameter models
	}
}

// parseMemorySize parses a memory size string like "16GB" or "8192MB" and returns GB
func (d *Detector) parseMemorySize(sizeStr string) int {
	if sizeStr == "" {
		return 0
	}

	// Anchor the pattern to the ENTIRE (trimmed) string. An un-anchored pattern
	// matched a digit substring anywhere, so a leading '-' ("-5GB") or
	// scientific notation ("1e308TB") was silently dropped/truncated and a
	// negative/malformed size became positive memory. Anchoring forces such
	// inputs to fail the match → 0.
	re := regexp.MustCompile(`^(\d+(?:\.\d+)?)\s*(GB|MB|TB|G|M|T)?$`)
	matches := re.FindStringSubmatch(strings.TrimSpace(strings.ToUpper(sizeStr)))
	if len(matches) < 2 {
		log.Print(tr(stdctx.Background(), "internal_hardware_parse_memory_size_failed", map[string]any{"SizeStr": sizeStr}))
		return 0
	}

	value, err := strconv.ParseFloat(matches[1], 64)
	if err != nil {
		log.Print(tr(stdctx.Background(), "internal_hardware_parse_memory_value_failed", map[string]any{"Value": matches[1], "Error": err}))
		return 0
	}

	// Defence in depth: reject negative or non-finite values so a malformed
	// size can never become positive memory.
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
		log.Print(tr(stdctx.Background(), "internal_hardware_parse_memory_value_failed", map[string]any{"Value": matches[1], "Error": "negative or non-finite size"}))
		return 0
	}

	// Determine unit and convert to GB
	unit := "GB" // Default to GB if no unit specified
	if len(matches) >= 3 && matches[2] != "" {
		unit = matches[2]
	}

	switch unit {
	case "TB", "T":
		return int(value * 1024)
	case "GB", "G":
		return int(value)
	case "MB", "M":
		return int(value / 1024)
	default:
		return int(value) // Assume GB
	}
}

// CanRunModel checks if the hardware can run a specific model size
func (d *Detector) CanRunModel(modelSize string) bool {
	optimalSize := d.GetOptimalModelSize()

	// Model size comparison (larger numbers are better)
	sizeOrder := map[string]int{
		"3B":  1,
		"7B":  2,
		"13B": 3,
		"34B": 4,
		"70B": 5,
	}

	requestedOrder, ok := sizeOrder[modelSize]
	if !ok {
		// Unknown / typo'd / unsupported model size: a map miss yields order 0,
		// which would otherwise be <= optimalOrder and wrongly reported as
		// runnable. An unrecognised size is NOT runnable.
		return false
	}
	optimalOrder := sizeOrder[optimalSize]

	return requestedOrder <= optimalOrder
}

// GetCompilationFlags returns hardware-specific compilation flags
func (d *Detector) GetCompilationFlags() []string {
	flags := make([]string, 0)

	// GPU acceleration flags
	if d.info.GPU.SupportsCUDA {
		flags = append(flags, "-DGGML_USE_CUBLAS")
	}
	if d.info.GPU.SupportsMetal {
		flags = append(flags, "-DGGML_USE_METAL")
	}

	return flags
}

// Private detection methods

func (d *Detector) detectCPU() error {
	d.info.CPU = CPUInfo{
		Architecture: runtime.GOARCH,
		Cores:        runtime.NumCPU(),
		Threads:      runtime.NumCPU(), // Simplified - same as cores
	}

	// Try to get more detailed CPU info
	switch runtime.GOOS {
	case "linux":
		return d.detectCPULinux()
	case "darwin":
		return d.detectCPUMacOS()
	default:
		return d.detectCPUGeneric()
	}
}

func (d *Detector) detectCPULinux() error {
	// Read /proc/cpuinfo for detailed CPU information
	data, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return err
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "model name") {
			parts := strings.Split(line, ":")
			if len(parts) > 1 {
				d.info.CPU.Model = strings.TrimSpace(parts[1])
			}
		} else if strings.HasPrefix(line, "vendor_id") {
			parts := strings.Split(line, ":")
			if len(parts) > 1 {
				d.info.CPU.Vendor = strings.TrimSpace(parts[1])
			}
		}
	}

	return nil
}

func (d *Detector) detectCPUMacOS() error {
	// Use sysctl to get CPU information on macOS
	cmd := exec.Command("sysctl", "-n", "machdep.cpu.brand_string")
	if output, err := cmd.Output(); err == nil {
		d.info.CPU.Model = strings.TrimSpace(string(output))
	}

	cmd = exec.Command("sysctl", "-n", "machdep.cpu.vendor")
	if output, err := cmd.Output(); err == nil {
		d.info.CPU.Vendor = strings.TrimSpace(string(output))
	}

	return nil
}

func (d *Detector) detectCPUGeneric() error {
	// Generic fallback
	d.info.CPU.Model = "Unknown"
	d.info.CPU.Vendor = "Unknown"
	return nil
}

func (d *Detector) detectGPU() error {
	// Try different GPU detection methods based on platform
	switch runtime.GOOS {
	case "linux":
		return d.detectGPULinux()
	case "darwin":
		return d.detectGPUMacOS()
	default:
		return d.detectGPUGeneric()
	}
}

func (d *Detector) detectGPULinux() error {
	// Try to detect NVIDIA GPUs
	if _, err := exec.LookPath("nvidia-smi"); err == nil {
		return d.detectNVIDIA()
	}

	// Fallback to generic detection
	return d.detectGPUGeneric()
}

func (d *Detector) detectGPUMacOS() error {
	// macOS GPU detection
	cmd := exec.Command("system_profiler", "SPDisplaysDataType")
	if output, err := cmd.Output(); err == nil {
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			if strings.Contains(line, "Chipset Model:") {
				parts := strings.Split(line, ":")
				if len(parts) > 1 {
					d.info.GPU.Model = strings.TrimSpace(parts[1])
				}
			} else if strings.Contains(line, "VRAM") {
				parts := strings.Split(line, ":")
				if len(parts) > 1 {
					d.info.GPU.VRAM = strings.TrimSpace(parts[1])
				}
			}
		}
	}

	d.info.GPU.Vendor = "Apple"
	d.info.GPU.SupportsMetal = true

	return nil
}

func (d *Detector) detectGPUGeneric() error {
	// Generic fallback
	d.info.GPU.Model = "Unknown"
	d.info.GPU.Vendor = "Unknown"
	return nil
}

func (d *Detector) detectNVIDIA() error {
	cmd := exec.Command("nvidia-smi", "--query-gpu=name,memory.total", "--format=csv,noheader")
	if output, err := cmd.Output(); err == nil {
		lines := strings.Split(strings.TrimSpace(string(output)), "\n")
		if len(lines) > 0 {
			parts := strings.Split(lines[0], ", ")
			if len(parts) >= 2 {
				d.info.GPU.Model = strings.TrimSpace(parts[0])
				d.info.GPU.VRAM = strings.TrimSpace(parts[1])
			}
		}
	}

	d.info.GPU.Vendor = "NVIDIA"
	d.info.GPU.SupportsCUDA = true

	return nil
}

func (d *Detector) detectMemory() error {
	switch runtime.GOOS {
	case "linux":
		return d.detectMemoryLinux()
	case "darwin":
		return d.detectMemoryMacOS()
	default:
		return d.detectMemoryGeneric()
	}
}

func (d *Detector) detectMemoryLinux() error {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		log.Print(tr(stdctx.Background(), "internal_hardware_open_meminfo_failed", map[string]any{"Error": err}))
		return d.detectMemoryGeneric()
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "MemTotal:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				memKB, err := strconv.ParseInt(fields[1], 10, 64)
				if err != nil {
					log.Printf("Warning: failed to parse MemTotal value: %v", err)
					return d.detectMemoryGeneric()
				}
				// Convert KB to GB
				memGB := memKB / (1024 * 1024)
				d.info.Memory.TotalRAM = fmt.Sprintf("%dGB", memGB)
				return nil
			}
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("Warning: error reading /proc/meminfo: %v", err)
	}

	return d.detectMemoryGeneric()
}

func (d *Detector) detectMemoryMacOS() error {
	cmd := exec.Command("sysctl", "-n", "hw.memsize")
	output, err := cmd.Output()
	if err != nil {
		log.Printf("Warning: failed to run sysctl for memory: %v", err)
		return d.detectMemoryGeneric()
	}

	memBytes, err := strconv.ParseInt(strings.TrimSpace(string(output)), 10, 64)
	if err != nil {
		log.Printf("Warning: failed to parse memory size from sysctl: %v", err)
		return d.detectMemoryGeneric()
	}

	// Convert bytes to GB
	memGB := memBytes / (1024 * 1024 * 1024)
	d.info.Memory.TotalRAM = fmt.Sprintf("%dGB", memGB)
	return nil
}

func (d *Detector) detectMemoryGeneric() error {
	// Use Go runtime to estimate available memory
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// Estimate total system memory based on Sys (rough approximation)
	// Sys is the total bytes of memory obtained from the OS
	// Multiply by 4 as a heuristic since Go typically uses a fraction
	estimatedTotal := m.Sys * 4
	memGB := estimatedTotal / (1024 * 1024 * 1024)

	// Set a reasonable minimum
	if memGB < 4 {
		memGB = 4
	}

	d.info.Memory.TotalRAM = fmt.Sprintf("%dGB", memGB)
	log.Print(tr(stdctx.Background(), "internal_hardware_memory_fallback_to_estimate", map[string]any{"Estimate": d.info.Memory.TotalRAM}))
	return nil
}

func (d *Detector) detectPlatform() error {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	d.info.Platform = PlatformInfo{
		OS:           runtime.GOOS,
		Architecture: runtime.GOARCH,
		Hostname:     hostname,
	}

	return nil
}
