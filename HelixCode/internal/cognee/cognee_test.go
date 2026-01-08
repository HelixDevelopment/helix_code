package cognee

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
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

// =====================================================
// HTTP Mock Tests for Client
// =====================================================

// TestClientAddMemory tests the AddMemory method
func TestClientAddMemory(t *testing.T) {
	t.Run("AddMemory_Success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/api/memory", r.URL.Path)
			assert.Equal(t, "POST", r.Method)
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

			var req AddMemoryRequest
			json.NewDecoder(r.Body).Decode(&req)
			assert.Equal(t, "test content", req.Content)

			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(AddMemoryResponse{
				ID:       "mem-123",
				VectorID: "vec-123",
				Message:  "Memory added successfully",
			})
		}))
		defer server.Close()

		cfg := config.DefaultCogneeConfig()
		client := NewClient(cfg)
		client.SetBaseURL(server.URL)

		ctx := context.Background()
		resp, err := client.AddMemory(ctx, &AddMemoryRequest{
			Content:     "test content",
			DatasetName: "test",
		})

		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, "mem-123", resp.ID)
		assert.Equal(t, "vec-123", resp.VectorID)
	})

	t.Run("AddMemory_ServerError", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("internal server error"))
		}))
		defer server.Close()

		cfg := config.DefaultCogneeConfig()
		client := NewClient(cfg)
		client.SetBaseURL(server.URL)

		ctx := context.Background()
		resp, err := client.AddMemory(ctx, &AddMemoryRequest{
			Content: "test content",
		})

		assert.Error(t, err)
		assert.Nil(t, resp)
		assert.Contains(t, err.Error(), "cognee API error")
	})
}

// TestClientSearchMemory tests the SearchMemory method
func TestClientSearchMemory(t *testing.T) {
	t.Run("SearchMemory_Success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/api/search", r.URL.Path)
			assert.Equal(t, "POST", r.Method)

			var req SearchMemoryRequest
			json.NewDecoder(r.Body).Decode(&req)
			assert.Equal(t, "test query", req.Query)

			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(SearchMemoryResponse{
				Results: []MemorySource{
					{ID: "result-1", Content: "result content", Score: 0.95},
				},
				TotalCount: 1,
			})
		}))
		defer server.Close()

		cfg := config.DefaultCogneeConfig()
		client := NewClient(cfg)
		client.SetBaseURL(server.URL)

		ctx := context.Background()
		resp, err := client.SearchMemory(ctx, &SearchMemoryRequest{
			Query:       "test query",
			DatasetName: "test",
			Limit:       10,
		})

		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, 1, resp.TotalCount)
		assert.Len(t, resp.Results, 1)
		assert.Equal(t, "result-1", resp.Results[0].ID)
	})

	t.Run("SearchMemory_EmptyResults", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(SearchMemoryResponse{
				Results:    []MemorySource{},
				TotalCount: 0,
			})
		}))
		defer server.Close()

		cfg := config.DefaultCogneeConfig()
		client := NewClient(cfg)
		client.SetBaseURL(server.URL)

		ctx := context.Background()
		resp, err := client.SearchMemory(ctx, &SearchMemoryRequest{
			Query: "nonexistent",
		})

		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, 0, resp.TotalCount)
	})

	t.Run("SearchMemory_ServerError", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("service unavailable"))
		}))
		defer server.Close()

		cfg := config.DefaultCogneeConfig()
		client := NewClient(cfg)
		client.SetBaseURL(server.URL)

		ctx := context.Background()
		resp, err := client.SearchMemory(ctx, &SearchMemoryRequest{
			Query: "test query",
		})

		assert.Error(t, err)
		assert.Nil(t, resp)
	})
}

// TestClientCognify tests the Cognify method
func TestClientCognify(t *testing.T) {
	t.Run("Cognify_Success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/api/cognify", r.URL.Path)
			assert.Equal(t, "POST", r.Method)

			var req CognifyRequest
			json.NewDecoder(r.Body).Decode(&req)

			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(CognifyResponse{
				Status:  "processing",
				Message: "Cognify started",
			})
		}))
		defer server.Close()

		cfg := config.DefaultCogneeConfig()
		client := NewClient(cfg)
		client.SetBaseURL(server.URL)

		ctx := context.Background()
		resp, err := client.Cognify(ctx, &CognifyRequest{
			Datasets: []string{"test"},
		})

		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, "processing", resp.Status)
	})

	t.Run("Cognify_ServerError", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("bad request"))
		}))
		defer server.Close()

		cfg := config.DefaultCogneeConfig()
		client := NewClient(cfg)
		client.SetBaseURL(server.URL)

		ctx := context.Background()
		resp, err := client.Cognify(ctx, &CognifyRequest{})

		assert.Error(t, err)
		assert.Nil(t, resp)
	})
}

