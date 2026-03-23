package httpapi

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

//go:embed openapi.yaml
var rawSpec []byte

type MonthYear = string

type CreateSubscriptionRequest struct {
	EndDate     *MonthYear `json:"end_date,omitempty"`
	Price       int        `json:"price"`
	ServiceName string     `json:"service_name"`
	StartDate   MonthYear  `json:"start_date"`
	UserID      uuid.UUID  `json:"user_id"`
}

type UpdateSubscriptionRequest struct {
	EndDate     *MonthYear `json:"end_date,omitempty"`
	Price       int        `json:"price"`
	ServiceName string     `json:"service_name"`
	StartDate   MonthYear  `json:"start_date"`
	UserID      uuid.UUID  `json:"user_id"`
}

type SubscriptionResponse struct {
	EndDate     *MonthYear `json:"end_date,omitempty"`
	Id          uuid.UUID  `json:"id"`
	Price       int        `json:"price"`
	ServiceName string     `json:"service_name"`
	StartDate   MonthYear  `json:"start_date"`
	UserID      uuid.UUID  `json:"user_id"`
}

type SubscriptionListResponse struct {
	Items  []SubscriptionResponse `json:"items"`
	Limit  int                    `json:"limit"`
	Offset int                    `json:"offset"`
	Total  int64                  `json:"total"`
}

type TotalCostResponse struct {
	Currency    string    `json:"currency"`
	EndPeriod   MonthYear `json:"end_period"`
	StartPeriod MonthYear `json:"start_period"`
	TotalCost   int       `json:"total_cost"`
}

type ErrorResponse struct {
	Message string `json:"message"`
}

type ListSubscriptionsParams struct {
	Limit       *int
	Offset      *int
	ServiceName *string
	UserID      *uuid.UUID
}

type CalculateTotalCostParams struct {
	EndPeriod   MonthYear
	ServiceName *string
	StartPeriod MonthYear
	UserID      *uuid.UUID
}

type ServerInterface interface {
	ListSubscriptions(w http.ResponseWriter, r *http.Request, params ListSubscriptionsParams)
	CreateSubscription(w http.ResponseWriter, r *http.Request)
	GetSubscription(w http.ResponseWriter, r *http.Request, subscriptionID uuid.UUID)
	UpdateSubscription(w http.ResponseWriter, r *http.Request, subscriptionID uuid.UUID)
	DeleteSubscription(w http.ResponseWriter, r *http.Request, subscriptionID uuid.UUID)
	CalculateTotalCost(w http.ResponseWriter, r *http.Request, params CalculateTotalCostParams)
}

func HandlerFromMux(si ServerInterface, r chi.Router) http.Handler {
	r.Get("/api/v1/subscriptions", func(w http.ResponseWriter, req *http.Request) {
		params, err := decodeListSubscriptionsParams(req)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Message: err.Error()})
			return
		}
		si.ListSubscriptions(w, req, params)
	})
	r.Post("/api/v1/subscriptions", si.CreateSubscription)
	r.Get("/api/v1/subscriptions/{subscription_id}", func(w http.ResponseWriter, req *http.Request) {
		id, err := uuid.Parse(chi.URLParam(req, "subscription_id"))
		if err != nil {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Message: "invalid subscription_id"})
			return
		}
		si.GetSubscription(w, req, id)
	})
	r.Put("/api/v1/subscriptions/{subscription_id}", func(w http.ResponseWriter, req *http.Request) {
		id, err := uuid.Parse(chi.URLParam(req, "subscription_id"))
		if err != nil {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Message: "invalid subscription_id"})
			return
		}
		si.UpdateSubscription(w, req, id)
	})
	r.Delete("/api/v1/subscriptions/{subscription_id}", func(w http.ResponseWriter, req *http.Request) {
		id, err := uuid.Parse(chi.URLParam(req, "subscription_id"))
		if err != nil {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Message: "invalid subscription_id"})
			return
		}
		si.DeleteSubscription(w, req, id)
	})
	r.Get("/api/v1/reports/total-cost", func(w http.ResponseWriter, req *http.Request) {
		params, err := decodeCalculateTotalCostParams(req)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Message: err.Error()})
			return
		}
		si.CalculateTotalCost(w, req, params)
	})
	return r
}

func GetSwagger() ([]byte, error) {
	return rawSpec, nil
}

func decodeListSubscriptionsParams(r *http.Request) (ListSubscriptionsParams, error) {
	query := r.URL.Query()
	params := ListSubscriptionsParams{}
	if value := query.Get("limit"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return params, fmt.Errorf("invalid limit")
		}
		params.Limit = &parsed
	}
	if value := query.Get("offset"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return params, fmt.Errorf("invalid offset")
		}
		params.Offset = &parsed
	}
	if value := query.Get("service_name"); value != "" {
		params.ServiceName = &value
	}
	if value := query.Get("user_id"); value != "" {
		parsed, err := uuid.Parse(value)
		if err != nil {
			return params, fmt.Errorf("invalid user_id")
		}
		params.UserID = &parsed
	}
	return params, nil
}

func decodeCalculateTotalCostParams(r *http.Request) (CalculateTotalCostParams, error) {
	query := r.URL.Query()
	params := CalculateTotalCostParams{}
	params.StartPeriod = MonthYear(query.Get("start_period"))
	params.EndPeriod = MonthYear(query.Get("end_period"))
	if params.StartPeriod == "" || params.EndPeriod == "" {
		return params, fmt.Errorf("start_period and end_period are required")
	}
	if value := query.Get("service_name"); value != "" {
		params.ServiceName = &value
	}
	if value := query.Get("user_id"); value != "" {
		parsed, err := uuid.Parse(value)
		if err != nil {
			return params, fmt.Errorf("invalid user_id")
		}
		params.UserID = &parsed
	}
	return params, nil
}

func BindJSON[T any](ctx context.Context, r *http.Request, dst *T) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(dst)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
