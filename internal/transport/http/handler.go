package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/example/em_jgo/internal/domain/subscription"
	"github.com/example/em_jgo/internal/month"
	"github.com/example/em_jgo/internal/service"
)

type SubscriptionService interface {
	Create(ctx context.Context, item subscription.Subscription) (subscription.Subscription, error)
	GetByID(ctx context.Context, id uuid.UUID) (subscription.Subscription, error)
	Update(ctx context.Context, item subscription.Subscription) (subscription.Subscription, error)
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, filter subscription.ListFilter) ([]subscription.Subscription, int64, error)
	CalculateTotalCost(ctx context.Context, filter subscription.TotalCostFilter) (int, error)
}

type Handler struct {
	service SubscriptionService
	logger  *slog.Logger
}

func NewHandler(service SubscriptionService, logger *slog.Logger) *Handler {
	return &Handler{service: service, logger: logger}
}

func (h *Handler) ListSubscriptions(w http.ResponseWriter, r *http.Request, params ListSubscriptionsParams) {
	const methodCtx = "http.ListSubscriptions"
	log := h.logger.With("method", methodCtx)

	filter := subscription.ListFilter{Limit: 20}
	if params.UserID != nil {
		filter.UserID = params.UserID
	}
	if params.ServiceName != nil {
		filter.ServiceName = params.ServiceName
	}
	if params.Limit != nil {
		filter.Limit = *params.Limit
	}
	if params.Offset != nil {
		filter.Offset = *params.Offset
	}

	items, total, err := h.service.List(r.Context(), filter)
	if err != nil {
		log.ErrorContext(r.Context(), "list failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Message: err.Error()})
		return
	}

	responseItems := make([]SubscriptionResponse, 0, len(items))
	for _, item := range items {
		responseItems = append(responseItems, toResponse(item))
	}

	writeJSON(w, http.StatusOK, SubscriptionListResponse{Items: responseItems, Total: total, Limit: filter.Limit, Offset: filter.Offset})
}

func (h *Handler) CreateSubscription(w http.ResponseWriter, r *http.Request) {
	const methodCtx = "http.CreateSubscription"
	log := h.logger.With("method", methodCtx)

	var request CreateSubscriptionRequest
	if err := BindJSON(r.Context(), r, &request); err != nil {
		log.WarnContext(r.Context(), "decode failed", "error", err)
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Message: "invalid request body"})
		return
	}

	item, err := fromCreateRequest(request)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Message: err.Error()})
		return
	}

	created, err := h.service.Create(r.Context(), item)
	if err != nil {
		h.writeServiceError(w, r, err)
		return
	}

	writeJSON(w, http.StatusCreated, toResponse(created))
}

func (h *Handler) GetSubscription(w http.ResponseWriter, r *http.Request, subscriptionID uuid.UUID) {
	item, err := h.service.GetByID(r.Context(), subscriptionID)
	if err != nil {
		h.writeServiceError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, toResponse(item))
}

func (h *Handler) UpdateSubscription(w http.ResponseWriter, r *http.Request, subscriptionID uuid.UUID) {
	const methodCtx = "http.UpdateSubscription"
	log := h.logger.With("method", methodCtx)

	var request UpdateSubscriptionRequest
	if err := BindJSON(r.Context(), r, &request); err != nil {
		log.WarnContext(r.Context(), "decode failed", "error", err)
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Message: "invalid request body"})
		return
	}

	item, err := fromUpdateRequest(subscriptionID, request)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Message: err.Error()})
		return
	}

	updated, err := h.service.Update(r.Context(), item)
	if err != nil {
		h.writeServiceError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, toResponse(updated))
}

func (h *Handler) DeleteSubscription(w http.ResponseWriter, r *http.Request, subscriptionID uuid.UUID) {
	if err := h.service.Delete(r.Context(), subscriptionID); err != nil {
		h.writeServiceError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) CalculateTotalCost(w http.ResponseWriter, r *http.Request, params CalculateTotalCostParams) {
	start, err := month.Parse(string(params.StartPeriod))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Message: "invalid start_period"})
		return
	}
	end, err := month.Parse(string(params.EndPeriod))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Message: "invalid end_period"})
		return
	}

	filter := subscription.TotalCostFilter{
		StartPeriod: start,
		EndPeriod:   end,
		UserID:      params.UserID,
		ServiceName: params.ServiceName,
	}

	total, err := h.service.CalculateTotalCost(r.Context(), filter)
	if err != nil {
		h.writeServiceError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, TotalCostResponse{TotalCost: total, Currency: "RUB", StartPeriod: params.StartPeriod, EndPeriod: params.EndPeriod})
}

func (h *Handler) writeServiceError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, service.ErrNotFound):
		writeJSON(w, http.StatusNotFound, ErrorResponse{Message: err.Error()})
	case errors.Is(err, subscription.ErrInvalidDates):
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Message: err.Error()})
	default:
		if errors.Is(err, context.Canceled) {
			writeJSON(w, http.StatusRequestTimeout, ErrorResponse{Message: err.Error()})
			return
		}
		if err.Error() == "service name is required" || err.Error() == "price must be greater than zero" || err.Error() == "user id is required" || err.Error() == "start date is required" || err.Error() == "subscription id is required" || err.Error() == "start and end period are required" {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Message: err.Error()})
			return
		}
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Message: err.Error()})
	}
}

func fromCreateRequest(request CreateSubscriptionRequest) (subscription.Subscription, error) {
	start, err := month.Parse(string(request.StartDate))
	if err != nil {
		return subscription.Subscription{}, errors.New("invalid start_date")
	}
	var end *time.Time
	if request.EndDate != nil {
		value, err := month.Parse(string(*request.EndDate))
		if err != nil {
			return subscription.Subscription{}, errors.New("invalid end_date")
		}
		end = &value
	}

	return subscription.Subscription{ID: uuid.New(), ServiceName: request.ServiceName, Price: request.Price, UserID: request.UserID, StartDate: start, EndDate: end}, nil
}

func fromUpdateRequest(id uuid.UUID, request UpdateSubscriptionRequest) (subscription.Subscription, error) {
	start, err := month.Parse(string(request.StartDate))
	if err != nil {
		return subscription.Subscription{}, errors.New("invalid start_date")
	}
	var end *time.Time
	if request.EndDate != nil {
		value, err := month.Parse(string(*request.EndDate))
		if err != nil {
			return subscription.Subscription{}, errors.New("invalid end_date")
		}
		end = &value
	}

	return subscription.Subscription{ID: id, ServiceName: request.ServiceName, Price: request.Price, UserID: request.UserID, StartDate: start, EndDate: end}, nil
}

func toResponse(item subscription.Subscription) SubscriptionResponse {
	response := SubscriptionResponse{Id: item.ID, ServiceName: item.ServiceName, Price: item.Price, UserID: item.UserID, StartDate: month.Format(item.StartDate)}
	if item.EndDate != nil {
		formatted := MonthYear(month.Format(*item.EndDate))
		response.EndDate = &formatted
	}
	return response
}
