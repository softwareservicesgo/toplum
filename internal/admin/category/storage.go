package category

import "context"

type Repository interface {
	Create(ctx context.Context, category CategoryReqDTO, imagePath string, baseURL string) (*CategoryDTO, error)
	GetOne(ctx context.Context, categoryID int, baseURL string) (*CategoryDTO, error)
	GetAll(ctx context.Context, search string, page string, size string, baseURL string) (*CategoryAllDTO, error)
	Update(ctx context.Context, categoryID int, category CategoryReqDTO, imagePath string, baseURL string) (*CategoryDTO, error)
	Delete(ctx context.Context, categoryID int) error
}
