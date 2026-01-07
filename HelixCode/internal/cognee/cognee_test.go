package cognee

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.code/internal/config"
	"dev.helix.code/internal/hardware"
)

// TestCacheManager tests the CacheManager
func TestCacheManager(t *testing.T) {
	t.Run("NewCacheManager_Success", func(t *testing.T) {
		cfg := map[string]interface{}{"test": "config"}
		cm, err := NewCacheManager(cfg)

		assert.NoError(t, err)
		assert.NotNil(t, cm)
	})

	t.Run("NewCacheManager_NilConfig", func(t *testing.T) {
		cm, err := NewCacheManager(nil)

		assert.NoError(t, err)
		assert.NotNil(t, cm)
	})

	t.Run("CacheManager_Clear", func(t *testing.T) {
		cm, _ := NewCacheManager(nil)
		cm.Clear()
	})
}

// TestCogneeManager tests the CogneeManager
func TestCogneeManager(t *testing.T) {
	t.Run("NewCogneeManager_Success", func(t *testing.T) {
		cfg := &config.HelixConfig{
			Cognee: config.DefaultCogneeConfig(),
		}
		hwProfile := &hardware.HardwareProfile{
			CPU: hardware.CPUInfo{
				Model: "Test CPU",
				Cores: 4,
			},
		}

		cm, err := NewCogneeManager(cfg, hwProfile)

		assert.NoError(t, err)
		assert.NotNil(t, cm)
		assert.NotNil(t, cm.logger)
	})

	t.Run("NewCogneeManager_NilConfig", func(t *testing.T) {
		cm, err := NewCogneeManager(nil, nil)

		assert.NoError(t, err)
		assert.NotNil(t, cm)
	})

	t.Run("ProcessKnowledge_ServiceNotInitialized", func(t *testing.T) {
		cm := &CogneeManager{
			service: nil,
		}
		ctx := context.Background()

		err := cm.ProcessKnowledge(ctx, "test content")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not initialized")
	})

	t.Run("ProcessKnowledge_EmptyContent", func(t *testing.T) {
		cfg := &config.HelixConfig{
			Cognee: config.DefaultCogneeConfig(),
		}
		cm, _ := NewCogneeManager(cfg, nil)
		ctx := context.Background()

		err := cm.ProcessKnowledge(ctx, "")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "empty")
	})

	t.Run("SearchKnowledge_ServiceNotInitialized", func(t *testing.T) {
		cm := &CogneeManager{
			service: nil,
		}
		ctx := context.Background()

		result, err := cm.SearchKnowledge(ctx, "test query")

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "not initialized")
	})

	t.Run("SearchKnowledge_EmptyQuery", func(t *testing.T) {
		cfg := &config.HelixConfig{
			Cognee: config.DefaultCogneeConfig(),
		}
		cm, _ := NewCogneeManager(cfg, nil)
		ctx := context.Background()

		result, err := cm.SearchKnowledge(ctx, "")

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "empty")
	})

	t.Run("GetStatus_ServiceNotInitialized", func(t *testing.T) {
		cm := &CogneeManager{
			service: nil,
		}

		status := cm.GetStatus()

		assert.Equal(t, "not_initialized", status)
	})

	t.Run("GetStatus_WithService", func(t *testing.T) {
		cfg := &config.HelixConfig{
			Cognee: config.DefaultCogneeConfig(),
		}
		cm, _ := NewCogneeManager(cfg, nil)

		status := cm.GetStatus()

		assert.NotEmpty(t, status)
	})

	t.Run("Close_Success", func(t *testing.T) {
		cfg := &config.HelixConfig{
			Cognee: config.DefaultCogneeConfig(),
		}
		cm, _ := NewCogneeManager(cfg, nil)

		err := cm.Close()

		assert.NoError(t, err)
	})

	t.Run("Close_NilService", func(t *testing.T) {
		cm := &CogneeManager{
			service: nil,
		}

		err := cm.Close()

		assert.NoError(t, err)
	})

	t.Run("Close_MultipleCalls", func(t *testing.T) {
		cfg := &config.HelixConfig{
			Cognee: config.DefaultCogneeConfig(),
		}
		cm, _ := NewCogneeManager(cfg, nil)

		err1 := cm.Close()
		err2 := cm.Close()

		assert.NoError(t, err1)
		assert.NoError(t, err2)
	})
}

