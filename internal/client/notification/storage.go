package notification

import (
	"context"
	"restaurants/internal/client/deviceToken"
	"restaurants/pkg/fcm"
)

type Repository interface {
	CreateNotification(ctx context.Context, notification NotificationReqDTO, userId int) (*fcm.NotificationDTO, error)
	CreateClientNotification(ctx context.Context, tokens *[]deviceToken.DeviceTokens, userId int) error
	GetNotificationById(ctx context.Context, notificationId int) (*fcm.NotificationDTO, error)
	GetAll(ctx context.Context, filter NotificationFilter) (*Notifications, error)
	GetAllByClient(ctx context.Context, clientId int, deviceToken string, filter NotificationClientFilter) (*NotificationsClient, error)
	ReadClient(ctx context.Context, userId int, deviceToken string) error
	Delete(ctx context.Context, notificationId, userId int) error
}
