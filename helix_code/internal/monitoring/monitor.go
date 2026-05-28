package monitoring

import (
	"context"
	"fmt"
	"sync"
	"time"

	"dev.helix.code/internal/logging"
)

// Monitor provides system monitoring functionality
type Monitor struct {
	logger     *logging.Logger
	metrics    map[string]interface{}
	mutex      sync.RWMutex
	collectors []Collector
}

// Collector defines the interface for metric collectors
type Collector interface {
	Name() string
	Collect() (map[string]interface{}, error)
}

// NewMonitor creates a new monitoring instance
func NewMonitor() *Monitor {
	return &Monitor{
		logger:     logging.DefaultLogger(),
		metrics:    make(map[string]interface{}),
		collectors: make([]Collector, 0),
	}
}

// AddCollector adds a metric collector
func (m *Monitor) AddCollector(collector Collector) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.collectors = append(m.collectors, collector)
}

// CollectMetrics collects metrics from all registered collectors
func (m *Monitor) CollectMetrics(ctx context.Context) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	for _, collector := range m.collectors {
		metrics, err := m.safeCollect(ctx, collector)
		if err != nil {
			m.logger.Error("%s", tr(ctx, "internal_monitoring_failed_collect_metrics_named", map[string]any{"Name": collector.Name(), "Err": err.Error()}))
			continue
		}

		for key, value := range metrics {
			m.metrics[key] = value
		}
	}

	return nil
}

// safeCollect invokes a single collector's Collect() with panic isolation. A
// collector backed by a faulting source (HTTP scrape, /proc read, disk stat)
// can panic; without isolation that panic would unwind out of CollectMetrics
// through the locked critical section and crash the entire process, taking down
// every unrelated goroutine. safeCollect converts a panic into a normal error
// so the panicking collector is skipped (logged by the caller) while its
// co-collectors still run and the Monitor stays usable.
func (m *Monitor) safeCollect(ctx context.Context, collector Collector) (metrics map[string]interface{}, err error) {
	defer func() {
		if r := recover(); r != nil {
			metrics = nil
			err = fmt.Errorf("%s", tr(ctx, "internal_monitoring_collector_panic", map[string]any{"Name": collector.Name(), "Panic": fmt.Sprintf("%v", r)}))
		}
	}()
	return collector.Collect()
}

// GetMetric retrieves a specific metric
func (m *Monitor) GetMetric(key string) (interface{}, bool) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	value, exists := m.metrics[key]
	return value, exists
}

// GetAllMetrics returns all collected metrics
func (m *Monitor) GetAllMetrics() map[string]interface{} {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// Return a copy to avoid race conditions
	metrics := make(map[string]interface{})
	for k, v := range m.metrics {
		metrics[k] = v
	}
	return metrics
}

// StartPeriodicCollection starts periodic metric collection
func (m *Monitor) StartPeriodicCollection(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := m.CollectMetrics(ctx); err != nil {
				m.logger.Error("%s", tr(ctx, "internal_monitoring_failed_collect_metrics", map[string]any{"Err": err.Error()}))
			}
		}
	}
}

// HealthCheck performs a health check
func (m *Monitor) HealthCheck() error {
	// Basic health check - can be extended
	if len(m.collectors) == 0 {
		return fmt.Errorf("%s", tr(context.Background(), "internal_monitoring_no_collectors_registered", nil))
	}
	return nil
}
