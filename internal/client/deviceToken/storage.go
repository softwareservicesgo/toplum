package deviceToken

import "context"

type Repository interface {
	Create(ctx context.Context, review DeviceToken) error
	Delete(ctx context.Context, clientId int) error
}
