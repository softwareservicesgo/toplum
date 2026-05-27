package jobs

import (
	"context"
	"database/sql"
	"fmt"
	"restaurants/internal/client/notification"
	"restaurants/internal/client/reservation"
	"restaurants/pkg/fcm"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/robfig/cron/v3"
)

type cleanDataBase struct {
	DB        *pgxpool.Pool
	FCMClient *fcm.FCMClient
}

func CleanDataBase(db *pgxpool.Pool, fcmClient *fcm.FCMClient) *cleanDataBase {
	return &cleanDataBase{
		DB:        db,
		FCMClient: fcmClient,
	}
}

func (r *cleanDataBase) StartCleanDataBaseScheduler(
	ctx context.Context,
	repository reservation.Repository) {
	c := cron.New()

	_, err := c.AddFunc("0 0 0 * * *", func() {
		r.Run(ctx, repository)
	})
	if err != nil {
		fmt.Println("[ERROR-CLEAN] Failed to schedule job: ", err)
	}

	c.Start()
	fmt.Println("[JOB-CLEAN] ReservationChecker started — runs every 1 minute")
}

func (r *cleanDataBase) Run(
	ctx context.Context,
	repository reservation.Repository,
) {
	fmt.Println("[JOB-CLEAN] Start:", time.Now())

	r.deleteExpiredCoupons(ctx)
	r.deleteReservations(ctx)

	fmt.Println("[JOB-CLEAN] Finish:", time.Now())
}

func (r *cleanDataBase) deleteReservations(ctx context.Context) {

	_, err := r.DB.Exec(ctx, `
		DELETE FROM client_coupons
		WHERE reservation_id IN (
			SELECT id
			FROM reservations
			WHERE (
				status IN (
					'completed',
					'no_show',
					'cancelled_by_client',
					'cancelled_by_restaurant'
				)
				AND updated_at <= NOW() - INTERVAL '1 days'
			)
			OR reservation_date <= NOW() - INTERVAL '1 days'
		)
		OR reservation_id < 0
	`)
	if err != nil {
		fmt.Printf("[ERROR-CLEAN] delete client_coupons: %+v\n", err)
		return
	}

	_, err = r.DB.Exec(ctx, `
		DELETE FROM reservation_foods
		WHERE reservation_id IN (
			SELECT id
			FROM reservations
			WHERE (
				status IN (
					'completed',
					'no_show',
					'cancelled_by_client',
					'cancelled_by_restaurant'
				)
				AND updated_at <= NOW() - INTERVAL '1 days'
			)
			OR reservation_date <= NOW() - INTERVAL '1 days'
		)
		OR reservation_id < 0
	`)
	if err != nil {
		fmt.Printf("[ERROR-CLEAN] delete client_coupons: %+v\n", err)
		return
	}

	rows, err := r.DB.Query(ctx, `
		DELETE FROM reservations
		WHERE (
			status IN (
				'completed',
				'no_show',
				'cancelled_by_client',
				'cancelled_by_restaurant'
			)
			AND updated_at <= NOW() - INTERVAL '1 days'
		)
		OR reservation_date <= NOW() - INTERVAL '1 days'
		RETURNING id
	`)
	if err != nil {
		fmt.Printf("[ERROR-CLEAN] delete reservations: %+v\n", err)
		return
	}

	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			fmt.Println("[ERROR-CLEAN] scan:", err)
			rows.Close()
			return
		}
		fmt.Println("[JOB-CLEAN] Deleted reservation:", id)
	}

	if err := rows.Err(); err != nil {
		fmt.Println("[ERROR-CLEAN] rows error:", err)
		rows.Close()
		return
	}

	rows.Close()
}

func (r *cleanDataBase) deleteExpiredCoupons(ctx context.Context) {
	fmt.Println("[JOB-CLEAN] Cleaning expired coupons")

	title := notification.DictionaryDTO{
		Tm: "Bildiriş",
		En: "Announce",
		Ru: "Объявить",
	}

	deleteClientCoupons := `
		DELETE FROM client_coupons cc
	USING restaurant_coupons rc
	WHERE rc.id = cc.restaurant_coupon_id
	AND rc.created_at + (rc.life || ' days')::INTERVAL < NOW()
	AND cc.reservation_id > 0
	RETURNING
		cc.id,
		cc.client_id,
		rc.coupon_dictionary_id,
		rc.restaurant_id;
	`

	rows, err := r.DB.Query(ctx, deleteClientCoupons)
	if err != nil {
		fmt.Println("[ERROR-CLEAN] Delete client_coupons failed:", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var (
			id, clientId, couponDictionaryId, restaurantId int
			resName                                        string
			couponTM, couponRU, couponEN                   string
			FCMToken                                       sql.NullString
		)
		_ = rows.Scan(&id, &clientId, &couponDictionaryId, &restaurantId)

		fmt.Println("[JOB-CLEAN] Deleted client_coupon_id:", id)
		fmt.Println("[JOB-CLEAN] Deleted client_id:", clientId)
		fmt.Println("[JOB-CLEAN] Deleted restaurant_шd:", restaurantId)

		q := `
			SELECT
				r.name  AS res_name
				dCoupon.tm AS coupon_tm,
				dCoupon.ru AS coupon_ru,
				dCoupon.en AS coupon_en,
				dt.token
			FROM dictionary dCoupon ON dCoupon.id = $2
			JOIN restaurant r 
			JOIN device_tokens dt ON dt.client_id = $3
			WHERE r.id = $1;
		`
		err := r.DB.QueryRow(ctx, q, restaurantId, couponDictionaryId, clientId).Scan(
			&resName,
			&couponTM,
			&couponRU,
			&couponEN,
			&FCMToken,
		)
		if err != nil {
			return
		}

		if !FCMToken.Valid {
			continue
		}
		tm := fmt.Sprintf(
			"%s restarana degişli siziň %s kuponyňyzyň wagty gutaranlygy sebäpli ol ýatyryldy",
			resName,
			couponTM,
		)

		ru := fmt.Sprintf(
			"Ваш купон %s ресторана %s был отменён, так как истёк срок его действия",
			couponRU,
			resName,
		)

		en := fmt.Sprintf(
			"Your %s coupon from the restaurant %s was cancelled because it has expired",
			couponEN,
			resName,
		)

		content := fcm.DictionaryDTO{
			Tm: tm,
			Ru: ru,
			En: en,
		}

		restaurant := fcm.Restaurant{
			Id:   restaurantId,
			Name: resName,
		}

		ntf := fcm.NotificationDTO{
			Restaurant: restaurant,
			Title:      fcm.DictionaryDTO(title),
			Content:    fcm.DictionaryDTO(content),
			CreatedAt:  time.Now().Format("2006-01-02 15:04"),
		}

		err = r.FCMClient.SendToToken(ctx, FCMToken.String, &ntf)
		if err != nil {
			fmt.Println("[ERROR-RES-CHECK] Push failed:", err)
		} else {
			fmt.Println("[JOB-RES-CHECK] Push sent to:", clientId)
		}
	}
}
