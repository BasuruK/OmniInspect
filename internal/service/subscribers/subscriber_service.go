package subscribers

import (
	"OmniView/internal/core/domain"
	"OmniView/internal/core/ports"
	"context"
	"errors"
	"fmt"
)

// Service: Manages subscriber information
// Uses dedicated SubscriberRepository for persistence
type SubscriberService struct {
	db      ports.DatabaseRepository
	subRepo ports.SubscriberRepository
}

// Constructor: NewSubscriberService Constructor for SubscriberService
func NewSubscriberService(db ports.DatabaseRepository, subRepo ports.SubscriberRepository) *SubscriberService {
	return &SubscriberService{
		db:      db,
		subRepo: subRepo,
	}
}

// SetSubscriber stores the subscriber in the bolt database
func (ss *SubscriberService) SetSubscriber(ctx context.Context, subscriber *domain.Subscriber) error {
	if subscriber == nil {
		return errors.New("subscriber cannot be nil")
	}
	fmt.Printf("[DEBUG] Saving subscriber: %s\n", subscriber.Name())
	if err := ss.subRepo.Save(ctx, *subscriber); err != nil {
		return fmt.Errorf("failed to save subscriber: %w", err)
	}
	fmt.Printf("[DEBUG] Subscriber saved successfully\n")
	return nil
}

// GetSubscriber retrieves the subscriber from the bolt database
func (ss *SubscriberService) GetSubscriber(ctx context.Context) (*domain.Subscriber, error) {
	subs, err := ss.subRepo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list subscribers: %w", err)
	}
	fmt.Printf("[DEBUG] Found %d subscribers\n", len(subs))
	if len(subs) == 0 {
		return nil, domain.ErrSubscriberNotFound
	}
	fmt.Printf("[DEBUG] Returning subscriber: %s\n", subs[0].Name())
	return &subs[0], nil
}

// NewSubscriber Generates and stores a new unique subscriber name
func (ss *SubscriberService) NewSubscriber(ctx context.Context) (*domain.Subscriber, error) {
	// Use domain factory to create a new subscriber with random name
	subscriber, err := domain.NewRandomSubscriber()
	if err != nil {
		return nil, err
	}

	if err := ss.SetSubscriber(ctx, subscriber); err != nil {
		return nil, err
	}

	return subscriber, nil
}

// RegisterSubscriber Retrieves existing subscriber or creates a new one if not found
// Registers the subscriber as a listener in the oracle database.
func (ss *SubscriberService) RegisterSubscriber(ctx context.Context) (*domain.Subscriber, error) {
	subscriber, err := ss.GetSubscriber(ctx)
	if err != nil {
		if !errors.Is(err, domain.ErrSubscriberNotFound) {
			return nil, err // return other errors
		}
		// If not found, create a new subscriber
		subscriber, err = ss.NewSubscriber(ctx)
		if err != nil {
			return nil, err
		}
	}
	// Register Subscriber in Oracle DB
	if err := ss.db.RegisterNewSubscriber(*subscriber); err != nil {
		return nil, err
	}

	return subscriber, nil
}
