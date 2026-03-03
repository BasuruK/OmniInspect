package tracer

import (
	"OmniView/assets"
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
	fmt.Println("[Tracer] Starting event listener for subscriber:", subscriber.Name())

	// Initial processing to handle any existing messages
	// any remaining messages for the subscriber that was sent before starting the listener will be processed here
	go func() {
		ts.processBatch(subscriber)
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
		err := ts.processBatch(subscriber)
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
func (ts *TracerService) processBatch(subscriber *domain.Subscriber) error {
	// Lock for processing to avoid concurrent dequeues
	ts.processMu.Lock()
	defer ts.processMu.Unlock()

	messages, msgIDs, count, err := ts.db.BulkDequeueTracerMessages(*subscriber)
	if err != nil {
		return err
	}

	if count == 0 {
		return nil // return
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
