package order

import "context"

type Repository interface {
	Create(ctx context.Context, clientID int, order CreateOrderReq) (*int, error)
	GetOne(ctx context.Context, orderId int, baseURL string) (*OrderOne, error)
	GetAllForClient(ctx context.Context, clientID int, limit, offset, status, search, baseURL string) (*OrderAllForClient, error)
	GetAllForBusinesses(ctx context.Context, businessesId, userId int, limit, offset, status, baseURL string) (*OrderAllForBusinesses, error)
	Update(ctx context.Context, clientID, orderID int, req UpdateOrderReq, baseURL string) (*OrderOne, error)
	Delete(ctx context.Context, clientID, orderID int) (int, int, error) // businessesId, clientId, error
	UpdateStatusByBusinesses(ctx context.Context, userId, orderID int, req UpdateOrderStatusReq) (int, int, error) // businessesId, clientId, error
	UpdateStatusByClient(ctx context.Context, clientID, orderID int, req UpdateOrderStatusReq) (int, int, error) // businessesId, clientId, error
}

type RepositoryWS interface {
	CheckBusinesses(ctx context.Context, restaurantId int, userId int) error
	CheckClient(ctx context.Context, clientId int) error
	CheckOrder(ctx context.Context, orderId int) error
}