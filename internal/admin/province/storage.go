package province

import "context"

type Repository interface {
	Create(ctx context.Context, province DictionaryDTO) (*ProvinceDTO, error)
	GetOne(ctx context.Context, provinceId int) (*ProvinceDTO, error)
	GetAll(ctx context.Context, search string, page string, size string) (*[]ProvinceDTO, error)
	Update(ctx context.Context, provinceId int, province DictionaryDTO) (*ProvinceDTO, error)
	Delete(ctx context.Context, provinceId int)  error
}
