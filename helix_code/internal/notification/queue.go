package notification

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// QueuedNotification represents a notification in the queue
type QueuedNotification struct {
	ID           string
	Notification *Notification
	Channels     []string
	EnqueuedAt   time.Time
	Attempts     int
	LastAttempt  time.Time
	MaxRetries   int
}

// NotificationQueue manages a queue of notifications to be sent
type NotificationQueue struct {
	queue    []*QueuedNotification
	mutex    sync.RWMutex
	engine   *NotificationEngine
	workers  int
	stopChan chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup
	stats    *QueueStats
	maxSize  int
}

// QueueStats tracks queue statistics
type QueueStats struct {
	Enqueued  int64
	Dequeued  int64
	Failed    int64
	Succeeded int64
	mutex     sync.Mutex
}

// NewNotificationQueue creates a new notification queue
func NewNotificationQueue(engine *NotificationEngine, workers int, maxSize int) *NotificationQueue {
	return &NotificationQueue{
		queue:    make([]*QueuedNotification, 0),
		engine:   engine,
		workers:  workers,
		stopChan: make(chan struct{}),
		stats:    &QueueStats{},
		maxSize:  maxSize,
	}
}

// Start starts the queue workers
func (q *NotificationQueue) Start() {
	for i := 0; i < q.workers; i++ {
		q.wg.Add(1)
		go q.worker(i)
	}
	log.Printf("Notification queue started with %d workers", q.workers)
}

// Stop stops the queue workers. It is idempotent: the channel is closed exactly
// once (via stopOnce) so a second call never panics on a closed channel. wg.Wait()
// is OUTSIDE the Once so that EVERY caller — including a second CONCURRENT Stop()
// — blocks until all workers have actually exited, rather than a concurrent
// second caller getting a false "stopped" signal while workers still run.
func (q *NotificationQueue) Stop() {
	q.stopOnce.Do(func() {
		close(q.stopChan)
		log.Println("Notification queue stopped")
	})
	q.wg.Wait()
}

// Enqueue adds a notification to the queue
func (q *NotificationQueue) Enqueue(notification *Notification, channels []string, maxRetries int) error {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	if q.maxSize > 0 && len(q.queue) >= q.maxSize {
		return fmt.Errorf("queue is full (size: %d)", len(q.queue))
	}

	queued := &QueuedNotification{
		ID:           fmt.Sprintf("queued-%d", time.Now().UnixNano()),
		Notification: notification,
		Channels:     channels,
		EnqueuedAt:   time.Now(),
		Attempts:     0,
		MaxRetries:   maxRetries,
	}

	q.queue = append(q.queue, queued)

	q.stats.mutex.Lock()
	q.stats.Enqueued++
	q.stats.mutex.Unlock()

	return nil
}

// Dequeue removes and returns the next notification from the queue
func (q *NotificationQueue) Dequeue() *QueuedNotification {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	if len(q.queue) == 0 {
		return nil
	}

	item := q.queue[0]
	q.queue = q.queue[1:]

	q.stats.mutex.Lock()
	q.stats.Dequeued++
	q.stats.mutex.Unlock()

	return item
}

// Size returns the current queue size
func (q *NotificationQueue) Size() int {
	q.mutex.RLock()
	defer q.mutex.RUnlock()
	return len(q.queue)
}

// IsEmpty returns whether the queue is empty
func (q *NotificationQueue) IsEmpty() bool {
	return q.Size() == 0
}

// Clear removes all items from the queue
func (q *NotificationQueue) Clear() {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	q.queue = make([]*QueuedNotification, 0)
}

// GetStats returns a snapshot of the queue statistics.
//
// It returns a VALUE COPY of the counters taken while holding stats.mutex, never
// the live shared *QueueStats. Handing out the live pointer (and releasing the
// mutex on return) let callers read the int64 counters unlocked while worker
// goroutines wrote them in Enqueue/Dequeue/processNext — a data race on the
// documented public API. Copying the counters under the lock makes every read by
// the caller race-free. The returned QueueStats carries its own zero-value mutex,
// which is unused and must not be copied/shared further.
func (q *NotificationQueue) GetStats() QueueStats {
	q.stats.mutex.Lock()
	defer q.stats.mutex.Unlock()
	return QueueStats{
		Enqueued:  q.stats.Enqueued,
		Dequeued:  q.stats.Dequeued,
		Failed:    q.stats.Failed,
		Succeeded: q.stats.Succeeded,
	}
}

// ResetStats resets queue statistics
func (q *NotificationQueue) ResetStats() {
	q.stats.mutex.Lock()
	defer q.stats.mutex.Unlock()
	q.stats = &QueueStats{}
}

// worker processes notifications from the queue
func (q *NotificationQueue) worker(id int) {
	defer q.wg.Done()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-q.stopChan:
			return
		case <-ticker.C:
			q.processNext(id)
		}
	}
}

// processNext processes the next notification in the queue
func (q *NotificationQueue) processNext(workerID int) {
	item := q.Dequeue()
	if item == nil {
		return
	}

	item.Attempts++
	item.LastAttempt = time.Now()

	// Send notification
	err := q.engine.SendDirect(context.Background(), item.Notification, item.Channels)

	if err != nil {
		log.Printf("Worker %d: Failed to send queued notification %s (attempt %d/%d): %v",
			workerID, item.ID, item.Attempts, item.MaxRetries, err)

		// Retry if under max retries
		if item.Attempts < item.MaxRetries {
			q.mutex.Lock()
			q.queue = append(q.queue, item)
			q.mutex.Unlock()
		} else {
			q.stats.mutex.Lock()
			q.stats.Failed++
			q.stats.mutex.Unlock()
			log.Printf("Worker %d: Notification %s failed after %d attempts", workerID, item.ID, item.Attempts)
		}
	} else {
		q.stats.mutex.Lock()
		q.stats.Succeeded++
		q.stats.mutex.Unlock()
	}
}

// GetQueueItems returns a copy of current queue items (for inspection)
func (q *NotificationQueue) GetQueueItems() []*QueuedNotification {
	q.mutex.RLock()
	defer q.mutex.RUnlock()

	items := make([]*QueuedNotification, len(q.queue))
	copy(items, q.queue)
	return items
}
