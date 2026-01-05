package tracer

import (
	"OmniView/assets"
	"OmniView/internal/adapter/subscription"
	"OmniView/internal/core/ports"
	"context"
	"fmt"
	"log"
	"time"
	"unsafe"
)

// Service: Manages package deployments
// Injects a DatabaseRepository and ConfigRepository to interact with the database
type TracerService struct {
	db              ports.DatabaseRepository
	bolt            ports.ConfigRepository
	subscriptionMgr *subscription.SubscriptionManager
}

// Constructor: NewTracerService Constructor for TracerService
func NewTracerService(db ports.DatabaseRepository, bolt ports.ConfigRepository) *TracerService {
	return &TracerService{
		db:   db,
		bolt: bolt,
	}
}

func (ts *TracerService) StartEventListener(ctx context.Context, subscriberName string) error {
	// Get raw ODPI-C handles from the database repository
	// TODO: see if we can avoid this type assertion by defining a more specific interface, or directly getting it from Ports.DatabaseRepository
	oracleAdapter, ok := ts.db.(interface {
		GetRawConnection() unsafe.Pointer
		GetRawContext() unsafe.Pointer
	})
	if !ok {
		return fmt.Errorf("database repository does not expose raw ODPI-C handles [Context and Connection not Exposed]")
	}

	// Create the subscription manager using the raw handles (tracer-specific adapter)
	// Note: NewSubscriptionManager expects (context, connection) order
	ts.subscriptionMgr = subscription.NewSubscriptionManager(
		oracleAdapter.GetRawConnection(),
		oracleAdapter.GetRawContext(),
	)

	// Create a notification channel
	notifyChan := make(chan struct{}, 10) // Buffered channel to avoid blocking and handle bursts

	// Subscribe to the queue
	subscriberID, err := ts.subscriptionMgr.Subscribe(subscriberName, notifyChan)
	if err != nil {
		return fmt.Errorf("failed to subscribe to queue: %w", err)
	}

	fmt.Println("[OCI] Subscription Success")

	// Start the goroutine to listen for notifications
	go ts.eventLoop(ctx, notifyChan, subscriberID)

	return nil
}

func (ts *TracerService) eventLoop(ctx context.Context, notifyChan <-chan struct{}, subscriberID string) {
	ticker := time.NewTicker(5 * time.Second) // Periodic check interval, fallback polling
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			fmt.Println("Event listener stopped.")
			ts.cleanUp(subscriberID)
			return
		case <-notifyChan:
			// Process the notification
			fmt.Println("Received notification for subscriber:", subscriberID)
			ts.processBatch(subscriberID)
		case <-ticker.C:
			// Periodic check (fallback polling)
			queueDepth := ts.checkQueueDepth(subscriberID)
			if queueDepth > 0 {
				fmt.Printf("Periodic check: Processing %d messages for subscriber: %s\n", queueDepth, subscriberID)
				ts.processBatch(subscriberID)
			}
		}
	}
}

// cleanUp handles cleanup operations when stopping the event listener
func (ts *TracerService) cleanUp(subscriberID string) {
	if ts.subscriptionMgr != nil {
		if err := ts.subscriptionMgr.Unsubscribe(subscriberID); err != nil {
			fmt.Printf("failed to unsubscribe: %v", err)
		} else {
			fmt.Println("Unsubscribed successfully for subscriber:", subscriberID)
		}
	}
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

// processBatch processes a batch of tracer data for the given subscriber ID
// TODO: implement actual processing logic
func (ts *TracerService) processBatch(subscriberID string) {
	log.Println("Processing tracer batch...")
}

// checkQueueDepth checks the queue depth for the given subscriber ID
// TODO: implement actual queue depth checking logic
func (ts *TracerService) checkQueueDepth(subscriberID string) int {
	// Placeholder implementation
	return 0
}
