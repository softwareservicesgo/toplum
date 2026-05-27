package reservation

import (
	"context"
	"restaurants/pkg/fcm"
	"time"
)

type Repository interface {
	CreateRecervation(ctx context.Context, restaurantId, clientId int, restaurant ReservationReqDTO, baseURL string) (*ReservationById, error)
	ReservationForCheck(ctx context.Context, restaurantId, clientId int, restaurant ReservationReqDTO, baseURL string) (*ReservationById, error)
	Update(ctx context.Context, reservationId int, res ReservationPatchDTO, clientId int, baseURL string) (*ReservationById, *int, error)
	GetReservationById(ctx context.Context, restaurantId int, baseURL string) (*ReservationById, error)
	GetByRestaurantId(ctx context.Context, restaurantId int, filter ReservationFilter, baseURL string) (*Reservation, error)
	GetByClientId(ctx context.Context, clientId int, filter ReservationClientFilter, baseURL string) (*Reservations, error)
	DeleteClientReservation(ctx context.Context, clientId, reservationId int, reason Reason) (*int, *int, *string, error)
	UpdateByRestaurantStatusReservation(ctx context.Context, clientId, reservationId int, status UpdateStatusByRestaurant) (*int, *int, *string, error)
	FindForNotification(ctx context.Context, restaurantId, clientId int) (*fcm.NotificationDTO, error)
}

type RepositoryWS interface {
	CheckRestaurant(ctx context.Context, restaurantId int, clientId int) error
	AvailabilityDay(ctx context.Context, restaurantId int, availabilityDay AvailabilityDayReq, today, endDate time.Time) (*[]AvailabilityDay, error)
	AvailabilityTimes(ctx context.Context, restaurantId int, availabilityTime AvailabilityTimeReq) (*[]AvailabilityTime, *bool, error)
}