// TestHostOptimizer tests the HostOptimizer stub
func TestHostOptimizer(t *testing.T) {
	t.Run("NewHostOptimizer_Success", func(t *testing.T) {
		hwProfile := &hardware.HardwareProfile{
			CPU: hardware.CPUInfo{
				Model: "Test CPU",
				Cores: 8,
			},
		}

		ho := NewHostOptimizer(hwProfile)

		assert.NotNil(t, ho)
	})

	t.Run("NewHostOptimizer_NilProfile", func(t *testing.T) {
		ho := NewHostOptimizer(nil)

		assert.NotNil(t, ho)
	})

	t.Run("OptimizeConfig_ReturnsUnchanged", func(t *testing.T) {
		ho := NewHostOptimizer(nil)
		originalConfig := map[string]interface{}{
			"key1": "value1",
			"key2": 123,
		}

		optimizedConfig := ho.OptimizeConfig(originalConfig)

		assert.Equal(t, originalConfig, optimizedConfig)
	})

	t.Run("OptimizeConfig_WithNilConfig", func(t *testing.T) {
		ho := NewHostOptimizer(nil)

		optimizedConfig := ho.OptimizeConfig(nil)

		assert.Nil(t, optimizedConfig)
	})

	t.Run("OptimizeConfig_WithComplexConfig", func(t *testing.T) {
		ho := NewHostOptimizer(&hardware.HardwareProfile{})
		complexConfig := map[string]interface{}{
			"nested": map[string]interface{}{
				"key": "value",
			},
			"array": []int{1, 2, 3},
		}

		optimizedConfig := ho.OptimizeConfig(complexConfig)

		assert.Equal(t, complexConfig, optimizedConfig)
	})
}

