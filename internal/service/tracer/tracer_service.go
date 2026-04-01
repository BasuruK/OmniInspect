package tracer

import (
	"OmniView/assets"
	"OmniView/internal/core/domain"
	"OmniView/internal/core/ports"
	"OmniView/internal/service/webhook"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"
)

const (
	// Webhook worker pool settings
	webhookWorkers   = 4
	webhookQueueSize = 100
)

// webhookDispatcher handles bounded webhook delivery
type webhookDispatcher struct {
	service  *webhook.WebhookService
	queue    chan webhookJob
	wg       sync.WaitGroup
	stopped  bool
	mu       sync.RWMutex
	stopOnce sync.Once
}

// webhookJob represents a single webhook delivery task
type webhookJob struct {
	payload []byte
	url     string
	meta    webhook.WebhookMetadata
}

// newWebhookDispatcher creates and starts a new webhookDispatcher with a worker pool
func newWebhookDispatcher() *webhookDispatcher {
	d := &webhookDispatcher{
		service: webhook.NewWebhookService(),
		queue:   make(chan webhookJob, webhookQueueSize),
	}
	// Start worker pool
	for i := 0; i < webhookWorkers; i++ {
		d.wg.Add(1)
		go d.worker()
	}
	return d
}

// worker processes webhook jobs from the queue
func (d *webhookDispatcher) worker() {
	defer d.wg.Done()
	for job := range d.queue {
		if err := d.service.SendToWebhook(job.payload, job.url, job.meta); err != nil {
			log.Printf("[Tracer] Failed to send webhook: %v", err)
		}
	}
}

// Enqueue adds a webhook job to the dispatcher's queue if not stopped
func (d *webhookDispatcher) Enqueue(payload []byte, url string, meta webhook.WebhookMetadata) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if d.stopped {
		log.Printf("[Tracer] Webhook dispatcher stopped, dropping message")
		return
	}

	select {
	case d.queue <- webhookJob{payload: payload, url: url, meta: meta}:
		// Job queued successfully
	default:
		// Queue full - drop the message
		log.Printf("[Tracer] Webhook queue full, dropping message")
	}
}

// Stop signals the dispatcher to stop accepting new jobs and waits for in-flight deliveries to complete
func (d *webhookDispatcher) Stop() {
	d.stopOnce.Do(func() {
		d.mu.Lock()
		d.stopped = true
		close(d.queue)
		d.mu.Unlock()
		d.wg.Wait()
	})
}

// Global webhook dispatcher (initialized on first use)
var globalWebhookDispatcher *webhookDispatcher
var dispatcherOnce sync.Once

func getWebhookDispatcher() *webhookDispatcher {
	dispatcherOnce.Do(func() {
		globalWebhookDispatcher = newWebhookDispatcher()
	})
	return globalWebhookDispatcher
}

// StopAll stops the webhook dispatcher and event listener goroutines, then waits for them to complete.
// Note: The caller (Model) owns the channel lifecycle. The channel is closed by the Model, not here.
func StopAll(tracerService *TracerService) {
	// Cancel the event listener context and wait for goroutines to finish
	if tracerService != nil && tracerService.listenerCancel != nil {
		tracerService.listenerCancel()
		tracerService.listenerWg.Wait()
	}
}

// Service: Manages package deployments
// Injects a DatabaseRepository and ConfigRepository to interact with the database
type TracerService struct {
	db             ports.DatabaseRepository
	bolt           ports.ConfigRepository
	processMu      sync.Mutex
	eventChannel   chan *domain.QueueMessage
	listenerCtx    context.Context
	listenerCancel context.CancelFunc
	listenerWg     sync.WaitGroup
}

// Constructor: NewTracerService Constructor for TracerService
func NewTracerService(
	db ports.DatabaseRepository,
	bolt ports.ConfigRepository,
	eventChannel chan *domain.QueueMessage,
) *TracerService {
	return &TracerService{
		db:           db,
		bolt:         bolt,
		eventChannel: eventChannel,
	}
}

// StartEventListener starts goroutines that listen for new tracer messages for the given subscriber and processes them
func (ts *TracerService) StartEventListener(ctx context.Context, subscriber *domain.Subscriber, schema string) error {
	if subscriber == nil {
		return fmt.Errorf("subscriber cannot be nil")
	}
	fmt.Println("[Tracer] Starting event listener for subscriber:", subscriber.Name())

	// Create a cancellable context for event listeners
	ts.listenerCtx, ts.listenerCancel = context.WithCancel(ctx)

	// Initial processing to handle any existing messages
	// any remaining messages for the subscriber that was sent before starting the listener will be processed here
	ts.listenerWg.Add(1)
	go func() {
		defer ts.listenerWg.Done()
		if err := ts.processBatch(ts.listenerCtx, subscriber); err != nil {
			log.Printf("initial batch processing failed for subscriber %s: %v", subscriber.Name(), err)
		}
	}()

	// Start the goroutine to listen for notifications
	ts.listenerWg.Add(1)
	go ts.blockingConsumerLoop(ts.listenerCtx, subscriber)

	return nil
}

