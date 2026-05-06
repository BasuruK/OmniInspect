package domain

import (
	"encoding/json"
	"testing"
	"time"
)

func TestSubscriberJSONRoundTripPreservesFunnyName(t *testing.T) {
	subscriber, err := NewSubscriberWithFunnyName("TEST_SUB", "BARNACLE", DefaultBatchSize, DefaultWaitTime)
	if err != nil {
		t.Fatalf("NewSubscriberWithFunnyName() returned error: %v", err)
	}
	subscriber.createdAt = time.Unix(1710000000, 0)
	subscriber.active = false

	encoded, err := json.Marshal(subscriber)
	if err != nil {
		t.Fatalf("json.Marshal() returned error: %v", err)
	}

	var decoded Subscriber
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() returned error: %v", err)
	}

	if decoded.Name() != subscriber.Name() {
		t.Fatalf("decoded name = %q, want %q", decoded.Name(), subscriber.Name())
	}
	if decoded.FunnyName() != subscriber.FunnyName() {
		t.Fatalf("decoded funny name = %q, want %q", decoded.FunnyName(), subscriber.FunnyName())
	}
	if decoded.ConsumerName() != subscriber.FunnyName() {
		t.Fatalf("decoded consumer name = %q, want %q", decoded.ConsumerName(), subscriber.FunnyName())
	}
	if !decoded.CreatedAt().Equal(subscriber.CreatedAt()) {
		t.Fatalf("decoded createdAt = %v, want %v", decoded.CreatedAt(), subscriber.CreatedAt())
	}
	if decoded.IsActive() != subscriber.IsActive() {
		t.Fatalf("decoded active = %v, want %v", decoded.IsActive(), subscriber.IsActive())
	}
}