// TestPerformanceOptimizer tests the PerformanceOptimizer
func TestPerformanceOptimizer(t *testing.T) {
	t.Run("NewPerformanceOptimizer_Success", func(t *testing.T) {
		cfg := &config.CogneeConfig{
			Enabled: true,
			Mode:    "local",
		}
		hwProfile := &hardware.HardwareProfile{
			CPU: hardware.CPUInfo{
				Model: "Test CPU",
				Cores: 4,
			},
		}

		po, err := NewPerformanceOptimizer(cfg, hwProfile)

		assert.NoError(t, err)
		assert.NotNil(t, po)
		assert.Equal(t, cfg, po.config)
		assert.Equal(t, hwProfile, po.hwProfile)
		assert.False(t, po.initialized)
		assert.False(t, po.running)
	})

	t.Run("NewPerformanceOptimizer_NilConfig", func(t *testing.T) {
		po, err := NewPerformanceOptimizer(nil, nil)

		assert.Error(t, err)
		assert.Nil(t, po)
		assert.Contains(t, err.Error(), "config is required")
	})

	t.Run("GetMetrics_InitialState", func(t *testing.T) {
		cfg := &config.CogneeConfig{Enabled: true}
		po, _ := NewPerformanceOptimizer(cfg, &hardware.HardwareProfile{})

		metrics := po.GetMetrics()

		assert.NotNil(t, metrics)
		assert.Equal(t, float64(0), metrics.TraversalSpeed)
		assert.Greater(t, metrics.MemoryUsage, int64(0), "Memory usage should reflect actual runtime allocation")
	})

	t.Run("GetStatus_InitialState", func(t *testing.T) {
		cfg := &config.CogneeConfig{Enabled: true}
		po, _ := NewPerformanceOptimizer(cfg, &hardware.HardwareProfile{})

		status := po.GetStatus()

		assert.NotNil(t, status)
		assert.Equal(t, false, status["initialized"])
		assert.Equal(t, false, status["running"])
		assert.NotNil(t, status["metrics"])
	})

	t.Run("Optimize_WithoutInitialize_ReturnsError", func(t *testing.T) {
		cfg := &config.CogneeConfig{Enabled: true}
		po, _ := NewPerformanceOptimizer(cfg, &hardware.HardwareProfile{})
		ctx := context.Background()

		result, err := po.Optimize(ctx)

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "not initialized")
	})

	t.Run("Start_WithoutInitialize_ReturnsError", func(t *testing.T) {
		cfg := &config.CogneeConfig{Enabled: true}
		po, _ := NewPerformanceOptimizer(cfg, &hardware.HardwareProfile{})
		ctx := context.Background()

		err := po.Start(ctx)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not initialized")
	})

	t.Run("Initialize_Success", func(t *testing.T) {
		cfg := &config.CogneeConfig{
			Enabled: true,
			Performance: &config.CogneePerformanceConfig{
				Workers: 4,
			},
			Optimization: &config.CogneeOptimizationConfig{
				CPUOptimization:    true,
				MemoryOptimization: true,
			},
		}
		hwProfile := &hardware.HardwareProfile{
			CPU: hardware.CPUInfo{
				Model: "Test CPU",
				Cores: 8,
			},
		}
		po, _ := NewPerformanceOptimizer(cfg, hwProfile)
		ctx := context.Background()

		err := po.Initialize(ctx)

		assert.NoError(t, err)
		assert.True(t, po.initialized)
		assert.NotNil(t, po.cache)
		assert.NotNil(t, po.pool)
		assert.NotNil(t, po.optimizer)
	})

	t.Run("Initialize_Idempotent", func(t *testing.T) {
		cfg := &config.CogneeConfig{
			Enabled: true,
			Performance: &config.CogneePerformanceConfig{
				Workers: 2,
			},
		}
		po, _ := NewPerformanceOptimizer(cfg, &hardware.HardwareProfile{
			CPU: hardware.CPUInfo{Cores: 4},
		})
		ctx := context.Background()

		err1 := po.Initialize(ctx)
		err2 := po.Initialize(ctx)

		assert.NoError(t, err1)
		assert.NoError(t, err2)
		assert.True(t, po.initialized)
	})

	t.Run("Start_Success", func(t *testing.T) {
		cfg := &config.CogneeConfig{
			Enabled: true,
			Performance: &config.CogneePerformanceConfig{
				Workers: 2,
			},
		}
		hwProfile := &hardware.HardwareProfile{
			CPU: hardware.CPUInfo{Cores: 4},
		}
		po, _ := NewPerformanceOptimizer(cfg, hwProfile)
		ctx := context.Background()

		err := po.Initialize(ctx)
		require.NoError(t, err)

		err = po.Start(ctx)

		assert.NoError(t, err)
		assert.True(t, po.running)
	})

	t.Run("Start_Idempotent", func(t *testing.T) {
		cfg := &config.CogneeConfig{
			Enabled: true,
			Performance: &config.CogneePerformanceConfig{
				Workers: 2,
			},
		}
		po, _ := NewPerformanceOptimizer(cfg, &hardware.HardwareProfile{
			CPU: hardware.CPUInfo{Cores: 4},
		})
		ctx := context.Background()

		po.Initialize(ctx)
		err1 := po.Start(ctx)
		err2 := po.Start(ctx)

		assert.NoError(t, err1)
		assert.NoError(t, err2)
	})

	t.Run("Stop_Success", func(t *testing.T) {
		cfg := &config.CogneeConfig{
			Enabled: true,
			Performance: &config.CogneePerformanceConfig{
				Workers: 2,
			},
		}
		po, _ := NewPerformanceOptimizer(cfg, &hardware.HardwareProfile{
			CPU: hardware.CPUInfo{Cores: 4},
		})
		ctx := context.Background()

		po.Initialize(ctx)
		po.Start(ctx)
		err := po.Stop(ctx)

		assert.NoError(t, err)
		assert.False(t, po.running)
	})

	t.Run("Stop_WithoutStart_Success", func(t *testing.T) {
		cfg := &config.CogneeConfig{Enabled: true}
		po, _ := NewPerformanceOptimizer(cfg, &hardware.HardwareProfile{})
		ctx := context.Background()

		err := po.Stop(ctx)

		assert.NoError(t, err)
	})

	t.Run("Optimize_Success", func(t *testing.T) {
		cfg := &config.CogneeConfig{
			Enabled: true,
			Performance: &config.CogneePerformanceConfig{
				Workers: 2,
			},
			Optimization: &config.CogneeOptimizationConfig{
				CPUOptimization:    true,
				MemoryOptimization: true,
			},
		}
		hwProfile := &hardware.HardwareProfile{
			CPU: hardware.CPUInfo{Cores: 4},
		}
		po, _ := NewPerformanceOptimizer(cfg, hwProfile)
		ctx := context.Background()

		po.Initialize(ctx)
		result, err := po.Optimize(ctx)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.NotNil(t, result.MetricsBefore)
		assert.NotNil(t, result.MetricsAfter)
		assert.NotZero(t, result.Timestamp)
		assert.NotZero(t, result.Duration)
	})

	t.Run("Optimize_WithGPU", func(t *testing.T) {
		cfg := &config.CogneeConfig{
			Enabled: true,
			Performance: &config.CogneePerformanceConfig{
				Workers: 2,
			},
			Optimization: &config.CogneeOptimizationConfig{
				GPUOptimization: true,
			},
		}
		hwProfile := &hardware.HardwareProfile{
			CPU: hardware.CPUInfo{Cores: 4},
			GPU: &hardware.GPUInfo{
				Model: "Test GPU",
				VRAM:  "8GB",
			},
		}
		po, _ := NewPerformanceOptimizer(cfg, hwProfile)
		ctx := context.Background()

		po.Initialize(ctx)
		result, err := po.Optimize(ctx)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Greater(t, len(result.AppliedOpts), 0)
	})
}

