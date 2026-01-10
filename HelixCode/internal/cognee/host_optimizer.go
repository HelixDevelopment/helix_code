package cognee

import (
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"dev.helix.code/internal/config"
	"dev.helix.code/internal/hardware"
)

// HostOptimizer optimizes Cognee configuration based on host hardware
type HostOptimizer struct {
	profile         *hardware.HardwareProfile
	cpuCores        int
	totalMemoryMB   int64
	gpuAvailable    bool
	gpuMemoryMB     int64
	recommendedLoad float64
}

// NewHostOptimizer creates a new host optimizer based on hardware profile
func NewHostOptimizer(profile *hardware.HardwareProfile) *HostOptimizer {
	ho := &HostOptimizer{
		profile:         profile,
		cpuCores:        runtime.NumCPU(),
		recommendedLoad: 0.75, // Use 75% of available resources by default
	}

	if profile != nil {
		if profile.CPU.Cores > 0 {
			ho.cpuCores = profile.CPU.Cores
		}
		if profile.Memory.Total > 0 {
			ho.totalMemoryMB = int64(profile.Memory.Total / (1024 * 1024))
		}
		if profile.GPU != nil {
			ho.gpuAvailable = true
			ho.gpuMemoryMB = parseVRAMString(profile.GPU.VRAM)
		}
	}

	// Fallback to runtime detection for memory
	if ho.totalMemoryMB == 0 {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		// Estimate total system memory as ~4x heap size (rough heuristic)
		ho.totalMemoryMB = int64(m.Sys / (1024 * 1024) * 4)
		if ho.totalMemoryMB < 1024 {
			ho.totalMemoryMB = 4096 // Default to 4GB
		}
	}

	return ho
}

// OptimizeConfig optimizes a configuration based on host hardware
func (ho *HostOptimizer) OptimizeConfig(cfg interface{}) interface{} {
	if cfg == nil {
		return nil
	}

	switch c := cfg.(type) {
	case *config.CogneeConfig:
		return ho.optimizeCogneeConfig(c)
	case config.CogneeConfig:
		optimized := ho.optimizeCogneeConfig(&c)
		return *optimized
	default:
		// Return unchanged for unsupported types
		return cfg
	}
}

// optimizeCogneeConfig optimizes a CogneeConfig based on hardware
func (ho *HostOptimizer) optimizeCogneeConfig(cfg *config.CogneeConfig) *config.CogneeConfig {
	if cfg == nil {
		return nil
	}

	// Create a copy to avoid modifying the original
	optimized := *cfg

	// Initialize Performance if nil
	if optimized.Performance == nil {
		optimized.Performance = &config.CogneePerformanceConfig{}
	}

	// Initialize Optimization if nil
	if optimized.Optimization == nil {
		optimized.Optimization = &config.CogneeOptimizationConfig{}
	}

	// Optimize worker count based on CPU cores
	optimalWorkers := ho.calculateOptimalWorkers()
	if optimized.Performance.Workers == 0 || optimized.Performance.Workers > optimalWorkers {
		optimized.Performance.Workers = optimalWorkers
	}

	// Optimize queue size based on workers and memory
	optimalQueueSize := ho.calculateOptimalQueueSize(optimized.Performance.Workers)
	if optimized.Performance.QueueSize == 0 || optimized.Performance.QueueSize > optimalQueueSize {
		optimized.Performance.QueueSize = optimalQueueSize
	}

	// Optimize batch size based on memory
	optimalBatchSize := ho.calculateOptimalBatchSize()
	if optimized.Performance.BatchSize == 0 || optimized.Performance.BatchSize > optimalBatchSize {
		optimized.Performance.BatchSize = optimalBatchSize
	}

	// Optimize memory limits
	optimalMaxMemory := ho.calculateOptimalMaxMemory()
	if optimized.Performance.MaxMemory == 0 || optimized.Performance.MaxMemory > optimalMaxMemory {
		optimized.Performance.MaxMemory = optimalMaxMemory
	}

	// Optimize cache size
	optimalCacheSize := ho.calculateOptimalCacheSize()
	if optimized.Performance.CacheSize == 0 || optimized.Performance.CacheSize > optimalCacheSize {
		optimized.Performance.CacheSize = optimalCacheSize
	}

	// Set optimization flags based on hardware capabilities
	optimized.Optimization.HostAware = true
	optimized.Optimization.CPUOptimization = ho.cpuCores >= 4
	optimized.Optimization.MemoryOptimization = ho.totalMemoryMB >= 8192
	optimized.Optimization.GPUOptimization = ho.gpuAvailable && ho.gpuMemoryMB >= 2048

	// Set optimization level based on hardware
	optimized.Performance.OptimizationLevel = ho.determineOptimizationLevel()

	// Add host-specific settings
	if optimized.Optimization.HostSpecific == nil {
		optimized.Optimization.HostSpecific = make(map[string]interface{})
	}
	optimized.Optimization.HostSpecific["cpu_cores"] = ho.cpuCores
	optimized.Optimization.HostSpecific["memory_mb"] = ho.totalMemoryMB
	optimized.Optimization.HostSpecific["gpu_available"] = ho.gpuAvailable
	optimized.Optimization.HostSpecific["gpu_memory_mb"] = ho.gpuMemoryMB

	return &optimized
}

