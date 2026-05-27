package db

import (
	"context"
	"fmt"
	"restaurants/internal/appresult"
	"restaurants/internal/client/deviceToken"
	"restaurants/pkg/client/postgresql"
	"restaurants/pkg/logging"
)

type repository struct {
	client postgresql.Client
	logger *logging.Logger
}

func NewRepository(client postgresql.Client, logger *logging.Logger) deviceToken.Repository {
	return &repository{
		client: client,
		logger: logger,
	}
}

func (r *repository) Create(ctx context.Context, token deviceToken.DeviceToken) error {
	var (
		exists, clientExists bool
	)

	if token.ClientId != nil {
		q := `
			SELECT EXISTS (
				SELECT 1 FROM clients WHERE id = $1
			)
		`
		err := r.client.QueryRow(ctx, q, token.ClientId).Scan(&clientExists)
		if err != nil {
			return appresult.ErrInternalServer
		}

		if !clientExists {
			return appresult.ErrNotFoundType(*token.ClientId, "client")
		}
	}

	q := `
		SELECT EXISTS (
			SELECT 1 
			FROM device_token 
			WHERE token = $1
		)
	`
	err := r.client.QueryRow(ctx, q, token.Token).Scan(&exists)
	if err != nil {
		return appresult.ErrInternalServer
	}

	if exists {
		return appresult.ErrAlreadyData(
			fmt.Sprintf("token=%s", token.Token),
		)
	}

	q = `
		INSERT INTO device_token (client_id, token)
		VALUES ($1, $2)
	`

	_, err = r.client.Exec(ctx, q, token.ClientId, token.Token)
	if err != nil {
		return appresult.ErrInternalServer
	}

	return nil
}

func (r *repository) Delete(ctx context.Context, clientId int) error {
	q := `
		DELETE FROM device_token
		WHERE client_id = $1
	`

	res, err := r.client.Exec(ctx, q, clientId)
	if err != nil {
		fmt.Println("error:", err)
		return appresult.ErrInternalServer
	}

	if res.RowsAffected() == 0 {
		return appresult.ErrNotFoundType(clientId, "device token for client")
	}

	return nil
}
