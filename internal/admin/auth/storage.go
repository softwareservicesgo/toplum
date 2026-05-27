package auth

import "context"

type Repository interface {
	Login(ctx context.Context, dto LoginDTO) (*ResLoginDTO, error)
}