// TestPerformanceOptimizerConcurrency tests concurrent access
func TestPerformanceOptimizerConcurrency(t *testing.T) {
	t.Run("ConcurrentGetMetrics", func(t *testing.T) {
		cfg := &config.CogneeConfig{Enabled: true}
		po, _ := NewPerformanceOptimizer(cfg, &hardware.HardwareProfile{})

		done := make(chan bool, 10)
		for i := 0; i < 10; i++ {
			go func() {
				metrics := po.GetMetrics()
				assert.NotNil(t, metrics)
				done <- true
			}()
		}

		for i := 0; i < 10; i++ {
			<-done
		}
	})

	t.Run("ConcurrentGetStatus", func(t *testing.T) {
		cfg := &config.CogneeConfig{Enabled: true}
		po, _ := NewPerformanceOptimizer(cfg, &hardware.HardwareProfile{})

		done := make(chan bool, 10)
		for i := 0; i < 10; i++ {
			go func() {
				status := po.GetStatus()
				assert.NotNil(t, status)
				done <- true
			}()
		}

		for i := 0; i < 10; i++ {
			<-done
		}
	})
}

// TestPerformanceMetrics tests the metrics structure
func TestPerformanceMetrics(t *testing.T) {
	t.Run("InitialMetrics_ZeroValues", func(t *testing.T) {
		metrics := &PerformanceMetrics{}

		assert.Equal(t, float64(0), metrics.TraversalSpeed)
		assert.Equal(t, float64(0), metrics.UpdateSpeed)
		assert.Equal(t, float64(0), metrics.QuerySpeed)
		assert.Equal(t, int64(0), metrics.MemoryUsage)
		assert.Equal(t, float64(0), metrics.CPUUsage)
	})

	t.Run("MetricsUpdate_NonZeroValues", func(t *testing.T) {
		cfg := &config.CogneeConfig{Enabled: true}
		po, _ := NewPerformanceOptimizer(cfg, &hardware.HardwareProfile{})

		metrics := po.GetMetrics()

		assert.NotNil(t, metrics)
		assert.NotZero(t, metrics.StartTime)
	})
}

