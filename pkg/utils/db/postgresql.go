package db

import (
	"context"
	"errors"
	"fmt"
	"restaurants/internal/appresult"
	"restaurants/internal/enum"
	"restaurants/pkg/client/postgresql"
	"restaurants/pkg/logging"
	"restaurants/pkg/utils"
	"time"

	"github.com/jackc/pgx/v4"
)

type repository struct {
	client postgresql.Client
	logger *logging.Logger
}

func NewRepository(client postgresql.Client, logger *logging.Logger) utils.Repository {
	return &repository{
		client: client,
		logger: logger,
	}
}

var (
	updateLimitDays = 14
)

func (r *repository) UserRoleById(ctx context.Context, userId int, businessesId *int) (*string, error) {
	var role string

	query := `
		SELECT role
		FROM user_businesses
		WHERE (user_id = $1 AND role = $2 AND status = 'APPROVED') 
		   OR (user_id = $1 AND businesses_id = $3)

	`
	err := r.client.QueryRow(ctx, query, userId, enum.RoleAdmin, businessesId).Scan(&role)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			var content string
			if businessesId != nil {
				content = fmt.Sprintf("in user_businesses table, businesses_id=%d", *businessesId)
			} else {
				content = "don't have permission user"
			}

			return nil, appresult.ErrNotFoundType(userId, content)
		}
		return nil, err
	}

	return &role, nil
}

func (r *repository) UpdatePeriod(ctx context.Context, id int, tableName string) error {
	var updatedAt time.Time

	query := fmt.Sprintf(`SELECT updated_at FROM %s WHERE id = $1`, tableName)

	err := r.client.QueryRow(ctx, query, id).Scan(&updatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return appresult.ErrNotFoundType(id, tableName)
		}
		return err
	}

	cooldown := time.Duration(updateLimitDays) * 24 * time.Hour
	limitAt := updatedAt.Add(cooldown)

	if time.Now().UTC().Before(limitAt) {
		return appresult.ErrUpdatePeriodExpired(updateLimitDays)
	}

	query = fmt.Sprintf(`UPDATE %s SET updated_at = NOW() WHERE id = $1`, tableName)

	if _, err = r.client.Exec(ctx, query, id); err != nil {
		fmt.Println("error: ", err)
		return appresult.ErrInternalServer
	}

	return nil
}
