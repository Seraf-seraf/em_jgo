package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/example/em_jgo/internal/domain/subscription"
	"github.com/example/em_jgo/internal/pkg/month"
	"github.com/example/em_jgo/internal/service"
)

type Repository struct {
	pool    *pgxpool.Pool
	builder sq.StatementBuilderType
}

func New(pool *pgxpool.Pool) *Repository {
	return &Repository{
		pool:    pool,
		builder: sq.StatementBuilder.PlaceholderFormat(sq.Dollar),
	}
}

func (r *Repository) Create(ctx context.Context, item subscription.Subscription) (subscription.Subscription, error) {
	query, args, err := r.builder.Insert("subscriptions").
		Columns("id", "service_name", "price", "user_id", "start_date", "end_date").
		Values(item.ID, item.ServiceName, item.Price, item.UserID, item.StartDate, item.EndDate).
		Suffix("RETURNING id, service_name, price, user_id, start_date, end_date, created_at, updated_at").
		ToSql()
	if err != nil {
		return subscription.Subscription{}, fmt.Errorf("build insert query: %w", err)
	}

	created, err := scanSubscription(r.pool.QueryRow(ctx, query, args...))
	if err != nil {
		return subscription.Subscription{}, fmt.Errorf("insert subscription: %w", err)
	}

	return created, nil
}

func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (subscription.Subscription, error) {
	query, args, err := r.builder.Select("id", "service_name", "price", "user_id", "start_date", "end_date", "created_at", "updated_at").
		From("subscriptions").
		Where(sq.Eq{"id": id}).
		ToSql()
	if err != nil {
		return subscription.Subscription{}, fmt.Errorf("build get query: %w", err)
	}

	item, err := scanSubscription(r.pool.QueryRow(ctx, query, args...))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return subscription.Subscription{}, service.ErrNotFound
		}
		return subscription.Subscription{}, fmt.Errorf("query subscription: %w", err)
	}

	return item, nil
}

func (r *Repository) Update(ctx context.Context, item subscription.Subscription) (subscription.Subscription, error) {
	query, args, err := r.builder.Update("subscriptions").
		Set("service_name", item.ServiceName).
		Set("price", item.Price).
		Set("user_id", item.UserID).
		Set("start_date", item.StartDate).
		Set("end_date", item.EndDate).
		Set("updated_at", sq.Expr("NOW()")).
		Where(sq.Eq{"id": item.ID}).
		Suffix("RETURNING id, service_name, price, user_id, start_date, end_date, created_at, updated_at").
		ToSql()
	if err != nil {
		return subscription.Subscription{}, fmt.Errorf("build update query: %w", err)
	}

	updated, err := scanSubscription(r.pool.QueryRow(ctx, query, args...))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return subscription.Subscription{}, service.ErrNotFound
		}
		return subscription.Subscription{}, fmt.Errorf("update subscription: %w", err)
	}

	return updated, nil
}

func (r *Repository) Delete(ctx context.Context, id uuid.UUID) error {
	query, args, err := r.builder.Delete("subscriptions").Where(sq.Eq{"id": id}).ToSql()
	if err != nil {
		return fmt.Errorf("build delete query: %w", err)
	}

	commandTag, err := r.pool.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("delete subscription: %w", err)
	}
	if commandTag.RowsAffected() == 0 {
		return service.ErrNotFound
	}

	return nil
}

func (r *Repository) List(ctx context.Context, filter subscription.ListFilter) ([]subscription.Subscription, int64, error) {
	base := r.builder.Select("id", "service_name", "price", "user_id", "start_date", "end_date", "created_at", "updated_at").
		From("subscriptions")
	countBase := r.builder.Select("COUNT(*)").From("subscriptions")

	base, countBase = applyListFilters(base, countBase, filter)

	countSQL, countArgs, err := countBase.ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("build count query: %w", err)
	}

	var total int64
	if err := r.pool.QueryRow(ctx, countSQL, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count subscriptions: %w", err)
	}

	query, args, err := base.OrderBy("start_date DESC", "created_at DESC").Limit(uint64(filter.Limit)).Offset(uint64(filter.Offset)).ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("build list query: %w", err)
	}

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list subscriptions: %w", err)
	}
	defer rows.Close()

	items := make([]subscription.Subscription, 0)
	for rows.Next() {
		item, err := scanSubscription(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan subscription: %w", err)
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate subscriptions: %w", err)
	}

	return items, total, nil
}

func (r *Repository) CalculateTotalCost(ctx context.Context, filter subscription.TotalCostFilter) (int, error) {
	query := strings.TrimSpace(`
		SELECT COALESCE(SUM((
			(EXTRACT(YEAR FROM LEAST(COALESCE(end_date, $2::date), $2::date)) - EXTRACT(YEAR FROM GREATEST(start_date, $1::date))) * 12 +
			(EXTRACT(MONTH FROM LEAST(COALESCE(end_date, $2::date), $2::date)) - EXTRACT(MONTH FROM GREATEST(start_date, $1::date))) + 1
		) * price), 0)
		FROM subscriptions
		WHERE start_date <= $2::date
		AND COALESCE(end_date, $2::date) >= $1::date`)

	args := []any{filter.StartPeriod, month.LastDay(filter.EndPeriod)}
	index := 3
	if filter.UserID != nil {
		query += fmt.Sprintf(" AND user_id = $%d", index)
		args = append(args, *filter.UserID)
		index++
	}
	if filter.ServiceName != nil {
		query += fmt.Sprintf(" AND service_name ILIKE $%d", index)
		args = append(args, *filter.ServiceName)
	}

	var total int
	if err := r.pool.QueryRow(ctx, query, args...).Scan(&total); err != nil {
		return 0, fmt.Errorf("calculate total cost: %w", err)
	}

	return total, nil
}

func applyListFilters(base sq.SelectBuilder, countBase sq.SelectBuilder, filter subscription.ListFilter) (sq.SelectBuilder, sq.SelectBuilder) {
	if filter.UserID != nil {
		base = base.Where(sq.Eq{"user_id": *filter.UserID})
		countBase = countBase.Where(sq.Eq{"user_id": *filter.UserID})
	}
	if filter.ServiceName != nil {
		base = base.Where(sq.ILike{"service_name": *filter.ServiceName})
		countBase = countBase.Where(sq.ILike{"service_name": *filter.ServiceName})
	}
	return base, countBase
}

type scanner interface {
	Scan(dest ...any) error
}

func scanSubscription(row scanner) (subscription.Subscription, error) {
	var item subscription.Subscription
	var endDate *time.Time
	if err := row.Scan(&item.ID, &item.ServiceName, &item.Price, &item.UserID, &item.StartDate, &endDate, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return subscription.Subscription{}, err
	}
	item.EndDate = endDate
	return item, nil
}
