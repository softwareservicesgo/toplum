package itemCategory

import (
	"context"
)

type Repository interface {
	Create(ctx context.Context, dto ItemCategoryCreateDTO) (*ItemCategoryDTO, error)
	GetOne(ctx context.Context, itemCategoryId int) (*ItemCategoryDTO, error)
	GetAll(ctx context.Context, businessId, search, limit, offset string) (*ItemCategoryAllDTO, error)
	Update(ctx context.Context, itemCategoryId int, itemCategory ItemCategoryNameDTO) (*ItemCategoryDTO, error)
	Delete(ctx context.Context, itemCategoryId int) error
}
