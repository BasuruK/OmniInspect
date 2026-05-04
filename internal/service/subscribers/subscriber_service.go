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
	procGen *ProcedureGenerator
}

// Constructor: NewSubscriberService Constructor for SubscriberService
func NewSubscriberService(db ports.DatabaseRepository, subRepo ports.SubscriberRepository, procGen *ProcedureGenerator) *SubscriberService {
	return &SubscriberService{
		db:      db,
		subRepo: subRepo,
		procGen: procGen,
	}
}

// SetSubscriber stores the subscriber in the bolt database
func (ss *SubscriberService) SetSubscriber(ctx context.Context, subscriber *domain.Subscriber) error {
	if subscriber == nil {
		return errors.New("subscriber cannot be nil")
	}
	return ss.subRepo.Save(ctx, *subscriber)
}

// GetSubscriber retrieves the subscriber from the bolt database
func (ss *SubscriberService) GetSubscriber(ctx context.Context) (*domain.Subscriber, error) {
	subs, err := ss.subRepo.List(ctx)
	if err != nil {
		return nil, err
	}
	if len(subs) == 0 {
		return nil, domain.ErrSubscriberNotFound
	}
	if len(subs) > 1 {
		return nil, fmt.Errorf("expected 1 subscriber, found %d", len(subs))
	}
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
		subscriber, err = domain.NewRandomSubscriber()
		if err != nil {
			return nil, err
		}
	}

	reservedFunnyName := ""
	reservedNewFunnyName := false
	if ss.procGen != nil {
		reservedFunnyName, reservedNewFunnyName, err = ss.procGen.ReserveFunnyName(ctx, subscriber)
		if err != nil {
			return nil, fmt.Errorf("failed to reserve funny name: %w", err)
		}
		if subscriber.FunnyName() == "" {
			if err := subscriber.AssignFunnyName(reservedFunnyName); err != nil {
				if reservedNewFunnyName {
					ss.procGen.ReleaseFunnyName(ctx, reservedFunnyName)
				}
				return nil, fmt.Errorf("failed to assign funny name: %w", err)
			}
		}
	}

	// Register Subscriber in Oracle DB
	if err := ss.db.RegisterNewSubscriber(ctx, *subscriber); err != nil {
		if reservedNewFunnyName {
			ss.procGen.ReleaseFunnyName(ctx, reservedFunnyName)
		}
		return nil, err
	}

	if ss.procGen != nil {
		if err := ss.procGen.GenerateSubscriberProcedure(ctx, subscriber); err != nil {
			if reservedNewFunnyName {
				ss.procGen.ReleaseFunnyName(ctx, reservedFunnyName)
			}
			return nil, fmt.Errorf("failed to generate subscriber procedure: %w", err)
		}
	}

	if err := ss.SetSubscriber(ctx, subscriber); err != nil {
		return nil, fmt.Errorf("failed to save subscriber: %w", err)
	}

	return subscriber, nil
}