// TestConstructorsWithVariousInputs tests edge cases
func TestConstructorsWithVariousInputs(t *testing.T) {
	t.Run("CogneeManager_WithEmptyConfig", func(t *testing.T) {
		cm, err := NewCogneeManager(&config.HelixConfig{}, nil)
		require.NoError(t, err)
		assert.NotNil(t, cm)
	})

	t.Run("PerformanceOptimizer_WithMinimalConfig", func(t *testing.T) {
		cfg := &config.CogneeConfig{Enabled: false}
		po, err := NewPerformanceOptimizer(cfg, &hardware.HardwareProfile{})
		require.NoError(t, err)
		assert.NotNil(t, po)
		assert.False(t, po.initialized)
	})
}

// TestOptimizationCache tests cache operations
func TestOptimizationCache(t *testing.T) {
	t.Run("Set_And_Get", func(t *testing.T) {
		cache := &OptimizationCache{
			items:          make(map[string]*CacheItem),
			maxSize:        1024 * 1024,
			evictionPolicy: "lru",
		}

		err := cache.Set("key1", "value1", 0)
		assert.NoError(t, err)

		val, found := cache.Get("key1")
		assert.True(t, found)
		assert.Equal(t, "value1", val)
	})

	t.Run("Get_NonExistent", func(t *testing.T) {
		cache := &OptimizationCache{
			items:          make(map[string]*CacheItem),
			maxSize:        1024 * 1024,
			evictionPolicy: "lru",
		}

		val, found := cache.Get("nonexistent")
		assert.False(t, found)
		assert.Nil(t, val)
	})

	t.Run("Delete", func(t *testing.T) {
		cache := &OptimizationCache{
			items:          make(map[string]*CacheItem),
			maxSize:        1024 * 1024,
			evictionPolicy: "lru",
		}

		cache.Set("key1", "value1", 0)
		cache.Delete("key1")

		val, found := cache.Get("key1")
		assert.False(t, found)
		assert.Nil(t, val)
	})

	t.Run("evictLeastRecentlyUsed", func(t *testing.T) {
		cache := &OptimizationCache{
			items:          make(map[string]*CacheItem),
			maxSize:        1024 * 1024,
			evictionPolicy: "lru",
		}

		cache.Set("key1", "value1", 0)
		cache.Set("key2", "value2", 0)
		cache.evictLeastRecentlyUsed()

		assert.LessOrEqual(t, len(cache.items), 2)
	})

	t.Run("evictExpired", func(t *testing.T) {
		cache := &OptimizationCache{
			items:          make(map[string]*CacheItem),
			maxSize:        1024 * 1024,
			evictionPolicy: "lru",
		}

		cache.Set("key1", "value1", 1)
		cache.evictExpired()

		assert.LessOrEqual(t, len(cache.items), 1)
	})
}

// TestCompressionAlgorithms tests algorithm implementations
func TestCompressionAlgorithms(t *testing.T) {
	t.Run("NeuralSymbolicCompression", func(t *testing.T) {
		alg := &NeuralSymbolicCompression{}
		assert.Equal(t, "neural_symbolic", alg.GetName())
		assert.Equal(t, 0.75, alg.GetCompressionRatio())

		_, err := alg.Compress("test")
		assert.NoError(t, err)

		err = alg.Decompress([]byte("test"), nil)
		assert.NoError(t, err)
	})

	t.Run("AdaptiveHuffmanCompression", func(t *testing.T) {
		alg := &AdaptiveHuffmanCompression{}
		assert.Equal(t, "adaptive_huffman", alg.GetName())
		assert.Equal(t, 0.65, alg.GetCompressionRatio())

		_, err := alg.Compress("test")
		assert.NoError(t, err)

		err = alg.Decompress([]byte("test"), nil)
		assert.NoError(t, err)
	})

	t.Run("NeuralEmbeddingCompression", func(t *testing.T) {
		alg := &NeuralEmbeddingCompression{}
		assert.Equal(t, "neural_embedding", alg.GetName())
		assert.Equal(t, 0.80, alg.GetCompressionRatio())

		_, err := alg.Compress("test")
		assert.NoError(t, err)

		err = alg.Decompress([]byte("test"), nil)
		assert.NoError(t, err)
	})
}