// calculateOptimalWorkers calculates the optimal number of workers
func (ho *HostOptimizer) calculateOptimalWorkers() int {
	// Use 75% of available cores, minimum 1, maximum 32
	workers := int(float64(ho.cpuCores) * ho.recommendedLoad)
	if workers < 1 {
		workers = 1
	}
	if workers > 32 {
		workers = 32
	}
	return workers
}

// calculateOptimalQueueSize calculates the optimal queue size
func (ho *HostOptimizer) calculateOptimalQueueSize(workers int) int {
	// Queue size should be proportional to workers and memory
	// Base: 100 items per worker, adjusted by memory
	baseSize := workers * 100

	// Adjust based on available memory (more memory = larger queue)
	memoryFactor := float64(ho.totalMemoryMB) / 8192.0 // 8GB baseline
	if memoryFactor < 0.5 {
		memoryFactor = 0.5
	}
	if memoryFactor > 2.0 {
		memoryFactor = 2.0
	}

	queueSize := int(float64(baseSize) * memoryFactor)
	if queueSize < 100 {
		queueSize = 100
	}
	if queueSize > 10000 {
		queueSize = 10000
	}

	return queueSize
}

// calculateOptimalBatchSize calculates the optimal batch size
func (ho *HostOptimizer) calculateOptimalBatchSize() int {
	// Batch size based on memory and CPU
	// More memory and CPU = larger batches
	baseBatch := 32

	// Adjust for CPU
	if ho.cpuCores >= 16 {
		baseBatch = 128
	} else if ho.cpuCores >= 8 {
		baseBatch = 64
	} else if ho.cpuCores >= 4 {
		baseBatch = 32
	} else {
		baseBatch = 16
	}

	// Adjust for memory
	if ho.totalMemoryMB >= 32768 { // 32GB+
		baseBatch *= 2
	} else if ho.totalMemoryMB < 4096 { // Less than 4GB
		baseBatch /= 2
	}

	if baseBatch < 8 {
		baseBatch = 8
	}
	if baseBatch > 512 {
		baseBatch = 512
	}

	return baseBatch
}

// calculateOptimalMaxMemory calculates the optimal max memory limit in bytes
func (ho *HostOptimizer) calculateOptimalMaxMemory() int64 {
	// Use 50% of available memory for Cognee operations
	maxMemoryMB := int64(float64(ho.totalMemoryMB) * 0.5)

	// Minimum 512MB, maximum 64GB
	if maxMemoryMB < 512 {
		maxMemoryMB = 512
	}
	if maxMemoryMB > 65536 {
		maxMemoryMB = 65536
	}

	return maxMemoryMB * 1024 * 1024 // Convert to bytes
}

