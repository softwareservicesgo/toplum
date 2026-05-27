package jobs

import (
	"context"
	"fmt"
	"restaurants/internal/client/notification"
	"restaurants/internal/client/reservation"
	"restaurants/pkg/fcm"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/robfig/cron/v3"
)

type ReservationChecker struct {
	DB        *pgxpool.Pool
	FCMClient *fcm.FCMClient
}

func ReservationDB(db *pgxpool.Pool, fcmClient *fcm.FCMClient) *ReservationChecker {
	return &ReservationChecker{
		DB:        db,
		FCMClient: fcmClient,
	}
}

func (r *ReservationChecker) StartReservationCheckerScheduler(
	ctx context.Context,
	repository reservation.Repository) {
	c := cron.New()

	_, err := c.AddFunc("@every 1m", func() {
		r.Run(ctx, repository)
	})
	if err != nil {
		fmt.Println("[ERROR-RES-CHECK] Failed to schedule job: ", err)
	}

	c.Start()
	fmt.Println("[JOB-RES-CHECK] ReservationChecker started — runs every 1 minute")
}

func (r *ReservationChecker) Run(
	ctx context.Context,
	repository reservation.Repository,
) {
	var (
		updatedRestaurantIDs []int
		reservationIDs       []int
		rWS                  reservation.RepositoryWS
	)

	fmt.Println("[JOB-RES-CHECK] Starting reservation check:", time.Now())

	query := `
		UPDATE reservations AS res
		SET status = 'no_show',
			updated_at = CURRENT_TIMESTAMP AT TIME ZONE 'UTC'
		FROM restaurants AS rest
		WHERE res.restaurant_id = rest.id
		AND res.status = 'approved'
		AND res.reservation_date + (rest.expires || ' minutes')::interval <= NOW()
		RETURNING rest.id, res.id
	`

	rows, err := r.DB.Query(ctx, query)
	if err != nil {
		fmt.Println("[ERROR-RES-CHECK] Reservation update failed:", err)
		return
	}
	defer rows.Close()

	noShowRestaurants := make(map[int]bool)

	qForCoupon := `
		UPDATE client_coupons 
		SET reservation_id = $1,
			updated_at = CURRENT_TIMESTAMP AT TIME ZONE 'UTC'
		WHERE reservation_id = $2
	`

	for rows.Next() {
		var restID, resID int
		if err := rows.Scan(&restID, &resID); err != nil {
			fmt.Println("[ERROR-RES-CHECK] Scan error:", err)
			continue
		}

		if !noShowRestaurants[restID] {
			noShowRestaurants[restID] = true
			updatedRestaurantIDs = append(updatedRestaurantIDs, restID)
		}

		_, err := r.DB.Exec(ctx, qForCoupon, resID*(-1), resID)
		if err != nil {
			fmt.Println("[ERROR-RES-CHECK] client coupon update failed:", err)
			return
		}

		reservationIDs = append(reservationIDs, resID)
	}

	if len(reservationIDs) == 0 {
		fmt.Println("[JOB-RES-CHECK] No reservations to update.")
		return
	}

	fmt.Println("[JOB-RES-CHECK] Updated to no_show:", reservationIDs)

	q := `
		SELECT 
			dt.token,
			res.id,
			res.name,
			r.client_id
		FROM reservations r
		JOIN device_token dt ON dt.client_id = r.client_id
		JOIN restaurants res ON res.id = r.restaurant_id
		WHERE r.id = $1
	`
	title := notification.DictionaryDTO{
		Tm: "Bildiriş",
		En: "Announce",
		Ru: "Объявить",
	}

	content := notification.DictionaryDTO{
		Tm: "Siz berilen wagtyň içinde gelmedigiňiz sebäpli bron ýatyryldy.",
		En: "Your booking was cancelled because you did not arrive on time.",
		Ru: "Ваше бронирование отменено, так как вы не пришли вовремя.",
	}

	for _, resID := range reservationIDs {

		var (
			FCMToken   string
			restaurant fcm.Restaurant
			clientID   int
		)

		err := r.DB.QueryRow(ctx, q, resID).Scan(
			&FCMToken,
			&restaurant.Id,
			&restaurant.Name,
			&clientID,
		)

		if err != nil {
			fmt.Println("[ERROR-RES-CHECK] Failed get info for reservation:", resID, ":", err)
			continue
		}

		if FCMToken == "" {
			fmt.Println("[JOB-RES-CHECK] No FCM token for client:", clientID)
			continue
		}

		ntf := fcm.NotificationDTO{
			Restaurant: restaurant,
			Title:      fcm.DictionaryDTO(title),
			Content:    fcm.DictionaryDTO(content),
			CreatedAt:  time.Now().Format("2006-01-02 15:04"),
		}

		err = r.FCMClient.SendToToken(ctx, FCMToken, &ntf)
		if err != nil {
			fmt.Println("[ERROR-RES-CHECK] Push failed:", err)
		} else {
			fmt.Println("[JOB-RES-CHECK] Push sent to:", clientID)
		}
	}

	for _, restID := range updatedRestaurantIDs {
		err := reservation.NotifyRestaurantUpdate(ctx, restID, 0, repository, rWS)
		if err != nil {
			fmt.Println("[ERROR] Notify restaurant failed:", restID, ":", err)
		}
	}

	fmt.Println("[JOB-RES-CHECK] Completed.")
}
