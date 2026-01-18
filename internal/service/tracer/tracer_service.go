package tracer

import (
	"OmniView/assets"
	"OmniView/internal/adapter/subscription"
	"OmniView/internal/core/domain"
	"OmniView/internal/core/ports"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"
)

// Service: Manages package deployments
// Injects a DatabaseRepository and ConfigRepository to interact with the database
type TracerService struct {
	db              ports.DatabaseRepository
	bolt            ports.ConfigRepository
	subscriptionMgr *subscription.SubscriptionManager
	processMu       sync.Mutex
}

// Constructor: NewTracerService Constructor for TracerService
func NewTracerService(db ports.DatabaseRepository, bolt ports.ConfigRepository) *TracerService {
	rawConn := db.GetRawConnection()
	rawCtx := db.GetRawContext()
	if rawConn == nil || rawCtx == nil {
		fmt.Println("database connection or context is nil during TracerService initialization")
		return nil
	}

	// Note: NewSubscriptionManager expects (connection, context) order
	subscriptionMgr := subscription.NewSubscriptionManager(rawConn, rawCtx)

	return &TracerService{
		db:              db,
		bolt:            bolt,
		subscriptionMgr: subscriptionMgr,
	}
}

func (ts *TracerService) StartEventListener(ctx context.Context, subscriber *domain.Subscriber, schema string) error {
	// Create a notification channel
	notifyChan := make(chan struct{}, 10) // Buffered channel to avoid blocking and handle bursts

	// Subscribe to the queue
	if err := ts.subscriptionMgr.Subscribe(*subscriber, schema, notifyChan); err != nil {
		return fmt.Errorf("failed to subscribe to queue: %w", err)
	}

	fmt.Println("[OCI] Subscription Success for subscriber:", subscriber)

	go func() {
		fmt.Println("queue readiness check!")
		ts.processBatch(subscriber)
	}()

	// Start the goroutine to listen for notifications
	go ts.eventLoop(ctx, notifyChan, subscriber)

	return nil
}

func (ts *TracerService) eventLoop(ctx context.Context, notifyChan <-chan struct{}, subscriber *domain.Subscriber) {
	ticker := time.NewTicker(5 * time.Second) // Periodic check interval, fallback polling. TODO: Take this
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			fmt.Println("Event listener stopped.")
			ts.cleanUp(subscriber)
			return
		case <-notifyChan:
			// Process the notification
			fmt.Println("[GO] Received notification for subscriber:", subscriber.Name)
			ts.processBatch(subscriber)
		case <-ticker.C:
			// Periodic check (fallback polling)
			queueDepth := ts.checkQueueDepth(subscriber)
			if queueDepth > 0 {
				fmt.Printf("[GO] [Periodic check] Processing %d messages for subscriber: %s\n", queueDepth, subscriber.Name)
				ts.processBatch(subscriber)
			}
		}
	}
}

// cleanUp handles cleanup operations when stopping the event listener
func (ts *TracerService) cleanUp(subscriber *domain.Subscriber) {
	if ts.subscriptionMgr != nil {
		if err := ts.subscriptionMgr.Unsubscribe(*subscriber); err != nil {
			fmt.Printf("failed to unsubscribe: %v\n", err)
		} else {
			fmt.Println("Unsubscribed successfully for subscriber:", subscriber.Name)
		}
	}
}

// processBatch processes a batch of tracer data for the given subscriber ID
func (ts *TracerService) processBatch(subscriber *domain.Subscriber) {
	// Lock for processing to avoid concurrent dequeues
	ts.processMu.Lock()
	defer ts.processMu.Unlock()

	messages, msgIDs, count, err := ts.db.BulkDequeueTracerMessages(*subscriber)
	if err != nil {
		log.Printf("failed to dequeue messages for subscriber %s: %v", subscriber.Name, err)
		return
	}

	if count == 0 {
		return // return
	}

	fmt.Printf("[INFO] Processing batch of %d messages for subscriber: %s\n", count, subscriber.Name)

	for i := 0; i < count; i++ {
		var msg domain.QueueMessage
		if err := json.Unmarshal([]byte(messages[i]), &msg); err != nil {
			log.Printf("failed to unmarshal message ID %s: %v", msgIDs[i], err)
			continue
		}

		ts.handleTracerMessage(msg, msgIDs[i])
	}
}

func (ts *TracerService) handleTracerMessage(msg domain.QueueMessage, msgID []byte) {
	fmt.Printf("[%s] [%s] %s: %s (MsgID: %x)\n", msg.Timestamp, msg.LogLevel, msg.ProcessName, msg.Payload, msgID)
}

// checkQueueDepth checks the queue depth for the given subscriber ID
func (ts *TracerService) checkQueueDepth(subscriber *domain.Subscriber) int {
	depth, err := ts.db.CheckQueueDepth(subscriber.Name, domain.QueueTableName)
	if err != nil {
		log.Printf("failed to check queue depth for subscriber %s: %v", subscriber.Name, err)
		return 0
	}
	return depth
}

// DeployAndCheck ensures the necessary tracer package is deployed and initialized
func (ts *TracerService) DeployAndCheck() error {
	// Check if the tracer package is already deployed
	// if not, deploy it
	var exists bool
	if err := deployTracerPackage(ts, &exists); err != nil {
		return fmt.Errorf("failed to deploy tracer package: %w", err)
	}
	if !exists {
		// Initialize the tracer package
		if err := initializeTracerPackage(ts); err != nil {
			return fmt.Errorf("failed to initialize tracer package: %w", err)
		}
	}

	return nil
}

// DeployTracerPackage deploys the Omni tracer package to the database if not already present
func deployTracerPackage(ts *TracerService, exists *bool) error {
	var err error
	*exists, err = ts.db.PackageExists("OMNI_TRACER_API")
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

	if err := ts.db.DeployFile(string(omniTracerSQLPackage)); err != nil {
		return fmt.Errorf("failed to deploy Omni tracer package: %w", err)
	}

	return nil
}

// InitializeTracerPackage initializes the Omni tracer package in the database
func initializeTracerPackage(ts *TracerService) error {
	omniInitInsFile, err := assets.GetInsFile("Omni_Initialize.ins")
	if err != nil {
		return fmt.Errorf("failed to read Omni initialize file: %w", err)
	}

	if err := ts.db.ExecuteStatement(string(omniInitInsFile)); err != nil {
		return fmt.Errorf("failed to deploy Omni initialize file: %w", err)
	}

	return nil
}
