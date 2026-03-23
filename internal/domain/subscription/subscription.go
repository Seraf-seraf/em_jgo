package subscription

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrInvalidDates        = errors.New("end date must not be before start date")
	ErrServiceNameRequired = errors.New("service name is required")
	ErrPriceRequired       = errors.New("price must be greater than zero")
	ErrUserIDRequired      = errors.New("user id is required")
	ErrStartDateRequired   = errors.New("start date is required")
)

type Subscription struct {
	ID          uuid.UUID
	ServiceName string
	Price       int
	UserID      uuid.UUID
	StartDate   time.Time
	EndDate     *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type ListFilter struct {
	UserID      *uuid.UUID
	ServiceName *string
	Limit       int
	Offset      int
}

type TotalCostFilter struct {
	StartPeriod time.Time
	EndPeriod   time.Time
	UserID      *uuid.UUID
	ServiceName *string
}

func (s Subscription) Validate() error {
	if strings.TrimSpace(s.ServiceName) == "" {
		return ErrServiceNameRequired
	}
	if s.Price < 1 {
		return ErrPriceRequired
	}
	if s.UserID == uuid.Nil {
		return ErrUserIDRequired
	}
	if s.StartDate.IsZero() {
		return ErrStartDateRequired
	}
	if s.EndDate != nil && s.EndDate.Before(s.StartDate) {
		return ErrInvalidDates
	}
	return nil
}
