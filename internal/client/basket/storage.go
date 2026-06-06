package basket

import "context"

type Repository interface {
	Create(ctx context.Context, clientID int, basket BasketReq) error
	GetAll(ctx context.Context, clientId int, page string, size string, baseURL string) (*BasketsAll, error)
	GetCount(ctx context.Context, userId, itemId int) (int, error)
	Delete(ctx context.Context, clientId, itemId int) error
	DeleteFullById(ctx context.Context, userId, itemId int) error
	DeleteFull(ctx context.Context, userId int) error
}
