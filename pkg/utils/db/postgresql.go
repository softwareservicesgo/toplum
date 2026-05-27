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

func (r *repository) UserRoleById(ctx context.Context, userId int, businessesId *int) (*string, error) {
	var role string

	query := `
		SELECT role
		FROM user_businesses
		WHERE (user_id = $1 AND role = $2 ) 
		   OR (user_id = $1 AND businesses_id = $3)

	`
	err := r.client.QueryRow(ctx, query, userId, enum.RoleAdmin, businessesId).Scan(&role)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			var content string
				if businessesId != nil {
					content = fmt.Sprintf("in user_businesses table, businesses_id=%d", *businessesId)
				} else {
					content = "in user_businesses table"
				}
			
			return nil, appresult.ErrNotFoundType(userId, content)
		}
		return nil, err
	}

	return &role, nil
}