// TestTraversalAlgorithms tests traversal implementations
func TestTraversalAlgorithms(t *testing.T) {
	t.Run("ParallelNeuralSymbolicTraversal", func(t *testing.T) {
		alg := &ParallelNeuralSymbolicTraversal{}
		assert.Equal(t, "parallel_neural_symbolic", alg.GetName())
		assert.Equal(t, "O(n log n)", alg.GetComplexity())

		_, err := alg.Traverse(nil, nil)
		assert.NoError(t, err)
	})

	t.Run("GPUAcceleratedTraversal", func(t *testing.T) {
		alg := &GPUAcceleratedTraversal{}
		assert.Equal(t, "gpu_accelerated", alg.GetName())
		assert.Equal(t, "O(n)", alg.GetComplexity())

		_, err := alg.Traverse(nil, nil)
		assert.NoError(t, err)
	})

	t.Run("MemoryOptimizedTraversal", func(t *testing.T) {
		alg := &MemoryOptimizedTraversal{}
		assert.Equal(t, "memory_optimized", alg.GetName())
		assert.Equal(t, "O(n)", alg.GetComplexity())

		_, err := alg.Traverse(nil, nil)
		assert.NoError(t, err)
	})
}

// TestPartitioningAlgorithms tests partitioning implementations
func TestPartitioningAlgorithms(t *testing.T) {
	t.Run("AdaptiveMemoryAwarePartitioning", func(t *testing.T) {
		alg := &AdaptiveMemoryAwarePartitioning{}
		assert.Equal(t, "adaptive_memory_aware", alg.GetName())
		assert.Equal(t, 0.85, alg.GetPartitionQuality())

		_, err := alg.Partition(nil, 4)
		assert.NoError(t, err)
	})

	t.Run("NeuralBasedPartitioning", func(t *testing.T) {
		alg := &NeuralBasedPartitioning{}
		assert.Equal(t, "neural_based", alg.GetName())
		assert.Equal(t, 0.90, alg.GetPartitionQuality())

		_, err := alg.Partition(nil, 4)
		assert.NoError(t, err)
	})

	t.Run("SymbolicOptimizedPartitioning", func(t *testing.T) {
		alg := &SymbolicOptimizedPartitioning{}
		assert.Equal(t, "symbolic_optimized", alg.GetName())
		assert.Equal(t, 0.80, alg.GetPartitionQuality())

		_, err := alg.Partition(nil, 4)
		assert.NoError(t, err)
	})
}

