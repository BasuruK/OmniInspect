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
func NewTracerService(db ports.DatabaseRepository, bolt ports.ConfigRepository) (*TracerService, error) {
	rawConn := db.GetRawConnection()
	rawCtx := db.GetRawContext()
	if rawConn == nil || rawCtx == nil {
		return nil, fmt.Errorf("database connection or context is nil during TracerService initialization")
	}

	// Note: NewSubscriptionManager expects (connection, context) order
	subscriptionMgr := subscription.NewSubscriptionManager(rawConn, rawCtx)

	return &TracerService{
		db:              db,
		bolt:            bolt,
		subscriptionMgr: subscriptionMgr,
	}, nil
}

func (ts *TracerService) StartEventListener(ctx context.Context, subscriber *domain.Subscriber, schema string) error {
	/* Create a notification channel
	The channel will receive notifications from the database when new messages arrive
	Buffer size determines how many notifications can be queued before blocking
	i.e: make(chan struct{}, 100), means it can hold 100 notifications
	*/
	notifyChan := make(chan struct{}, 100) // Buffered channel to handle notification bursts

	// Subscribe to the queue
	if err := ts.subscriptionMgr.Subscribe(*subscriber, schema, notifyChan); err != nil {
		return fmt.Errorf("failed to subscribe to queue: %w", err)
	}

	fmt.Println("[OCI] Subscription Success for subscriber:", subscriber)

	// Initial processing to handle any existing messages
	// any remaining messages for the subscriber that was sent before starting the listener will be processed here
	go func() {
		ts.processBatch(subscriber)
	}()

	// Start the goroutine to listen for notifications
	go ts.eventLoop(ctx, notifyChan, subscriber)

	return nil
}

func (ts *TracerService) eventLoop(ctx context.Context, notifyChan chan struct{}, subscriber *domain.Subscriber) {
	ticker := time.NewTicker(5 * time.Second) // Periodic check interval, fallback polling. TODO: Take this
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			fmt.Println("Event listener stopped.")
			ts.cleanUp(subscriber, notifyChan)
			return
		case <-notifyChan:
			// Process the notification
			ts.processBatch(subscriber)
		case <-ticker.C:
			// Periodic check (fallback polling)
			queueDepth := ts.checkQueueDepth(subscriber)
			if queueDepth > 0 {
				ts.processBatch(subscriber)
			}
		}
	}
}

// cleanUp handles cleanup operations when stopping the event listener
func (ts *TracerService) cleanUp(subscriber *domain.Subscriber, notifyChan chan struct{}) {
	if ts.subscriptionMgr != nil {
		if err := ts.subscriptionMgr.Unsubscribe(*subscriber); err != nil {
			fmt.Printf("failed to unsubscribe: %v\n", err)
		} else {
			fmt.Println("Unsubscribed successfully for subscriber:", subscriber.Name)
		}
	}
	close(notifyChan)
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

	for i := 0; i < count; i++ {
		var msg domain.QueueMessage
		if err := json.Unmarshal([]byte(messages[i]), &msg); err != nil {
			log.Printf("failed to unmarshal message ID %s: %v", msgIDs[i], err)
			continue
		}

		ts.handleTracerMessage(msg)
	}
}

func (ts *TracerService) handleTracerMessage(msg domain.QueueMessage) {
	fmt.Printf("[%s] [%s] %s: %s \n", msg.Timestamp, msg.LogLevel, msg.ProcessName, msg.Payload)
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
