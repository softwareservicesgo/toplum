package db

import (
	"context"
	"database/sql"
	"fmt"
	"restaurants/internal/admin/auth"
	"restaurants/internal/appresult"
	"restaurants/pkg/client/postgresql"
	"restaurants/pkg/logging"

	"golang.org/x/crypto/bcrypt"
)

type repository struct {
	client postgresql.Client
	logger *logging.Logger
}

func NewRepository(client postgresql.Client, logger *logging.Logger) auth.Repository {
	return &repository{
		client: client,
		logger: logger,
	}
}

func (r *repository) Login(ctx context.Context, dto auth.LoginDTO) (*auth.ResLoginDTO, error) {
	q := `
			SELECT id, password, role, name, businesses_id
			FROM users
			WHERE name = $1
		`
	rows, err := r.client.Query(ctx, q, dto.Name)
	if err != nil {
		fmt.Println("error: ", err)
		return nil, appresult.ErrInternalServer
	}
	defer rows.Close()

	for rows.Next() {
		var resp auth.ResLoginDTO
		var businessesId sql.NullInt64

		if err := rows.Scan(&resp.Id, &resp.Password, &resp.Role, &resp.Name, &businessesId); err != nil {
			return nil, appresult.ErrInternalServer
		}

		if businessesId.Valid {
			resp.BusinessesId = int(businessesId.Int64)
		}

		if bcrypt.CompareHashAndPassword([]byte(resp.Password), []byte(dto.Password)) == nil {
			return &resp, nil
		}
	}

	return nil, appresult.ErrInvalidCredentials
}
