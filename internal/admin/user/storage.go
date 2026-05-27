package user

import "context"

type Repository interface {
	Create(ctx context.Context, User UserReqDTO, hashPassword string) (*UserDTO, error)
	GetOne(ctx context.Context, userId int) (*UserDTO, error)
	GetAll(ctx context.Context, value map[string]string) (*[]UserDTO, *int, error)
	Update(ctx context.Context, userID int, User UserUpdateDTO, hashPassword string) (*UserDTO, error)
	Delete(ctx context.Context, userId int, role string) error
	GetAllByBusiness(ctx context.Context, businessId int, limit int, offset int) (*UsersForBusiness, error)
}