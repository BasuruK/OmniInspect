package subscribers

import (
	"OmniView/internal/core/domain"
	"context"
	"errors"
	"testing"
)

type LoadSoleStubRepo struct {
	list    []domain.Subscriber
	listErr error
}

func (r *LoadSoleStubRepo) Save(context.Context, domain.Subscriber) error { return nil }
func (r *LoadSoleStubRepo) GetByName(context.Context, string) (*domain.Subscriber, error) {
	return nil, domain.ErrSubscriberNotFound
}
func (r *LoadSoleStubRepo) List(context.Context) ([]domain.Subscriber, error) {
	if r.listErr != nil {
		return nil, r.listErr
	}
	return append([]domain.Subscriber(nil), r.list...), nil
}
func (r *LoadSoleStubRepo) Exists(context.Context, string) (bool, error) { return false, nil }
func (r *LoadSoleStubRepo) Delete(context.Context, string) error         { return nil }

func TestLoadSoleSubscriber(t *testing.T) {
	ctx := context.Background()
	sub, err := domain.NewSubscriberWithDefaults("SUB_A")
	if err != nil {
		t.Fatalf("NewSubscriberWithDefaults: %v", err)
	}
	other, err := domain.NewSubscriberWithDefaults("SUB_B")
	if err != nil {
		t.Fatalf("NewSubscriberWithDefaults: %v", err)
	}

	t.Run("happy_path", func(t *testing.T) {
		got, err := LoadSoleSubscriber(ctx, &LoadSoleStubRepo{list: []domain.Subscriber{*sub}})
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if got.Name() != sub.Name() {
			t.Fatalf("name = %q, want %q", got.Name(), sub.Name())
		}
	})

	t.Run("not_found", func(t *testing.T) {
		_, err := LoadSoleSubscriber(ctx, &LoadSoleStubRepo{})
		if !errors.Is(err, domain.ErrSubscriberNotFound) {
			t.Fatalf("err = %v, want ErrSubscriberNotFound", err)
		}
	})

	t.Run("multiple", func(t *testing.T) {
		_, err := LoadSoleSubscriber(ctx, &LoadSoleStubRepo{list: []domain.Subscriber{*sub, *other}})
		if !errors.Is(err, domain.ErrMultipleSubscribers) {
			t.Fatalf("err = %v, want ErrMultipleSubscribers", err)
		}
	})

	t.Run("list_error_wrapped", func(t *testing.T) {
		boom := errors.New("bolt down")
		_, err := LoadSoleSubscriber(ctx, &LoadSoleStubRepo{listErr: boom})
		if !errors.Is(err, boom) {
			t.Fatalf("err = %v, want wrapped boom", err)
		}
		if err == nil || err.Error()[:len("list subscribers:")] != "list subscribers:" {
			t.Fatalf("err = %v, want list subscribers: prefix", err)
		}
	})
}
