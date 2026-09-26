package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/feaziest/kfdesktopbe/internal/db"
	"github.com/feaziest/kfdesktopbe/internal/domain"
)

// UserService handles user authentication sync, profiles, and activity statistics
type UserService struct {
	queries db.Querier
	pool    *pgxpool.Pool
}

// NewUserService creates a new UserService
func NewUserService(queries db.Querier, pool *pgxpool.Pool) *UserService {
	return &UserService{
		queries: queries,
		pool:    pool,
	}
}

// SyncUser upserts a Firebase user into Postgres
func (s *UserService) SyncUser(ctx context.Context, params domain.SyncUserParams) (*domain.User, error) {
	if strings.TrimSpace(params.ID) == "" {
		return nil, fmt.Errorf("%w: user id cannot be empty", domain.ErrValidation)
	}
	if strings.TrimSpace(params.Email) == "" {
		return nil, fmt.Errorf("%w: email cannot be empty", domain.ErrValidation)
	}

	u, err := s.queries.UpsertUser(ctx, db.UpsertUserParams{
		ID:          params.ID,
		Email:       params.Email,
		DisplayName: params.DisplayName,
		AvatarUrl:   params.AvatarURL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to sync user: %w", err)
	}

	return mapUserToDomain(u), nil
}

// GetMe retrieves the current user profile by UID
func (s *UserService) GetMe(ctx context.Context, uid string) (*domain.User, error) {
	if strings.TrimSpace(uid) == "" {
		return nil, domain.ErrUnauthorized
	}

	u, err := s.queries.GetUserByID(ctx, uid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return mapUserToDomain(u), nil
}

// GetContributions retrieves daily activity statistics for the user in the specified year
func (s *UserService) GetContributions(ctx context.Context, uid string, year int) ([]domain.DailyStat, error) {
	if strings.TrimSpace(uid) == "" {
		return nil, domain.ErrUnauthorized
	}
	if year <= 0 {
		year = time.Now().Year()
	}

	stats, err := s.queries.GetDailyStatsByYear(ctx, db.GetDailyStatsByYearParams{
		UserID: uid,
		Year:   int32(year),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get daily stats: %w", err)
	}

	res := make([]domain.DailyStat, 0, len(stats))
	for _, st := range stats {
		dateStr := ""
		if st.StatDate.Valid {
			dateStr = st.StatDate.Time.Format("2006-01-02")
		}
		res = append(res, domain.DailyStat{
			UserID:        st.UserID,
			StatDate:      dateStr,
			ActivityCount: st.ActivityCount,
		})
	}

	return res, nil
}
