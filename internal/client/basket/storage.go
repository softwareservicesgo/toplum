package basket

import "context"

type Repository interface {
	Create(ctx context.Context, clientID int, basket BasketReq) error
	GetOne(ctx context.Context, clientId int, restId int, clientCouponId string, baseURL string) (*Basket, error)
	GetAll(ctx context.Context, clientId int, page string, size string, baseURL string) (*BasketsAll, error)
	Delete(ctx context.Context, clientId, foodId int) error
}
