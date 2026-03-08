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
	service *webhook.WebhookService
	queue   chan webhookJob
	wg      sync.WaitGroup
}

type webhookJob struct {
	payload []byte
	url     string
	meta    webhook.WebhookMetadata
}

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

func (d *webhookDispatcher) worker() {
	defer d.wg.Done()
	for job := range d.queue {
		if err := d.service.SendToWebhook(job.payload, job.url, job.meta); err != nil {
			log.Printf("[Tracer] Failed to send webhook: %v", err)
		}
	}
}

func (d *webhookDispatcher) Enqueue(payload []byte, url string, meta webhook.WebhookMetadata) {
	select {
	case d.queue <- webhookJob{payload: payload, url: url, meta: meta}:
		// Job queued successfully
	default:
		// Queue full - drop the message
		log.Printf("[Tracer] Webhook queue full, dropping message")
	}
}

func (d *webhookDispatcher) Stop() {
	close(d.queue)
	d.wg.Wait()
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

// StopWebhookDispatcher stops the webhook dispatcher and waits for in-flight deliveries
func StopWebhookDispatcher() {
	if globalWebhookDispatcher != nil {
		globalWebhookDispatcher.Stop()
	}
}

// Service: Manages package deployments
// Injects a DatabaseRepository and ConfigRepository to interact with the database
type TracerService struct {
	db        ports.DatabaseRepository
	bolt      ports.ConfigRepository
	processMu sync.Mutex
}

// Constructor: NewTracerService Constructor for TracerService
func NewTracerService(db ports.DatabaseRepository, bolt ports.ConfigRepository) *TracerService {
	return &TracerService{
		db:   db,
		bolt: bolt,
	}
}

func (ts *TracerService) StartEventListener(ctx context.Context, subscriber *domain.Subscriber, schema string) error {
	if subscriber == nil {
		return fmt.Errorf("subscriber cannot be nil")
	}
	fmt.Println("[Tracer] Starting event listener for subscriber:", subscriber.Name())

	// Initial processing to handle any existing messages
	// any remaining messages for the subscriber that was sent before starting the listener will be processed here
	go func() {
		if err := ts.processBatch(ctx, subscriber); err != nil {
			log.Printf("initial batch processing failed for subscriber %s: %v", subscriber.Name(), err)
		}
	}()

	// Start the goroutine to listen for notifications
	go ts.blockingConsumerLoop(ctx, subscriber)

	return nil
}

func (ts *TracerService) blockingConsumerLoop(ctx context.Context, subscriber *domain.Subscriber) {
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
	// Lock for processing to avoid concurrent dequeues
	ts.processMu.Lock()
	defer ts.processMu.Unlock()

	messages, msgIDs, count, err := ts.db.BulkDequeueTracerMessages(ctx, *subscriber)

	if err != nil {
		return err
	}

	if count == 0 {
		return nil // return
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

		ts.handleTracerMessage(msg)
	}
	return nil
}

// handleTracerMessage processes a single tracer message
func (ts *TracerService) handleTracerMessage(msg *domain.QueueMessage) {
	fmt.Println(msg.Format())

	// Check if webhook is enabled and message has flag
	if msg.SendToWebhook() {
		// Get webhook config from BoltDB
		config, err := ts.bolt.GetWebhookConfig()
		if err != nil {
			log.Printf("[Tracer] Failed to load webhook config: %v", err)
			return
		}

		if config == nil || config.URL == "" {
			log.Printf("[Tracer] Message flagged for webhook but no webhook URL configured.")
			return
		}

		if config.Enabled {
			// Use bounded dispatcher instead of per-message goroutine
			dispatcher := getWebhookDispatcher()
			meta := webhook.WebhookMetadata{
				LogLevel:  msg.LogLevel().String(),
				Timestamp: msg.Timestamp().Format("2006-01-02T15:04:05Z07:00"),
			}
			dispatcher.Enqueue([]byte(msg.Payload()), config.URL, meta)
		}
	}
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
