package subscribers

import (
	"OmniView/internal/core/domain"
	"OmniView/internal/core/ports"
	"errors"
)

// Service: Manages subscriber information
// Injects a ConfigRepository to interact with the bolt database
type SubscriberService struct {
	db   ports.DatabaseRepository
	bolt ports.ConfigRepository
}

// Constructor: NewSubscriberService Constructor for SubscriberService
func NewSubscriberService(db ports.DatabaseRepository, bolt ports.ConfigRepository) *SubscriberService {
	return &SubscriberService{
		db:   db,
		bolt: bolt,
	}
}

// SetSubscriber stores the subscriber in the bolt database
func (ss *SubscriberService) SetSubscriber(subscriber *domain.Subscriber) error {
	return ss.bolt.SetSubscriber(*subscriber)
}

// GetSubscriber retrieves the subscriber from the bolt database
func (ss *SubscriberService) GetSubscriber() (*domain.Subscriber, error) {
	return ss.bolt.GetSubscriber()
}

// NewSubscriber Generates and stores a new unique subscriber name
func (ss *SubscriberService) NewSubscriber() (*domain.Subscriber, error) {
	// Use domain factory to create a new subscriber with random name
	subscriber, err := domain.NewRandomSubscriber()
	if err != nil {
		return nil, err
	}

	if err := ss.SetSubscriber(subscriber); err != nil {
		return nil, err
	}

	return subscriber, nil
}

// RegisterSubscriber Retrieves existing subscriber or creates a new one if not found
// Registers the subscriber as a listener in the oracle database.
func (ss *SubscriberService) RegisterSubscriber() (*domain.Subscriber, error) {
	subscriber, err := ss.GetSubscriber()
	if err != nil {
		if !errors.Is(err, domain.ErrSubscriberNotFound) {
			return nil, err // return other errors
		}
		// If not found, create a new subscriber
		subscriber, err = ss.NewSubscriber()
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