// blockingConsumerLoop continuously waits for new messages for the subscriber and processes them until the context is cancelled
func (ts *TracerService) blockingConsumerLoop(ctx context.Context, subscriber *domain.Subscriber) {
	defer ts.listenerWg.Done()

	const errorDelay = 5 * time.Second
	for {
		// Check if context is cancelled before blocking
		select {
		case <-ctx.Done():
			fmt.Println("Event Listener stopping for subscriber:", subscriber.Name())
			return
		default:
			// Continue to blocking wait
		}

		// Blocking wait — Oracle holds this call until messages arrive or wait time expires
		err := ts.processBatch(ctx, subscriber)
		if err != nil {
			log.Printf("failed to dequeue messages for subscriber %s: %v", subscriber.Name(), err)
			select {
			case <-time.After(errorDelay):
				continue
			case <-ctx.Done():
				return
			}
		}
	}
}

// processBatch processes a batch of tracer data for the given subscriber ID
func (ts *TracerService) processBatch(ctx context.Context, subscriber *domain.Subscriber) error {
	if subscriber == nil {
		return fmt.Errorf("subscriber cannot be nil")
	}

	// Use a closure to ensure the lock is released properly
	err := func() error {
		// Lock for processing to maintain dequeue+delivery atomicity
		ts.processMu.Lock()
		defer ts.processMu.Unlock()

		messages, msgIDs, count, err := ts.db.BulkDequeueTracerMessages(ctx, *subscriber)
		if err != nil {
			return err
		}

		if count == 0 {
			return nil
		}

		if len(messages) < count || len(msgIDs) < count {
			return fmt.Errorf("bulk dequeue invariant violated: count=%d messages=%d msgIDs=%d", count, len(messages), len(msgIDs))
		}

		for i := 0; i < count; i++ {
			msg := &domain.QueueMessage{}
			if err := json.Unmarshal([]byte(messages[i]), msg); err != nil {
				log.Printf("failed to unmarshal message ID %s: %v", msgIDs[i], err)
				continue
			}
			// Deliver while holding lock to preserve ordering
			ts.handleTracerMessage(msg)
		}
		return nil
	}()

	return err
}

// handleTracerMessage processes a single tracer message and dispatches to UI and webhooks
func (ts *TracerService) handleTracerMessage(msg *domain.QueueMessage) {
	// Always send to TUI if channel is available
	if ts.eventChannel != nil {
		ts.eventChannel <- msg
	} else {
		fmt.Println(msg.Format())
	}

	// Dispatch to webhook if configured
	webhookConfig, err := ts.bolt.GetWebhookConfig()
	if err != nil || webhookConfig == nil || webhookConfig.URL == "" {
		return
	}

	payload, err := json.Marshal(msg)
	if err != nil {
		log.Printf("failed to marshal message for webhook: %v", err)
		return
	}

	meta := webhook.WebhookMetadata{
		LogLevel:  string(msg.LogLevel()),
		Timestamp: msg.Timestamp().Format(time.RFC3339),
	}
	getWebhookDispatcher().Enqueue(payload, webhookConfig.URL, meta)
}

// DeployAndCheck ensures the necessary tracer package is deployed and initialized
func (ts *TracerService) DeployAndCheck(ctx context.Context) error {
	// Check if the tracer package is already deployed
	// if not, deploy it
	var exists bool
	if err := deployTracerPackage(ctx, ts, &exists); err != nil {
		return fmt.Errorf("failed to deploy tracer package: %w", err)
	}
	if !exists {
		// Initialize the tracer package
		if err := initializeTracerPackage(ctx, ts); err != nil {
			return fmt.Errorf("failed to initialize tracer package: %w", err)
		}
	}

	return nil
}

// DeployTracerPackage deploys the Omni tracer package to the database if not already present
func deployTracerPackage(ctx context.Context, ts *TracerService, exists *bool) error {
	var err error
	*exists, err = ts.db.PackageExists(ctx, "OMNI_TRACER_API")
	if err != nil {
		return fmt.Errorf("failed to check package existence: %w", err)
	}

	if *exists {
		// Package already exists, no need to deploy
		return nil
	}

	// Read the Omni tracer package file
	omniTracerSQLPackage, err := assets.GetSQLFile("Omni_Tracer.sql")
	if err != nil {
		return fmt.Errorf("failed to read Omni tracer package file: %w", err)
	}

	if err := ts.db.DeployFile(ctx, string(omniTracerSQLPackage)); err != nil {
		return fmt.Errorf("failed to deploy Omni tracer package: %w", err)
	}

	return nil
}

// InitializeTracerPackage initializes the Omni tracer package in the database
func initializeTracerPackage(ctx context.Context, ts *TracerService) error {
	omniInitInsFile, err := assets.GetInsFile("Omni_Initialize.ins")
	if err != nil {
		return fmt.Errorf("failed to read Omni initialize file: %w", err)
	}

	if err := ts.db.ExecuteStatement(ctx, string(omniInitInsFile)); err != nil {
		return fmt.Errorf("failed to deploy Omni initialize file: %w", err)
	}

	return nil
}