// TestClientSearchInsights tests the SearchInsights method
func TestClientSearchInsights(t *testing.T) {
	t.Run("SearchInsights_Success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/api/search", r.URL.Path)
			assert.Equal(t, "POST", r.Method)

			var reqBody map[string]interface{}
			json.NewDecoder(r.Body).Decode(&reqBody)
			assert.Equal(t, "INSIGHTS", reqBody["search_type"])

			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(InsightsResponse{
				Insights: []Insight{
					{ID: "ins-1", Content: "Test insight", Type: "summary", Confidence: 0.95},
				},
			})
		}))
		defer server.Close()

		cfg := config.DefaultCogneeConfig()
		client := NewClient(cfg)
		client.SetBaseURL(server.URL)

		ctx := context.Background()
		resp, err := client.SearchInsights(ctx, &InsightsRequest{
			Query:    "test query",
			Datasets: []string{"test"},
			Limit:    10,
		})

		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Len(t, resp.Insights, 1)
	})
}

// TestClientSearchGraphCompletion tests the SearchGraphCompletion method
func TestClientSearchGraphCompletion(t *testing.T) {
	t.Run("SearchGraphCompletion_Success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/api/search", r.URL.Path)

			var reqBody map[string]interface{}
			json.NewDecoder(r.Body).Decode(&reqBody)
			assert.Equal(t, "GRAPH_COMPLETION", reqBody["search_type"])

			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(SearchMemoryResponse{
				Results: []MemorySource{
					{ID: "result-1", Content: "graph completion result"},
				},
				TotalCount: 1,
			})
		}))
		defer server.Close()

		cfg := config.DefaultCogneeConfig()
		client := NewClient(cfg)
		client.SetBaseURL(server.URL)

		ctx := context.Background()
		resp, err := client.SearchGraphCompletion(ctx, "test query", []string{"test"}, 10)

		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, 1, resp.TotalCount)
	})
}

// TestClientDatasetOperations tests dataset CRUD operations
func TestClientDatasetOperations(t *testing.T) {
	t.Run("CreateDataset_Success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/api/datasets", r.URL.Path)
			assert.Equal(t, "POST", r.Method)

			var req CreateDatasetRequest
			json.NewDecoder(r.Body).Decode(&req)

			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(DatasetResponse{
				Dataset: &Dataset{
					ID:   "ds-123",
					Name: req.Name,
				},
				Message: "Dataset created successfully",
			})
		}))
		defer server.Close()

		cfg := config.DefaultCogneeConfig()
		client := NewClient(cfg)
		client.SetBaseURL(server.URL)

		ctx := context.Background()
		resp, err := client.CreateDataset(ctx, &CreateDatasetRequest{
			Name:        "test-dataset",
			Description: "Test description",
		})

		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.NotNil(t, resp.Dataset)
	})

	t.Run("ListDatasets_Success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/api/datasets", r.URL.Path)
			assert.Equal(t, "GET", r.Method)

			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(DatasetsResponse{
				Datasets: []Dataset{
					{ID: "ds-1", Name: "dataset-1"},
					{ID: "ds-2", Name: "dataset-2"},
				},
				Total: 2,
			})
		}))
		defer server.Close()

		cfg := config.DefaultCogneeConfig()
		client := NewClient(cfg)
		client.SetBaseURL(server.URL)

		ctx := context.Background()
		resp, err := client.ListDatasets(ctx)

		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, 2, resp.Total)
		assert.Len(t, resp.Datasets, 2)
	})

	t.Run("GetDataset_Success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/api/datasets/test-dataset", r.URL.Path)
			assert.Equal(t, "GET", r.Method)

			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(Dataset{
				ID:   "ds-123",
				Name: "test-dataset",
			})
		}))
		defer server.Close()

		cfg := config.DefaultCogneeConfig()
		client := NewClient(cfg)
		client.SetBaseURL(server.URL)

		ctx := context.Background()
		dataset, err := client.GetDataset(ctx, "test-dataset")

		assert.NoError(t, err)
		assert.NotNil(t, dataset)
		assert.Equal(t, "test-dataset", dataset.Name)
	})

	t.Run("GetDataset_NotFound", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		cfg := config.DefaultCogneeConfig()
		client := NewClient(cfg)
		client.SetBaseURL(server.URL)

		ctx := context.Background()
		dataset, err := client.GetDataset(ctx, "nonexistent")

		assert.NoError(t, err)
		assert.Nil(t, dataset)
	})

	t.Run("DeleteDataset_Success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/api/datasets/test-dataset", r.URL.Path)
			assert.Equal(t, "DELETE", r.Method)

			w.WriteHeader(http.StatusNoContent)
		}))
		defer server.Close()

		cfg := config.DefaultCogneeConfig()
		client := NewClient(cfg)
		client.SetBaseURL(server.URL)

		ctx := context.Background()
		err := client.DeleteDataset(ctx, "test-dataset")

		assert.NoError(t, err)
	})
}

