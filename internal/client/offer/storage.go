package offer

import "context"

type Repository interface {
	Create(ctx context.Context, req CreateOfferReq, userId int) (*OfferDTO, error)
	GetAll(ctx context.Context, limit, offset int) (*GetAll, error)
	Clean(ctx context.Context) error
}