// TestStatusMethods tests status reporting methods
func TestStatusMethods(t *testing.T) {
	t.Run("GetCacheStatus_WithCache", func(t *testing.T) {
		cfg := &config.CogneeConfig{
			Enabled: true,
			Performance: &config.CogneePerformanceConfig{
				Workers: 2,
			},
		}
		po, _ := NewPerformanceOptimizer(cfg, &hardware.HardwareProfile{
			CPU: hardware.CPUInfo{Cores: 4},
		})
		ctx := context.Background()

		po.Initialize(ctx)
		status := po.GetStatus()

		assert.NotNil(t, status)
		assert.NotNil(t, status["cache"])

		cacheStatus := status["cache"].(map[string]interface{})
		assert.Equal(t, "active", cacheStatus["status"])
	})

	t.Run("GetPoolStatus_WithPool", func(t *testing.T) {
		cfg := &config.CogneeConfig{
			Enabled: true,
			Performance: &config.CogneePerformanceConfig{
				Workers: 2,
			},
		}
		po, _ := NewPerformanceOptimizer(cfg, &hardware.HardwareProfile{
			CPU: hardware.CPUInfo{Cores: 4},
		})
		ctx := context.Background()

		po.Initialize(ctx)
		status := po.GetStatus()

		assert.NotNil(t, status)
		assert.NotNil(t, status["pool"])

		poolStatus := status["pool"].(map[string]interface{})
		assert.Equal(t, "active", poolStatus["status"])
		assert.Equal(t, 2, poolStatus["total_workers"])
	})

	t.Run("GetBatchProcessorStatus", func(t *testing.T) {
		cfg := &config.CogneeConfig{
			Enabled: true,
			Performance: &config.CogneePerformanceConfig{
				Workers: 2,
			},
		}
		po, _ := NewPerformanceOptimizer(cfg, &hardware.HardwareProfile{
			CPU: hardware.CPUInfo{Cores: 4},
		})
		ctx := context.Background()

		po.Initialize(ctx)
		status := po.GetStatus()

		assert.NotNil(t, status)
		assert.NotNil(t, status["pool"])

		poolStatus := status["pool"].(map[string]interface{})
		assert.NotNil(t, poolStatus["batch_processor"])

		batchStatus := poolStatus["batch_processor"].(map[string]interface{})
		assert.Equal(t, "active", batchStatus["status"])
	})
}

// TestBackgroundMethods tests background loop methods
func TestBackgroundMethods(t *testing.T) {
	t.Run("CollectMetrics", func(t *testing.T) {
		cfg := &config.CogneeConfig{
			Enabled: true,
			Performance: &config.CogneePerformanceConfig{
				Workers: 2,
			},
		}
		po, _ := NewPerformanceOptimizer(cfg, &hardware.HardwareProfile{
			CPU: hardware.CPUInfo{Cores: 4},
		})
		ctx := context.Background()

		po.Initialize(ctx)

		po.collectMetrics()

		metrics := po.GetMetrics()
		assert.NotNil(t, metrics)
		assert.NotZero(t, metrics.LastUpdate)
	})

	t.Run("MaintainCache", func(t *testing.T) {
		cfg := &config.CogneeConfig{
			Enabled: true,
			Performance: &config.CogneePerformanceConfig{
				Workers: 2,
			},
		}
		po, _ := NewPerformanceOptimizer(cfg, &hardware.HardwareProfile{
			CPU: hardware.CPUInfo{Cores: 4},
		})
		ctx := context.Background()

		po.Initialize(ctx)

		po.maintainCache()

		status := po.GetStatus()
		assert.NotNil(t, status)
		assert.NotNil(t, status["cache"])
	})
}

// TestCogneeService tests the main Cognee service
func TestCogneeService(t *testing.T) {
	t.Run("NewCogneeService_Success", func(t *testing.T) {
		cfg := config.DefaultCogneeConfig()
		hwProfile := &hardware.HardwareProfile{
			CPU: hardware.CPUInfo{Cores: 4},
		}

		service, err := NewCogneeService(cfg, hwProfile)

		assert.NoError(t, err)
		assert.NotNil(t, service)
		assert.Equal(t, ServiceStatusStopped, service.GetStatus())
	})

	t.Run("NewCogneeService_NilConfig", func(t *testing.T) {
		service, err := NewCogneeService(nil, nil)

		assert.Error(t, err)
		assert.Nil(t, service)
	})

	t.Run("Service_StartStop", func(t *testing.T) {
		cfg := config.DefaultCogneeConfig()
		cfg.AutoStart = false

		service, err := NewCogneeService(cfg, nil)
		require.NoError(t, err)

		ctx := context.Background()

		err = service.Start(ctx)
		assert.NoError(t, err)
		assert.Equal(t, ServiceStatusRunning, service.GetStatus())

		err = service.Stop(ctx)
		assert.NoError(t, err)
		assert.Equal(t, ServiceStatusStopped, service.GetStatus())
	})

	t.Run("Service_EnsureRunning", func(t *testing.T) {
		cfg := config.DefaultCogneeConfig()
		cfg.AutoStart = false

		service, err := NewCogneeService(cfg, nil)
		require.NoError(t, err)

		ctx := context.Background()

		err = service.EnsureRunning(ctx)
		assert.NoError(t, err)
		assert.Equal(t, ServiceStatusRunning, service.GetStatus())

		service.Stop(ctx)
	})

	t.Run("Service_GetHealth", func(t *testing.T) {
		cfg := config.DefaultCogneeConfig()

		service, err := NewCogneeService(cfg, nil)
		require.NoError(t, err)

		ctx := context.Background()
		health, err := service.GetHealth(ctx)

		assert.NoError(t, err)
		assert.NotNil(t, health)
	})

	t.Run("Service_GetStatistics", func(t *testing.T) {
		cfg := config.DefaultCogneeConfig()
		cfg.AutoStart = false

		service, err := NewCogneeService(cfg, nil)
		require.NoError(t, err)

		ctx := context.Background()
		service.Start(ctx)

		stats, err := service.GetStatistics(ctx)

		assert.NoError(t, err)
		assert.NotNil(t, stats)

		service.Stop(ctx)
	})
}