// calculateOptimalCacheSize calculates the optimal cache size in bytes
func (ho *HostOptimizer) calculateOptimalCacheSize() int64 {
	// Use 25% of available memory for caching
	cacheSizeMB := int64(float64(ho.totalMemoryMB) * 0.25)

	// Minimum 256MB, maximum 32GB
	if cacheSizeMB < 256 {
		cacheSizeMB = 256
	}
	if cacheSizeMB > 32768 {
		cacheSizeMB = 32768
	}

	return cacheSizeMB * 1024 * 1024 // Convert to bytes
}

// determineOptimizationLevel determines the optimization level string
func (ho *HostOptimizer) determineOptimizationLevel() string {
	score := 0

	// Score based on CPU
	if ho.cpuCores >= 16 {
		score += 3
	} else if ho.cpuCores >= 8 {
		score += 2
	} else if ho.cpuCores >= 4 {
		score += 1
	}

	// Score based on memory
	if ho.totalMemoryMB >= 32768 { // 32GB+
		score += 3
	} else if ho.totalMemoryMB >= 16384 { // 16GB+
		score += 2
	} else if ho.totalMemoryMB >= 8192 { // 8GB+
		score += 1
	}

	// Score based on GPU
	if ho.gpuAvailable {
		if ho.gpuMemoryMB >= 8192 { // 8GB+ VRAM
			score += 3
		} else if ho.gpuMemoryMB >= 4096 { // 4GB+ VRAM
			score += 2
		} else {
			score += 1
		}
	}

	// Map score to optimization level
	switch {
	case score >= 8:
		return "ultra"
	case score >= 6:
		return "high"
	case score >= 4:
		return "medium"
	case score >= 2:
		return "low"
	default:
		return "minimal"
	}
}

// GetOptimizationReport returns a report of optimization decisions
func (ho *HostOptimizer) GetOptimizationReport() map[string]interface{} {
	return map[string]interface{}{
		"hardware": map[string]interface{}{
			"cpu_cores":      ho.cpuCores,
			"memory_mb":      ho.totalMemoryMB,
			"gpu_available":  ho.gpuAvailable,
			"gpu_memory_mb":  ho.gpuMemoryMB,
		},
		"recommendations": map[string]interface{}{
			"workers":            ho.calculateOptimalWorkers(),
			"queue_size":         ho.calculateOptimalQueueSize(ho.calculateOptimalWorkers()),
			"batch_size":         ho.calculateOptimalBatchSize(),
			"max_memory_bytes":   ho.calculateOptimalMaxMemory(),
			"cache_size_bytes":   ho.calculateOptimalCacheSize(),
			"optimization_level": ho.determineOptimizationLevel(),
		},
		"optimizations": map[string]interface{}{
			"cpu_optimization":    ho.cpuCores >= 4,
			"memory_optimization": ho.totalMemoryMB >= 8192,
			"gpu_optimization":    ho.gpuAvailable && ho.gpuMemoryMB >= 2048,
		},
	}
}

// SetRecommendedLoad sets the recommended load factor (0.0 to 1.0)
func (ho *HostOptimizer) SetRecommendedLoad(load float64) {
	if load < 0.1 {
		load = 0.1
	}
	if load > 1.0 {
		load = 1.0
	}
	ho.recommendedLoad = load
}

// parseVRAMString parses a VRAM string like "8GB" or "16GB" and returns MB
func parseVRAMString(vram string) int64 {
	if vram == "" {
		return 0
	}

	// Use regex to extract number and unit
	re := regexp.MustCompile(`(\d+(?:\.\d+)?)\s*(GB|MB|TB)?`)
	matches := re.FindStringSubmatch(strings.ToUpper(vram))
	if len(matches) < 2 {
		return 0
	}

	value, err := strconv.ParseFloat(matches[1], 64)
	if err != nil {
		return 0
	}

	// Determine unit and convert to MB
	unit := "GB" // Default to GB
	if len(matches) >= 3 && matches[2] != "" {
		unit = matches[2]
	}

	switch unit {
	case "TB":
		return int64(value * 1024 * 1024)
	case "GB":
		return int64(value * 1024)
	case "MB":
		return int64(value)
	default:
		return int64(value * 1024) // Assume GB
	}
}
