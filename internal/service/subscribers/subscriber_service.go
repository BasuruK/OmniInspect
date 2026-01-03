package subscribers

import (
	"OmniView/internal/core/domain"
	"OmniView/internal/core/ports"

	"github.com/google/uuid"
)

// Service: Manages subscriber information
// Injects a ConfigRepository to interact with the bolt database
type SubscriberService struct {
	bolt ports.ConfigRepository
}

// Constructor: NewSubscriberService Constructor for SubscriberService
func NewSubscriberService(bolt ports.ConfigRepository) *SubscriberService {
	return &SubscriberService{
		bolt: bolt,
	}
}

// GetSubscriberName Retrieves the subscriber name from the bolt database
func (ss *SubscriberService) SetSubscriberName(subscriber domain.Subscriber) error {
	return ss.bolt.SetSubscriberName(subscriber)
}

// GetSubscriberName Retrieves the subscriber name from the bolt database
func (ss *SubscriberService) GetSubscriberName() (*domain.Subscriber, error) {
	return ss.bolt.GetSubscriberName()
}

// NewSubscriber Generates and stores a new unique subscriber name
func (ss *SubscriberService) NewSubscriber() (string, error) {
	subscriberName := generateSubscriberName()
	if err := ss.SetSubscriberName(domain.Subscriber{Name: subscriberName}); err != nil {
		return "", err
	}
	return subscriberName, nil
}

// RegisterSubscriber Retrieves existing subscriber or creates a new one if not found
func (ss *SubscriberService) RegisterSubscriber() (domain.Subscriber, error) {
	subscriber, err := ss.GetSubscriberName()
	if err != nil {
		// If not found, create a new subscriber
		newName, err := ss.NewSubscriber()
		if err != nil {
			return domain.Subscriber{}, err
		}
		return domain.Subscriber{Name: newName}, nil
	}
	return *subscriber, nil
}

// generateSubscriberName Generates a new unique subscriber name
func generateSubscriberName() string {
	// UUID V4 Generation
	uuidWithHyphen := uuid.New()
	return uuidWithHyphen.String()
}
