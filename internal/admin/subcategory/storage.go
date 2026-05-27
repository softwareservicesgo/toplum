package subcategory

import "context"

type Repository interface {
	Create(ctx context.Context, subcategory SubcategoryDTO, imagePath string, baseURL string) (*SubcategoryOneDTO, error)
	GetOne(ctx context.Context, subcategoryID int, baseURL string) (*SubcategoryOneDTO, error)
	GetAll(ctx context.Context, search string, page string, size string, categoryId string, baseURL string) (*SubcategoryAllDTO, error)
	Update(ctx context.Context, subcategoryID int, category SubcategoryDTO, imagePath string, baseURL string) (*SubcategoryOneDTO, error)
	Delete(ctx context.Context, subcategoryID int) error
	ByCategory(ctx context.Context, categoryIDs []int, baseURL string) (*[]ByCategory, error)
}
