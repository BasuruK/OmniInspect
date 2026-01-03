package subscribers

import (
	"OmniView/internal/core/domain"
	"OmniView/internal/core/ports"
	"strings"

	"github.com/google/uuid"
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

// SetSubscriberName Retrieves the subscriber name from the bolt database
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
// Registers the subscriber as a listener in the oracle database.
func (ss *SubscriberService) RegisterSubscriber() (domain.Subscriber, error) {
	subscriber, err := ss.GetSubscriberName()
	if err != nil {
		if err.Error() != "subscriber name not found" {
			return domain.Subscriber{}, err // return other errors
		}
		// If not found, create a new subscriber
		newName, err := ss.NewSubscriber()
		if err != nil {
			return domain.Subscriber{}, err
		}
		subscriber = &domain.Subscriber{Name: newName}
	}
	// Register Subscriber in Oracle DB
	if err := ss.db.RegisterNewSubscriber(*subscriber); err != nil {
		return domain.Subscriber{}, err
	}

	return *subscriber, nil
}

// generateSubscriberName Generates a new unique subscriber name
func generateSubscriberName() string {
	// UUID V4 Generation
	uuidWithHyphen := uuid.New()
	// Format the UUID as a named subscriber identifier
	// Replace - with _ to comply with Oracle naming conventions
	// Add a prefix for clarity : SUB_
	subscriberName := "SUB_" + strings.ReplaceAll(uuidWithHyphen.String(), "-", "_")

	return subscriberName
}
