package jobs

import (
	"context"
	"fmt"
	"time"

	"restaurants/internal/client/notification"
	"restaurants/pkg/fcm"

	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/robfig/cron/v3"
)

type StayedMinutes struct {
	DB        *pgxpool.Pool
	FCMClient *fcm.FCMClient
}

func StayedMinutesDB(db *pgxpool.Pool, fcmClient *fcm.FCMClient) *StayedMinutes {
	return &StayedMinutes{
		DB:        db,
		FCMClient: fcmClient,
	}
}

func (r *StayedMinutes) StartStayedMinutesScheduler(ctx context.Context) {
	c := cron.New()

	_, err := c.AddFunc("@every 1m", func() {
		r.Run(ctx)
	})
	if err != nil {
		fmt.Println("[ERROR-STAYED-15-MIN] Failed to schedule job: ", err)
	}
	c.Start()
	fmt.Println("[JOB-STAYED-15-MIN] ReservationChecker started — runs every 1 minute")
}

func (r *StayedMinutes) Run(ctx context.Context) {
	fmt.Println("[JOB-STAYED-15-MIN] Starting stayed ReservationIDs check. Now : ", time.Now())

	query := `
		SELECT 
			dt.token, 
			r.restaurant_id,
			res.name
	FROM reservations r
	JOIN device_token dt ON dt.client_id = r.client_id
	JOIN restaurants res ON res.id = r.restaurant_id
	WHERE r.status = 'approved'
		AND r.reservation_date = NOW() + (15 || ' minutes')::interval ;
	`
	rows, err := r.DB.Query(ctx, query)
	if err != nil {
		fmt.Println("[ERROR-STAYED-15-MIN] Reservation check failed: ", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var (
			FCMToken   string
			restaurant fcm.Restaurant
		)
		if err := rows.Scan(
			&FCMToken,
			&restaurant.Id,
			&restaurant.Name,
		); err != nil {
			fmt.Println("[ERROR-STAYED-15-MIN] Failed to scan id: ", err)
			continue
		}

		if FCMToken != "" {
			title := notification.DictionaryDTO{
				Tm: "Ýatlatma",
				En: "Reminder",
				Ru: "Напоминание",
			}

			content := notification.DictionaryDTO{
				Tm: "Siziň restarany bron eden wagtynyza 15 min galdy",
				En: "There are 15 minutes left until your restaurant reservation time",
				Ru: "До времени вашей брони ресторана осталось 15 минут",
			}

			ntfMin := fcm.NotificationDTO{
				Id:         0,
				Restaurant: restaurant,
				Title:      fcm.DictionaryDTO(title),
				Content:    fcm.DictionaryDTO(content),
				LifeDay:    0,
				CreatedAt:  time.Now().Format("2006-01-02 15:04"),
			}
			err = r.FCMClient.SendToToken(
				context.TODO(),
				FCMToken,
				&ntfMin,
			)

			if err != nil {
				fmt.Println("[ERROR-STAYED-15-MIN] Failed to send push to client with token:", FCMToken, "error:", err)
			} else {
				fmt.Println("[JOB-STAYED-15-MIN] Push sent to client with token:", FCMToken)
			}
		} else {
			fmt.Println("[JOB-STAYED-15-MIN] No clients stayed 15 min reservations")
		}
	}
}
