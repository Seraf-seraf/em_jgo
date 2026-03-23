package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/example/em_jgo/internal/domain/subscription"
)

var (
	ErrNotFound               = errors.New("subscription not found")
	ErrSubscriptionIDRequired = errors.New("subscription id is required")
	ErrPeriodsRequired        = errors.New("start and end period are required")
)

type Repository interface {
	Create(ctx context.Context, item subscription.Subscription) (subscription.Subscription, error)
	GetByID(ctx context.Context, id uuid.UUID) (subscription.Subscription, error)
	Update(ctx context.Context, item subscription.Subscription) (subscription.Subscription, error)
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, filter subscription.ListFilter) ([]subscription.Subscription, int64, error)
	CalculateTotalCost(ctx context.Context, filter subscription.TotalCostFilter) (int, error)
}

type Service struct {
	repo Repository
}

func New(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Create(ctx context.Context, item subscription.Subscription) (subscription.Subscription, error) {
	if err := item.Validate(); err != nil {
		return subscription.Subscription{}, err
	}

	created, err := s.repo.Create(ctx, item)
	if err != nil {
		return subscription.Subscription{}, fmt.Errorf("create subscription: %w", err)
	}

	return created, nil
}

func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (subscription.Subscription, error) {
	item, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return subscription.Subscription{}, err
		}
		return subscription.Subscription{}, fmt.Errorf("get subscription: %w", err)
	}

	return item, nil
}

func (s *Service) Update(ctx context.Context, item subscription.Subscription) (subscription.Subscription, error) {
	if item.ID == uuid.Nil {
		return subscription.Subscription{}, ErrSubscriptionIDRequired
	}
	if err := item.Validate(); err != nil {
		return subscription.Subscription{}, err
	}

	updated, err := s.repo.Update(ctx, item)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return subscription.Subscription{}, err
		}
		return subscription.Subscription{}, fmt.Errorf("update subscription: %w", err)
	}

	return updated, nil
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		if errors.Is(err, ErrNotFound) {
			return err
		}
		return fmt.Errorf("delete subscription: %w", err)
	}
	return nil
}

func (s *Service) List(ctx context.Context, filter subscription.ListFilter) ([]subscription.Subscription, int64, error) {
	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}

	items, total, err := s.repo.List(ctx, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("list subscriptions: %w", err)
	}

	return items, total, nil
}

func (s *Service) CalculateTotalCost(ctx context.Context, filter subscription.TotalCostFilter) (int, error) {
	if filter.StartPeriod.IsZero() || filter.EndPeriod.IsZero() {
		return 0, ErrPeriodsRequired
	}
	if filter.EndPeriod.Before(filter.StartPeriod) {
		return 0, subscription.ErrInvalidDates
	}

	total, err := s.repo.CalculateTotalCost(ctx, filter)
	if err != nil {
		return 0, fmt.Errorf("calculate total cost: %w", err)
	}

	return total, nil
}
