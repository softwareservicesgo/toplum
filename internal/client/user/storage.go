package clients

import "context"

type Repository interface {
	Register(ctx context.Context, dto RegisterDTO) (int, error)
	CheckOTP(ctx context.Context, dto CheckOTP) (*ResultsOTP, *int, error)
	Login(ctx context.Context, dto LoginDTO) (*ResultsOTP, *int, error)
	CreateProfile(ctx context.Context, clientID int, client UserReqDTO, imagePath *string, hashPassword string, baseURL string) (*Profile, error)
	GetProfile(ctx context.Context, clientID int, baseURL string) (*Profile, error)
	UpdateProfile(ctx context.Context, clientID int, client UserReqDTO, imagePath *string, hashPassword string, baseURL string) (*Profile, error)
	Logout(ctx context.Context, token string) error
}
