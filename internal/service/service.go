package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"github.com/example/em_jgo/internal/domain/subscription"
)

var ErrNotFound = errors.New("subscription not found")

type Repository interface {
	Create(ctx context.Context, item subscription.Subscription) (subscription.Subscription, error)
	GetByID(ctx context.Context, id uuid.UUID) (subscription.Subscription, error)
	Update(ctx context.Context, item subscription.Subscription) (subscription.Subscription, error)
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, filter subscription.ListFilter) ([]subscription.Subscription, int64, error)
	CalculateTotalCost(ctx context.Context, filter subscription.TotalCostFilter) (int, error)
}

type Service struct {
	repo   Repository
	logger *slog.Logger
}

func New(repo Repository, logger *slog.Logger) *Service {
	return &Service{repo: repo, logger: logger}
}

func (s *Service) Create(ctx context.Context, item subscription.Subscription) (subscription.Subscription, error) {
	const methodCtx = "service.Create"
	log := s.logger.With("method", methodCtx)

	if err := item.Validate(); err != nil {
		log.WarnContext(ctx, "validation failed", "error", err)
		return subscription.Subscription{}, err
	}

	created, err := s.repo.Create(ctx, item)
	if err != nil {
		log.ErrorContext(ctx, "create subscription failed", "error", err)
		return subscription.Subscription{}, fmt.Errorf("create subscription: %w", err)
	}

	log.InfoContext(ctx, "subscription created", "subscription_id", created.ID)
	return created, nil
}

func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (subscription.Subscription, error) {
	const methodCtx = "service.GetByID"
	log := s.logger.With("method", methodCtx)

	item, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			log.WarnContext(ctx, "subscription not found", "subscription_id", id)
			return subscription.Subscription{}, err
		}
		log.ErrorContext(ctx, "get subscription failed", "error", err, "subscription_id", id)
		return subscription.Subscription{}, fmt.Errorf("get subscription: %w", err)
	}

	return item, nil
}

func (s *Service) Update(ctx context.Context, item subscription.Subscription) (subscription.Subscription, error) {
	const methodCtx = "service.Update"
	log := s.logger.With("method", methodCtx)

	if item.ID == uuid.Nil {
		return subscription.Subscription{}, errors.New("subscription id is required")
	}
	if err := item.Validate(); err != nil {
		log.WarnContext(ctx, "validation failed", "error", err, "subscription_id", item.ID)
		return subscription.Subscription{}, err
	}

	updated, err := s.repo.Update(ctx, item)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			log.WarnContext(ctx, "subscription not found", "subscription_id", item.ID)
			return subscription.Subscription{}, err
		}
		log.ErrorContext(ctx, "update subscription failed", "error", err, "subscription_id", item.ID)
		return subscription.Subscription{}, fmt.Errorf("update subscription: %w", err)
	}

	log.InfoContext(ctx, "subscription updated", "subscription_id", updated.ID)
	return updated, nil
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	const methodCtx = "service.Delete"
	log := s.logger.With("method", methodCtx)

	if err := s.repo.Delete(ctx, id); err != nil {
		if errors.Is(err, ErrNotFound) {
			log.WarnContext(ctx, "subscription not found", "subscription_id", id)
			return err
		}
		log.ErrorContext(ctx, "delete subscription failed", "error", err, "subscription_id", id)
		return fmt.Errorf("delete subscription: %w", err)
	}

	log.InfoContext(ctx, "subscription deleted", "subscription_id", id)
	return nil
}

func (s *Service) List(ctx context.Context, filter subscription.ListFilter) ([]subscription.Subscription, int64, error) {
	const methodCtx = "service.List"
	log := s.logger.With("method", methodCtx)

	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}

	items, total, err := s.repo.List(ctx, filter)
	if err != nil {
		log.ErrorContext(ctx, "list subscriptions failed", "error", err)
		return nil, 0, fmt.Errorf("list subscriptions: %w", err)
	}

	log.InfoContext(ctx, "subscriptions listed", "count", len(items), "total", total)
	return items, total, nil
}

func (s *Service) CalculateTotalCost(ctx context.Context, filter subscription.TotalCostFilter) (int, error) {
	const methodCtx = "service.CalculateTotalCost"
	log := s.logger.With("method", methodCtx)

	if filter.StartPeriod.IsZero() || filter.EndPeriod.IsZero() {
		return 0, errors.New("start and end period are required")
	}
	if filter.EndPeriod.Before(filter.StartPeriod) {
		return 0, subscription.ErrInvalidDates
	}

	total, err := s.repo.CalculateTotalCost(ctx, filter)
	if err != nil {
		log.ErrorContext(ctx, "calculate total cost failed", "error", err)
		return 0, fmt.Errorf("calculate total cost: %w", err)
	}

	log.InfoContext(ctx, "total cost calculated", "total_cost", total)
	return total, nil
}
