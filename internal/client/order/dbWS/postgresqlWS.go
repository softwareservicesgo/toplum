package dbWS

import (
	"context"
	"fmt"
	"restaurants/internal/appresult"
	"restaurants/pkg/client/postgresql"
	"restaurants/pkg/logging"
)

type RepositoryWS struct {
	client postgresql.Client
	logger *logging.Logger
}

func NewRepository(client postgresql.Client, logger *logging.Logger) *RepositoryWS {
	return &RepositoryWS{
		client: client,
		logger: logger,
	}
}

func (r *RepositoryWS) CheckBusinesses(ctx context.Context, businessesId int, userId int) error {
	var exists bool

	queryRestaurant := `SELECT EXISTS(SELECT 1 FROM businesses WHERE id = $1)`
	err := r.client.QueryRow(ctx, queryRestaurant, businessesId).Scan(&exists)
	if err != nil {
		fmt.Println("error :", err)
		return appresult.ErrInternalServer
	}
	if !exists {
		return appresult.ErrNotFoundType(businessesId, "businesses")
	}

	queryClient := `SELECT EXISTS(SELECT 1 FROM users WHERE id = $1 AND (businesses_id = $2 OR role = 'admin'))`
	err = r.client.QueryRow(ctx, queryClient, userId, businessesId).Scan(&exists)
	if err != nil {
		fmt.Println("error :", err)
		return appresult.ErrInternalServer
	}
	if !exists {
		return appresult.ErrNotFoundType(userId, "businesses for to user")
	}

	return nil
}

func (r *RepositoryWS) CheckClient(ctx context.Context, clientId int) error {
	var exists bool

	query := `SELECT EXISTS(SELECT 1 FROM clients WHERE id = $1)`
	err := r.client.QueryRow(ctx, query, clientId).Scan(&exists)
	if err != nil {
		fmt.Println("error :", err)
		return appresult.ErrInternalServer
	}
	if !exists {
		return appresult.ErrNotFoundType(clientId, "client")
	}

	return nil
}

func (r *RepositoryWS) CheckOrder(ctx context.Context, orderId int) error {
	var exists bool

	query := `SELECT EXISTS(SELECT 1 FROM orders WHERE id = $1)`
	err := r.client.QueryRow(ctx, query, orderId).Scan(&exists)
	if err != nil {
		fmt.Println("error :", err)
		return appresult.ErrInternalServer
	}
	if !exists {
		return appresult.ErrNotFoundType(orderId, "order")
	}

	return nil
}
