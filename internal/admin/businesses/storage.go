package businesses

import "context"

type Repository interface {
	Create(ctx context.Context, userId int, business BusinessesReqDTO) (*int, error)
	AddImages(ctx context.Context, businessId int, mainImage string, additionalImages []string, baseURL string) (*BusinessesResDTO, error)
	GetOne(ctx context.Context, businessId int, baseURL string) (*BusinessesResDTO, error)
	GetAll(ctx context.Context, filter BusinessesFilter, baseURL string) (*[]BusinessesAllDTO, *int, error)
	Update(ctx context.Context, businessId int, business BusinessesReqDTO) error
	Delete(ctx context.Context, businessId int) error
	GetAndDeleteImage(ctx context.Context, businessId int, isMain bool) (*[]string, error)
	UpdateStatus(ctx context.Context, businessId int, status UpdateStatus)  error
	Index(ctx context.Context, filter IndexFilter  , baseURL string) (*Index, error)
}