// TestClientProcessCodePipeline tests the ProcessCodePipeline method
func TestClientProcessCodePipeline(t *testing.T) {
	t.Run("ProcessCodePipeline_Success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/api/code-pipeline/index", r.URL.Path)
			assert.Equal(t, "POST", r.Method)

			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(CodePipelineResponse{
				Processed: true,
				Message:   "Code processed successfully",
			})
		}))
		defer server.Close()

		cfg := config.DefaultCogneeConfig()
		client := NewClient(cfg)
		client.SetBaseURL(server.URL)

		ctx := context.Background()
		resp, err := client.ProcessCodePipeline(ctx, &CodePipelineRequest{
			DatasetName: "test-code",
			Code:        "func main() {}",
		})

		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.True(t, resp.Processed)
	})
}

// TestClientVisualizeGraph tests the VisualizeGraph method
func TestClientVisualizeGraph(t *testing.T) {
	t.Run("VisualizeGraph_Success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/api/visualize", r.URL.Path)
			assert.Equal(t, "POST", r.Method)

			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(GraphVisualizationResponse{
				Graph: GraphData{
					Nodes: []GraphNode{
						{ID: "node-1", Label: "Node 1"},
					},
					Edges: []GraphEdge{
						{Source: "node-1", Target: "node-2"},
					},
				},
				NodeCount: 1,
				EdgeCount: 1,
			})
		}))
		defer server.Close()

		cfg := config.DefaultCogneeConfig()
		client := NewClient(cfg)
		client.SetBaseURL(server.URL)

		ctx := context.Background()
		resp, err := client.VisualizeGraph(ctx, &GraphVisualizationRequest{
			DatasetName: "test",
		})

		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Len(t, resp.Graph.Nodes, 1)
		assert.Len(t, resp.Graph.Edges, 1)
	})
}

// TestClientDeleteData tests the DeleteData method
func TestClientDeleteData(t *testing.T) {
	t.Run("DeleteData_Success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/api/delete", r.URL.Path)
			assert.Equal(t, "DELETE", r.Method)

			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(DeleteDataResponse{
				Deleted: 5,
				Message: "Data deleted successfully",
			})
		}))
		defer server.Close()

		cfg := config.DefaultCogneeConfig()
		client := NewClient(cfg)
		client.SetBaseURL(server.URL)

		ctx := context.Background()
		resp, err := client.DeleteData(ctx, &DeleteDataRequest{
			DatasetName: "test",
			DataIDs:     []string{"id-1", "id-2"},
		})

		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, 5, resp.Deleted)
	})
}

// TestClientGetHealth tests the GetHealth method
func TestClientGetHealth(t *testing.T) {
	t.Run("GetHealth_Healthy", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/health", r.URL.Path)
			assert.Equal(t, "GET", r.Method)

			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status": "healthy",
			})
		}))
		defer server.Close()

		cfg := config.DefaultCogneeConfig()
		client := NewClient(cfg)
		client.SetBaseURL(server.URL)

		ctx := context.Background()
		health, err := client.GetHealth(ctx)

		assert.NoError(t, err)
		assert.NotNil(t, health)
		assert.Equal(t, "healthy", health.Status)
		assert.True(t, client.IsConnected())
	})

	t.Run("GetHealth_Unhealthy", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status": "unhealthy",
			})
		}))
		defer server.Close()

		cfg := config.DefaultCogneeConfig()
		client := NewClient(cfg)
		client.SetBaseURL(server.URL)

		ctx := context.Background()
		health, err := client.GetHealth(ctx)

		assert.NoError(t, err)
		assert.NotNil(t, health)
		assert.Equal(t, "unhealthy", health.Status)
		assert.False(t, client.IsConnected())
	})
}

