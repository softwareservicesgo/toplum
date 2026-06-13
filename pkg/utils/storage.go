package utils

import "context"

type Repository interface {
	UserRoleById(ctx context.Context, userId int, businessesId *int) (*string, error)
	UpdatePeriod(ctx context.Context, id int, businessesId string) error
}
