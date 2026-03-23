package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/example/em_jgo/internal/domain/subscription"
)

type repositoryStub struct {
	createFn             func(ctx context.Context, item subscription.Subscription) (subscription.Subscription, error)
	getByIDFn            func(ctx context.Context, id uuid.UUID) (subscription.Subscription, error)
	updateFn             func(ctx context.Context, item subscription.Subscription) (subscription.Subscription, error)
	deleteFn             func(ctx context.Context, id uuid.UUID) error
	listFn               func(ctx context.Context, filter subscription.ListFilter) ([]subscription.Subscription, int64, error)
	calculateTotalCostFn func(ctx context.Context, filter subscription.TotalCostFilter) (int, error)
}

func (r repositoryStub) Create(ctx context.Context, item subscription.Subscription) (subscription.Subscription, error) {
	return r.createFn(ctx, item)
}

func (r repositoryStub) GetByID(ctx context.Context, id uuid.UUID) (subscription.Subscription, error) {
	return r.getByIDFn(ctx, id)
}

func (r repositoryStub) Update(ctx context.Context, item subscription.Subscription) (subscription.Subscription, error) {
	return r.updateFn(ctx, item)
}

func (r repositoryStub) Delete(ctx context.Context, id uuid.UUID) error {
	return r.deleteFn(ctx, id)
}

func (r repositoryStub) List(ctx context.Context, filter subscription.ListFilter) ([]subscription.Subscription, int64, error) {
	return r.listFn(ctx, filter)
}

func (r repositoryStub) CalculateTotalCost(ctx context.Context, filter subscription.TotalCostFilter) (int, error) {
	return r.calculateTotalCostFn(ctx, filter)
}

func TestServiceCreateValidatesInput(t *testing.T) {
	t.Parallel()

	svc := New(repositoryStub{})
	_, err := svc.Create(context.Background(), subscription.Subscription{})
	require.Error(t, err)
	require.EqualError(t, err, "service name is required")
}

func TestServiceCreateDelegatesToRepository(t *testing.T) {
	t.Parallel()

	item := validSubscription()
	svc := New(repositoryStub{createFn: func(ctx context.Context, got subscription.Subscription) (subscription.Subscription, error) {
		require.Equal(t, item, got)
		return got, nil
	}})

	created, err := svc.Create(context.Background(), item)
	require.NoError(t, err)
	require.Equal(t, item, created)
}

func TestServiceUpdateReturnsNotFound(t *testing.T) {
	t.Parallel()

	item := validSubscription()
	svc := New(repositoryStub{updateFn: func(ctx context.Context, got subscription.Subscription) (subscription.Subscription, error) {
		return subscription.Subscription{}, ErrNotFound
	}})

	_, err := svc.Update(context.Background(), item)
	require.ErrorIs(t, err, ErrNotFound)
}

func TestServiceListSetsDefaults(t *testing.T) {
	t.Parallel()

	svc := New(repositoryStub{listFn: func(ctx context.Context, filter subscription.ListFilter) ([]subscription.Subscription, int64, error) {
		require.Equal(t, 20, filter.Limit)
		require.Equal(t, 0, filter.Offset)
		return []subscription.Subscription{validSubscription()}, 1, nil
	}})

	items, total, err := svc.List(context.Background(), subscription.ListFilter{Limit: 0, Offset: -1})
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.EqualValues(t, 1, total)
}

func TestServiceCalculateTotalCostValidatesPeriod(t *testing.T) {
	t.Parallel()

	svc := New(repositoryStub{})
	_, err := svc.CalculateTotalCost(context.Background(), subscription.TotalCostFilter{StartPeriod: time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC), EndPeriod: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)})
	require.ErrorIs(t, err, subscription.ErrInvalidDates)
}

func TestServiceCalculateTotalCostDelegatesToRepository(t *testing.T) {
	t.Parallel()

	filter := subscription.TotalCostFilter{StartPeriod: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), EndPeriod: time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)}
	svc := New(repositoryStub{calculateTotalCostFn: func(ctx context.Context, got subscription.TotalCostFilter) (int, error) {
		require.Equal(t, filter, got)
		return 1200, nil
	}})

	total, err := svc.CalculateTotalCost(context.Background(), filter)
	require.NoError(t, err)
	require.Equal(t, 1200, total)
}

func TestServiceDeleteWrapsUnexpectedError(t *testing.T) {
	t.Parallel()

	targetErr := errors.New("db unavailable")
	svc := New(repositoryStub{deleteFn: func(ctx context.Context, id uuid.UUID) error {
		return targetErr
	}})

	err := svc.Delete(context.Background(), uuid.New())
	require.Error(t, err)
	require.ErrorContains(t, err, "delete subscription")
	require.ErrorIs(t, err, targetErr)
}

func validSubscription() subscription.Subscription {
	return subscription.Subscription{
		ID:          uuid.New(),
		ServiceName: "Yandex Plus",
		Price:       400,
		UserID:      uuid.New(),
		StartDate:   time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	}
}