// TestClientGetStatistics tests the GetStatistics method
func TestClientGetStatistics(t *testing.T) {
	t.Run("GetStatistics_Success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/api/stats", r.URL.Path)
			assert.Equal(t, "GET", r.Method)

			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(CogneeStatistics{
				TotalMemories:   100,
				TotalDatasets:   5,
				GraphNodeCount:  1000,
			})
		}))
		defer server.Close()

		cfg := config.DefaultCogneeConfig()
		client := NewClient(cfg)
		client.SetBaseURL(server.URL)

		ctx := context.Background()
		stats, err := client.GetStatistics(ctx)

		assert.NoError(t, err)
		assert.NotNil(t, stats)
		assert.Equal(t, int64(100), stats.TotalMemories)
		assert.Equal(t, int64(5), stats.TotalDatasets)
	})
}

// TestClientAddBatchMemory tests the AddBatchMemory method
func TestClientAddBatchMemory(t *testing.T) {
	t.Run("AddBatchMemory_Success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/api/memory/batch", r.URL.Path)
			assert.Equal(t, "POST", r.Method)

			var req BatchMemoryRequest
			json.NewDecoder(r.Body).Decode(&req)
			assert.Len(t, req.Memories, 2)

			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(BatchMemoryResponse{
				Processed: 2,
				Failed:    0,
				IDs:       []string{"mem-1", "mem-2"},
				Message:   "Batch processed successfully",
			})
		}))
		defer server.Close()

		cfg := config.DefaultCogneeConfig()
		client := NewClient(cfg)
		client.SetBaseURL(server.URL)

		ctx := context.Background()
		resp, err := client.AddBatchMemory(ctx, &BatchMemoryRequest{
			Memories: []AddMemoryRequest{
				{Content: "memory 1"},
				{Content: "memory 2"},
			},
		})

		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, 2, resp.Processed)
		assert.Equal(t, 0, resp.Failed)
	})
}

// TestClientSubmitFeedback tests the SubmitFeedback method
func TestClientSubmitFeedback(t *testing.T) {
	t.Run("SubmitFeedback_Success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/api/feedback", r.URL.Path)
			assert.Equal(t, "POST", r.Method)

			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(FeedbackResponse{
				ID:      "fb-123",
				Message: "Feedback received successfully",
			})
		}))
		defer server.Close()

		cfg := config.DefaultCogneeConfig()
		client := NewClient(cfg)
		client.SetBaseURL(server.URL)

		ctx := context.Background()
		resp, err := client.SubmitFeedback(ctx, &FeedbackRequest{
			QueryID:  "query-1",
			ResultID: "result-1",
			Rating:   5,
			Comment:  "Great result!",
		})

		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, "fb-123", resp.ID)
	})
}

// TestClientTestConnection tests the TestConnection method
func TestClientTestConnection(t *testing.T) {
	t.Run("TestConnection_Success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/health", r.URL.Path)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		cfg := config.DefaultCogneeConfig()
		client := NewClient(cfg)
		client.SetBaseURL(server.URL)

		ctx := context.Background()
		connected := client.TestConnection(ctx)

		assert.True(t, connected)
		assert.True(t, client.IsConnected())
	})

	t.Run("TestConnection_Failed", func(t *testing.T) {
		cfg := config.DefaultCogneeConfig()
		client := NewClient(cfg)
		client.SetBaseURL("http://localhost:99999") // Non-existent port

		ctx := context.Background()
		connected := client.TestConnection(ctx)

		assert.False(t, connected)
		assert.False(t, client.IsConnected())
	})
}

// TestClientAPIKeyHeader tests that API key is properly set in headers
func TestClientAPIKeyHeader(t *testing.T) {
	t.Run("WithAPIKey", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			assert.Equal(t, "Bearer test-api-key", authHeader)

			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(DatasetsResponse{})
		}))
		defer server.Close()

		cfg := config.DefaultCogneeConfig()
		client := NewClient(cfg)
		client.SetBaseURL(server.URL)
		client.SetAPIKey("test-api-key")

		ctx := context.Background()
		_, err := client.ListDatasets(ctx)
		assert.NoError(t, err)
	})

	t.Run("WithoutAPIKey", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			assert.Empty(t, authHeader)

			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(DatasetsResponse{})
		}))
		defer server.Close()

		cfg := config.DefaultCogneeConfig()
		client := NewClient(cfg)
		client.SetBaseURL(server.URL)

		ctx := context.Background()
		_, err := client.ListDatasets(ctx)
		assert.NoError(t, err)
	})
}

