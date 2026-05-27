package item

import "context"

type Repository interface {
	Create(ctx context.Context, item ItemReqDTO, imagePath string, baseURL string) (*ItemGetOneDTO, error)
	GetOne(ctx context.Context, itemId int, baseURL string) (*ItemGetOneDTO, error)
	GetAll(ctx context.Context, filter ItemFilter, baseURL string) (*GetAllWithCount, error)
	Update(ctx context.Context, itemId int, item ItemReqDTO, imagePath string, baseURL string) (*ItemResForUpdateDTO, error)
	Delete(ctx context.Context, itemId int) error
	GetForUpdate(ctx context.Context, itemId int, baseURL string) (*ItemResForUpdateDTO, error)
	GetItemsByBusiness(ctx context.Context, businessId int, baseURL string) (*[]ItemGetAllDTO, error)
}
