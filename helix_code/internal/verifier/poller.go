package verifier

import (
	"context"
	"sync"
	"time"
)

// Poller runs a background goroutine that polls LLMsVerifier at a configurable interval.
// Since LLMsVerifier has no push/webhook support, polling is the only real-time mechanism.
type Poller struct {
	adapter    *Adapter
	interval   time.Duration
	ticker     *time.Ticker
	stopCh     chan struct{}
	wg         sync.WaitGroup
	lastModels map[string]*VerifiedModel
	lastScores map[string]float64
	mu         sync.RWMutex
	pollCount  int
	stopOnce   sync.Once
}

// NewPoller creates a poller. Minimum interval is 10s (enforced).
func NewPoller(adapter *Adapter, interval time.Duration) *Poller {
	if interval < 10*time.Second {
		interval = 10 * time.Second
	}
	return &Poller{
		adapter:    adapter,
		interval:   interval,
		stopCh:     make(chan struct{}),
		lastModels: make(map[string]*VerifiedModel),
		lastScores: make(map[string]float64),
	}
}

// Start begins the background polling goroutine.
func (p *Poller) Start() {
	p.wg.Add(1)
	go p.loop()
}

// Stop signals the poller to stop and waits for the goroutine to exit.
//
// Stop is idempotent (sync.Once guards the channel close): it may be called
// multiple times — e.g. via an idempotent server Shutdown that fans out to
// BootstrapResult.Shutdown -> Poller.Stop more than once — without panicking on
// a double close(stopCh). The wait is outside the Once so every caller still
// blocks until the loop goroutine has exited.
func (p *Poller) Stop() {
	p.stopOnce.Do(func() {
		close(p.stopCh)
	})
	p.wg.Wait()
}

// IsRunning returns true if the poller goroutine is active.
func (p *Poller) IsRunning() bool {
	select {
	case <-p.stopCh:
		return false
	default:
		return true
	}
}

func (p *Poller) loop() {
	defer p.wg.Done()

	// Immediate first poll
	p.poll()

	p.ticker = time.NewTicker(p.interval)
	defer p.ticker.Stop()

	for {
		select {
		case <-p.ticker.C:
			p.poll()
		case <-p.stopCh:
			return
		}
	}
}

func (p *Poller) poll() {
	if p.adapter == nil || !p.adapter.IsEnabled() {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 1. Fetch all models
	models, err := p.adapter.client.GetModels(ctx)
	if err != nil {
		if p.adapter.health != nil {
			p.adapter.health.RecordFailure()
		}
		return
	}

	// 2. Detect changes
	p.mu.Lock()
	newIndex := indexModels(models)
	changes := p.detectChanges(p.lastModels, newIndex)
	p.lastModels = newIndex
	p.mu.Unlock()

	// 3. Update adapter state
	if p.adapter.health != nil {
		p.adapter.health.RecordSuccess()
	}
	p.adapter.refreshScores(models)

	// 4. Update cache
	if p.adapter.cache != nil {
		p.adapter.cache.SetModels("all", models)
	}

	// 5. Publish events for changes
	if p.adapter.events != nil {
		for _, change := range changes {
			_ = p.adapter.events.Publish(change)
		}
	}

	// 6. Fetch scores every 3rd poll (scores change slower than model lists)
	p.pollCount++
	if p.pollCount%3 == 0 {
		scores, _ := p.adapter.client.GetProviderScores(ctx)
		if p.adapter.cache != nil && scores != nil {
			p.adapter.cache.SetScores(scores)
		}
		p.mu.Lock()
		p.lastScores = scores
		p.mu.Unlock()
	}
}

func (p *Poller) detectChanges(old, newModels map[string]*VerifiedModel) []ChangeEvent {
	changes := []ChangeEvent{}
	newIndex := indexModels(modelsToSlice(newModels))

	for id, model := range newIndex {
		if oldModel, ok := old[id]; !ok {
			changes = append(changes, ChangeEvent{
				Type:      "model.discovered",
				Model:     model,
				Timestamp: time.Now(),
			})
		} else {
			if oldModel.OverallScore != model.OverallScore {
				changes = append(changes, ChangeEvent{
					Type:      "model.score_changed",
					Model:     model,
					OldScore:  oldModel.OverallScore,
					Timestamp: time.Now(),
				})
			}
			if oldModel.VerificationStatus != model.VerificationStatus {
				changes = append(changes, ChangeEvent{
					Type:      "model.status_changed",
					Model:     model,
					OldStatus: oldModel.VerificationStatus,
					Timestamp: time.Now(),
				})
			}
		}
	}

	for id, model := range old {
		if _, ok := newIndex[id]; !ok {
			changes = append(changes, ChangeEvent{
				Type:      "model.removed",
				Model:     model,
				Timestamp: time.Now(),
			})
		}
	}

	return changes
}

func indexModels(models []*VerifiedModel) map[string]*VerifiedModel {
	idx := make(map[string]*VerifiedModel, len(models))
	for _, m := range models {
		idx[m.ID] = m
	}
	return idx
}

func modelsToSlice(m map[string]*VerifiedModel) []*VerifiedModel {
	s := make([]*VerifiedModel, 0, len(m))
	for _, v := range m {
		s = append(s, v)
	}
	return s
}