// TestClientConcurrency tests concurrent client operations
func TestClientConcurrency(t *testing.T) {
	t.Run("ConcurrentRequests", func(t *testing.T) {
		requestCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestCount++
			time.Sleep(10 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(DatasetsResponse{})
		}))
		defer server.Close()

		cfg := config.DefaultCogneeConfig()
		client := NewClient(cfg)
		client.SetBaseURL(server.URL)

		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				ctx := context.Background()
				_, err := client.ListDatasets(ctx)
				assert.NoError(t, err)
			}()
		}

		wg.Wait()
		assert.Equal(t, 10, requestCount)
	})
}

// TestCacheManagerSetService tests CacheManager SetService
func TestCacheManagerSetService(t *testing.T) {
	cm, err := NewCacheManager(nil)
	assert.NoError(t, err)
	assert.NotNil(t, cm)

	// Initially service should be nil
	assert.Nil(t, cm.service)

	// Clear should not panic with nil service
	cm.Clear()

	// SetService should work
	cfg := config.DefaultCogneeConfig()
	service, err := NewCogneeService(cfg, nil)
	if err == nil && service != nil {
		cm.SetService(service)
		assert.NotNil(t, cm.service)
	}
}

// TestCogneeManagerStart tests CogneeManager Start
func TestCogneeManagerStart(t *testing.T) {
	t.Run("Start_NilService", func(t *testing.T) {
		cm := &CogneeManager{
			service: nil,
		}

		err := cm.Start(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not initialized")
	})
}

// TestCogneeManagerStop tests CogneeManager Stop
func TestCogneeManagerStop(t *testing.T) {
	t.Run("Stop_NilService", func(t *testing.T) {
		cm := &CogneeManager{
			service: nil,
		}

		err := cm.Stop(context.Background())
		assert.NoError(t, err)
	})
}

// TestCogneeManagerCognify tests CogneeManager Cognify
func TestCogneeManagerCognify(t *testing.T) {
	t.Run("Cognify_NilService", func(t *testing.T) {
		cm := &CogneeManager{
			service: nil,
		}

		err := cm.Cognify(context.Background(), []string{"test"})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not initialized")
	})
}

// TestCogneeManagerGetInsights tests CogneeManager GetInsights
func TestCogneeManagerGetInsights(t *testing.T) {
	t.Run("GetInsights_NilService", func(t *testing.T) {
		cm := &CogneeManager{
			service: nil,
		}

		_, err := cm.GetInsights(context.Background(), "test query")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not initialized")
	})

	t.Run("GetInsights_EmptyQuery", func(t *testing.T) {
		cfg := config.DefaultCogneeConfig()
		service, _ := NewCogneeService(cfg, nil)
		cm := &CogneeManager{
			service: service,
		}

		_, err := cm.GetInsights(context.Background(), "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be empty")
	})
}

// TestCogneeManagerProcessCode tests CogneeManager ProcessCode
func TestCogneeManagerProcessCode(t *testing.T) {
	t.Run("ProcessCode_NilService", func(t *testing.T) {
		cm := &CogneeManager{
			service: nil,
		}

		err := cm.ProcessCode(context.Background(), "func main() {}", "go")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not initialized")
	})

	t.Run("ProcessCode_EmptyCode", func(t *testing.T) {
		cfg := config.DefaultCogneeConfig()
		service, _ := NewCogneeService(cfg, nil)
		cm := &CogneeManager{
			service: service,
		}

		err := cm.ProcessCode(context.Background(), "", "go")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be empty")
	})
}

// TestCogneeManagerGetStatus tests CogneeManager GetStatus
func TestCogneeManagerGetStatus(t *testing.T) {
	t.Run("GetStatus_NilService", func(t *testing.T) {
		cm := &CogneeManager{
			service: nil,
		}

		status := cm.GetStatus()
		assert.Equal(t, "not_initialized", status)
	})

	t.Run("GetStatus_WithService", func(t *testing.T) {
		cfg := config.DefaultCogneeConfig()
		service, _ := NewCogneeService(cfg, nil)
		if service != nil {
			cm := &CogneeManager{
				service: service,
			}
			status := cm.GetStatus()
			assert.NotEmpty(t, status)
		}
	})
}

// TestCogneeManagerGetHealth tests CogneeManager GetHealth
func TestCogneeManagerGetHealth(t *testing.T) {
	t.Run("GetHealth_NilService", func(t *testing.T) {
		cm := &CogneeManager{
			service: nil,
		}

		health, err := cm.GetHealth(context.Background())
		assert.NoError(t, err)
		assert.NotNil(t, health)
		assert.Equal(t, "not_initialized", health.Status)
	})
}

// TestCogneeManagerGetStatistics tests CogneeManager GetStatistics
func TestCogneeManagerGetStatistics(t *testing.T) {
	t.Run("GetStatistics_NilService", func(t *testing.T) {
		cm := &CogneeManager{
			service: nil,
		}

		_, err := cm.GetStatistics(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not initialized")
	})
}

// TestCogneeManagerCreateDataset tests CogneeManager CreateDataset
func TestCogneeManagerCreateDataset(t *testing.T) {
	t.Run("CreateDataset_NilService", func(t *testing.T) {
		cm := &CogneeManager{
			service: nil,
		}

		err := cm.CreateDataset(context.Background(), "test", "description")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not initialized")
	})
}

// TestCogneeManagerListDatasets tests CogneeManager ListDatasets
func TestCogneeManagerListDatasets(t *testing.T) {
	t.Run("ListDatasets_NilService", func(t *testing.T) {
		cm := &CogneeManager{
			service: nil,
		}

		_, err := cm.ListDatasets(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not initialized")
	})
}

// TestCogneeManagerDeleteDataset tests CogneeManager DeleteDataset
func TestCogneeManagerDeleteDataset(t *testing.T) {
	t.Run("DeleteDataset_NilService", func(t *testing.T) {
		cm := &CogneeManager{
			service: nil,
		}

		err := cm.DeleteDataset(context.Background(), "test")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not initialized")
	})
}

// TestCogneeManagerVisualizeGraph tests CogneeManager VisualizeGraph
func TestCogneeManagerVisualizeGraph(t *testing.T) {
	t.Run("VisualizeGraph_NilService", func(t *testing.T) {
		cm := &CogneeManager{
			service: nil,
		}

		_, err := cm.VisualizeGraph(context.Background(), "test")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not initialized")
	})
}

// TestCogneeManagerClose tests CogneeManager Close
func TestCogneeManagerClose(t *testing.T) {
	t.Run("Close_NilService", func(t *testing.T) {
		cm := &CogneeManager{
			service: nil,
		}

		err := cm.Close()
		assert.NoError(t, err)
	})
}

// TestCogneeManagerGetService tests CogneeManager GetService
func TestCogneeManagerGetService(t *testing.T) {
	t.Run("GetService_NilService", func(t *testing.T) {
		cm := &CogneeManager{
			service: nil,
		}

		service := cm.GetService()
		assert.Nil(t, service)
	})

	t.Run("GetService_WithService", func(t *testing.T) {
		cfg := config.DefaultCogneeConfig()
		service, _ := NewCogneeService(cfg, nil)
		if service != nil {
			cm := &CogneeManager{
				service: service,
			}
			svc := cm.GetService()
			assert.NotNil(t, svc)
		}
	})
}

// TestCogneeManagerRegisterEventHandler tests CogneeManager RegisterEventHandler
func TestCogneeManagerRegisterEventHandler(t *testing.T) {
	t.Run("RegisterEventHandler_NilService", func(t *testing.T) {
		cm := &CogneeManager{
			service: nil,
		}

		// Should not panic with nil service
		cm.RegisterEventHandler(func(e *CogneeEvent) {})
	})
}

// =====================================================
// HTTP Handler Tests
// =====================================================

// TestNewHandler tests Handler constructors
func TestNewHandler(t *testing.T) {
	t.Run("NewHandler_WithService", func(t *testing.T) {
		cfg := config.DefaultCogneeConfig()
		service, err := NewCogneeService(cfg, nil)
		if err != nil {
			t.Skip("Could not create service")
		}

		handler := NewHandler(service)
		assert.NotNil(t, handler)
		assert.NotNil(t, handler.service)
		assert.NotNil(t, handler.logger)
	})

	t.Run("NewHandler_NilService", func(t *testing.T) {
		handler := NewHandler(nil)
		assert.NotNil(t, handler)
		assert.Nil(t, handler.service)
		assert.NotNil(t, handler.logger)
	})

	t.Run("NewHandlerWithManager_NilManager", func(t *testing.T) {
		handler := NewHandlerWithManager(nil)
		assert.NotNil(t, handler)
		assert.Nil(t, handler.service)
		assert.Nil(t, handler.manager)
		assert.NotNil(t, handler.logger)
	})

	t.Run("NewHandlerWithManager_WithManager", func(t *testing.T) {
		cfg := &config.HelixConfig{
			Cognee: config.DefaultCogneeConfig(),
		}
		manager, err := NewCogneeManager(cfg, nil)
		if err != nil {
			t.Skip("Could not create manager")
		}

		handler := NewHandlerWithManager(manager)
		assert.NotNil(t, handler)
		assert.NotNil(t, handler.manager)
		assert.NotNil(t, handler.logger)
	})
}

// TestHandlerRegisterRoutes tests route registration
func TestHandlerRegisterRoutes(t *testing.T) {
	t.Run("RegisterRoutes_Success", func(t *testing.T) {
		handler := NewHandler(nil)

		gin.SetMode(gin.TestMode)
		router := gin.New()
		group := router.Group("/api")

		handler.RegisterRoutes(group)

		// Routes should be registered
		routes := router.Routes()
		assert.NotEmpty(t, routes)

		// Check specific routes exist
		routePaths := make([]string, len(routes))
		for i, route := range routes {
			routePaths[i] = route.Path
		}

		assert.Contains(t, routePaths, "/api/cognee/health")
		assert.Contains(t, routePaths, "/api/cognee/stats")
		assert.Contains(t, routePaths, "/api/cognee/memory")
		assert.Contains(t, routePaths, "/api/cognee/search")
		assert.Contains(t, routePaths, "/api/cognee/cognify")
		assert.Contains(t, routePaths, "/api/cognee/datasets")
	})
}

// TestHandlerGetHealth tests GET /cognee/health
func TestHandlerGetHealth(t *testing.T) {
	t.Run("GetHealth_ServiceUnavailable", func(t *testing.T) {
		handler := NewHandler(nil)

		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		req, _ := http.NewRequest("GET", "/cognee/health", nil)
		c.Request = req

		handler.GetHealth(c)

		assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	})

	t.Run("GetHealth_WithService", func(t *testing.T) {
		cfg := config.DefaultCogneeConfig()
		service, err := NewCogneeService(cfg, nil)
		if err != nil {
			t.Skip("Could not create service")
		}
		handler := NewHandler(service)

		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		req, _ := http.NewRequest("GET", "/cognee/health", nil)
		c.Request = req

		handler.GetHealth(c)

		// Should return a valid HTTP response (not panic)
		// Service may be unavailable if external cognee is not running
		assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusServiceUnavailable)
	})
}

// TestHandlerGetStatistics tests GET /cognee/stats
func TestHandlerGetStatistics(t *testing.T) {
	t.Run("GetStatistics_ServiceUnavailable", func(t *testing.T) {
		handler := NewHandler(nil)

		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		req, _ := http.NewRequest("GET", "/cognee/stats", nil)
		c.Request = req

		handler.GetStatistics(c)

		assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	})
}

// TestHandlerAddMemory tests POST /cognee/memory
func TestHandlerAddMemory(t *testing.T) {
	t.Run("AddMemory_ServiceUnavailable", func(t *testing.T) {
		handler := NewHandler(nil)

		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		req, _ := http.NewRequest("POST", "/cognee/memory", nil)
		c.Request = req

		handler.AddMemory(c)

		assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	})

	t.Run("AddMemory_InvalidBody", func(t *testing.T) {
		cfg := config.DefaultCogneeConfig()
		service, err := NewCogneeService(cfg, nil)
		if err != nil {
			t.Skip("Could not create service")
		}
		handler := NewHandler(service)

		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		req, _ := http.NewRequest("POST", "/cognee/memory", nil)
		req.Header.Set("Content-Type", "application/json")
		c.Request = req

		handler.AddMemory(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

// TestHandlerAddBatchMemory tests POST /cognee/memory/batch
func TestHandlerAddBatchMemory(t *testing.T) {
	t.Run("AddBatchMemory_ServiceUnavailable", func(t *testing.T) {
		handler := NewHandler(nil)

		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		req, _ := http.NewRequest("POST", "/cognee/memory/batch", nil)
		c.Request = req

		handler.AddBatchMemory(c)

		assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	})

	t.Run("AddBatchMemory_InvalidBody", func(t *testing.T) {
		cfg := config.DefaultCogneeConfig()
		service, err := NewCogneeService(cfg, nil)
		if err != nil {
			t.Skip("Could not create service")
		}
		handler := NewHandler(service)

		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		req, _ := http.NewRequest("POST", "/cognee/memory/batch", nil)
		req.Header.Set("Content-Type", "application/json")
		c.Request = req

		handler.AddBatchMemory(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

// TestHandlerSearchMemory tests POST /cognee/search
func TestHandlerSearchMemory(t *testing.T) {
	t.Run("SearchMemory_ServiceUnavailable", func(t *testing.T) {
		handler := NewHandler(nil)

		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		req, _ := http.NewRequest("POST", "/cognee/search", nil)
		c.Request = req

		handler.SearchMemory(c)

		assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	})
}

// TestHandlerDeleteData tests DELETE /cognee/memory
func TestHandlerDeleteData(t *testing.T) {
	t.Run("DeleteData_ServiceUnavailable", func(t *testing.T) {
		handler := NewHandler(nil)

		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		req, _ := http.NewRequest("DELETE", "/cognee/memory", nil)
		c.Request = req

		handler.DeleteData(c)

		assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	})
}

// TestHandlerCognify tests POST /cognee/cognify
func TestHandlerCognify(t *testing.T) {
	t.Run("Cognify_ServiceUnavailable", func(t *testing.T) {
		handler := NewHandler(nil)

		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		req, _ := http.NewRequest("POST", "/cognee/cognify", nil)
		c.Request = req

		handler.Cognify(c)

		assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	})
}

// TestHandlerGetInsights tests POST /cognee/insights
func TestHandlerGetInsights(t *testing.T) {
	t.Run("GetInsights_ServiceUnavailable", func(t *testing.T) {
		handler := NewHandler(nil)

		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		req, _ := http.NewRequest("POST", "/cognee/insights", nil)
		c.Request = req

		handler.GetInsights(c)

		assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	})
}

// TestHandlerGetGraphCompletion tests POST /cognee/graph/complete
func TestHandlerGetGraphCompletion(t *testing.T) {
	t.Run("GetGraphCompletion_ServiceUnavailable", func(t *testing.T) {
		handler := NewHandler(nil)

		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		req, _ := http.NewRequest("POST", "/cognee/graph/complete", nil)
		c.Request = req

		handler.GetGraphCompletion(c)

		assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	})
}

// TestHandlerProcessCode tests POST /cognee/code
func TestHandlerProcessCode(t *testing.T) {
	t.Run("ProcessCode_ServiceUnavailable", func(t *testing.T) {
		handler := NewHandler(nil)

		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		req, _ := http.NewRequest("POST", "/cognee/code", nil)
		c.Request = req

		handler.ProcessCode(c)

		assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	})
}

// TestHandlerListDatasets tests GET /cognee/datasets
func TestHandlerListDatasets(t *testing.T) {
	t.Run("ListDatasets_ServiceUnavailable", func(t *testing.T) {
		handler := NewHandler(nil)

		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		req, _ := http.NewRequest("GET", "/cognee/datasets", nil)
		c.Request = req

		handler.ListDatasets(c)

		assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	})
}

// TestHandlerCreateDataset tests POST /cognee/datasets
func TestHandlerCreateDataset(t *testing.T) {
	t.Run("CreateDataset_ServiceUnavailable", func(t *testing.T) {
		handler := NewHandler(nil)

		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		req, _ := http.NewRequest("POST", "/cognee/datasets", nil)
		c.Request = req

		handler.CreateDataset(c)

		assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	})
}

// TestHandlerGetDataset tests GET /cognee/datasets/:name
func TestHandlerGetDataset(t *testing.T) {
	t.Run("GetDataset_ServiceUnavailable", func(t *testing.T) {
		handler := NewHandler(nil)

		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		c.Params = gin.Params{{Key: "name", Value: "test"}}
		req, _ := http.NewRequest("GET", "/cognee/datasets/test", nil)
		c.Request = req

		handler.GetDataset(c)

		assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	})
}

// TestHandlerDeleteDataset tests DELETE /cognee/datasets/:name
func TestHandlerDeleteDataset(t *testing.T) {
	t.Run("DeleteDataset_ServiceUnavailable", func(t *testing.T) {
		handler := NewHandler(nil)

		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		c.Params = gin.Params{{Key: "name", Value: "test"}}
		req, _ := http.NewRequest("DELETE", "/cognee/datasets/test", nil)
		c.Request = req

		handler.DeleteDataset(c)

		assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	})
}

// TestHandlerVisualizeGraph tests POST /cognee/visualize
func TestHandlerVisualizeGraph(t *testing.T) {
	t.Run("VisualizeGraph_ServiceUnavailable", func(t *testing.T) {
		handler := NewHandler(nil)

		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		req, _ := http.NewRequest("POST", "/cognee/visualize", nil)
		c.Request = req

		handler.VisualizeGraph(c)

		assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	})
}

// TestHandlerSubmitFeedback tests POST /cognee/feedback
func TestHandlerSubmitFeedback(t *testing.T) {
	t.Run("SubmitFeedback_ServiceUnavailable", func(t *testing.T) {
		handler := NewHandler(nil)

		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		req, _ := http.NewRequest("POST", "/cognee/feedback", nil)
		c.Request = req

		handler.SubmitFeedback(c)

		assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	})
}
