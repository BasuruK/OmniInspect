package oracle

import (
	"OmniView/internal/core/domain"
	"context"
	"fmt"
	"strconv"
)

// RegisterNewSubscriber registers a new subscriber in the Oracle database.
// If subscriber already exists, it returns nil.
func (oa *OracleAdapter) RegisterNewSubscriber(ctx context.Context, subscriber domain.Subscriber) error {
	exists, err := subscriberExists(ctx, oa, subscriber)
	if err != nil {
		return fmt.Errorf("failed to check subscriber existence: %v", err)
	}
	if !exists {
		// Subscriber does not exist, register it
		err := oa.ExecuteWithParams(ctx, "BEGIN OMNI_TRACER_API.Register_Subscriber(:subscriberName); END;", map[string]interface{}{
			"subscriberName": subscriber.ConsumerName(),
		})
		if err != nil {
			return fmt.Errorf("failed to register subscriber: %v", err)
		}
		return nil
	}
	return nil
}

// subscriberExists checks if a subscriber with the given name already exists in the Oracle database.
func subscriberExists(ctx context.Context, oa *OracleAdapter, subscriber domain.Subscriber) (bool, error) {
	query := `SELECT COUNT(1)
			FROM ALL_QUEUE_SUBSCRIBERS
			WHERE QUEUE_NAME = :queueName
			AND CONSUMER_NAME = :subscriberName
			AND OWNER = :queueOwner`
	results, err := oa.FetchWithParams(ctx, query, map[string]interface{}{
		"queueName":      domain.QueueName,
		"subscriberName": subscriber.ConsumerName(),
		"queueOwner":     oa.config.Username(),
	})
	if err != nil {
		return false, fmt.Errorf("failed to query subscriber existence: %w", err)
	}
	if len(results) == 0 {
		return false, nil
	}

	count, err := parseCountResult(results)
	if err != nil {
		return false, fmt.Errorf("failed to parse subscriber existence count: %v", err)
	}
	return count > 0, nil
}

// parseCountResult parses the first element of a COUNT(*) query result.
func parseCountResult(results []string) (int, error) {
	if len(results) == 0 || results[0] == "" {
		return 0, nil
	}
	count, err := strconv.Atoi(results[0])
	if err != nil {
		return 0, fmt.Errorf("failed to parse count result: %v", err)
	}
	return count, nil
}