// TestCogneeClient tests the Cognee API client
func TestCogneeClient(t *testing.T) {
	t.Run("NewClient_Success", func(t *testing.T) {
		cfg := config.DefaultCogneeConfig()
		client := NewClient(cfg)

		assert.NotNil(t, client)
		assert.Contains(t, client.GetBaseURL(), "localhost")
	})

	t.Run("Client_SetAPIKey", func(t *testing.T) {
		cfg := config.DefaultCogneeConfig()
		client := NewClient(cfg)

		client.SetAPIKey("test-api-key")
		assert.NotNil(t, client)
	})

	t.Run("Client_SetBaseURL", func(t *testing.T) {
		cfg := config.DefaultCogneeConfig()
		client := NewClient(cfg)

		client.SetBaseURL("http://example.com")
		assert.Equal(t, "http://example.com", client.GetBaseURL())
	})

	t.Run("Client_SetTimeout", func(t *testing.T) {
		cfg := config.DefaultCogneeConfig()
		client := NewClient(cfg)

		client.SetTimeout(60 * time.Second)
		assert.NotNil(t, client)
	})

	t.Run("Client_Close", func(t *testing.T) {
		cfg := config.DefaultCogneeConfig()
		client := NewClient(cfg)

		err := client.Close()
		assert.NoError(t, err)
		assert.False(t, client.IsConnected())
	})
}

// TestCogneeModels tests the data models
func TestCogneeModels(t *testing.T) {
	t.Run("AddMemoryRequest", func(t *testing.T) {
		req := &AddMemoryRequest{
			Content:     "test content",
			DatasetName: "test",
			ContentType: "text",
		}

		assert.Equal(t, "test content", req.Content)
		assert.Equal(t, "test", req.DatasetName)
	})

	t.Run("SearchMemoryRequest", func(t *testing.T) {
		req := &SearchMemoryRequest{
			Query:       "test query",
			DatasetName: "test",
			Limit:       10,
		}

		assert.Equal(t, "test query", req.Query)
		assert.Equal(t, 10, req.Limit)
	})

	t.Run("CogneeMemory", func(t *testing.T) {
		memory := &CogneeMemory{
			ID:          "mem-123",
			Content:     "test content",
			DatasetName: "test",
			CreatedAt:   time.Now(),
		}

		assert.Equal(t, "mem-123", memory.ID)
		assert.Equal(t, "test content", memory.Content)
	})

	t.Run("Dataset", func(t *testing.T) {
		dataset := &Dataset{
			ID:          "ds-123",
			Name:        "test-dataset",
			Description: "Test dataset",
			CreatedAt:   time.Now(),
		}

		assert.Equal(t, "ds-123", dataset.ID)
		assert.Equal(t, "test-dataset", dataset.Name)
	})
}
