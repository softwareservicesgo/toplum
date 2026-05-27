package db

import (
	"context"
	"errors"
	"fmt"
	"math"
	"regexp"
	"restaurants/internal/appresult"
	"restaurants/internal/client/notification"
	"restaurants/internal/client/reservation"
	"restaurants/pkg/client/postgresql"
	"restaurants/pkg/fcm"
	"restaurants/pkg/logging"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v4"
)

type Repository struct {
	client postgresql.Client
	logger *logging.Logger
}

func NewRepository(client postgresql.Client, logger *logging.Logger) *Repository {
	return &Repository{
		client: client,
		logger: logger,
	}
}

func (r *Repository) CreateRecervation(ctx context.Context, restaurantId, clientId int, dto reservation.ReservationReqDTO, baseURL string) (*reservation.ReservationById, error) {
	tx, err := r.client.Begin(ctx)
	if err != nil {
		fmt.Println("error begin tx:", err)
		return nil, appresult.ErrInternalServer
	}
	defer tx.Rollback(ctx)

	var (
		reservationId int
		reservationID *int
		tableCount    int
		seats         int
		countUseTable int
	)
	query := `
				SELECT table_count, seats
			FROM restaurant_tables
			WHERE id = $1 AND restaurant_id = $2
		`
	err = tx.QueryRow(ctx, query, dto.TableId, restaurantId).Scan(&tableCount, &seats)
	// if err != nil {
	// 	fmt.Println("error :", err)
	// 	return nil, appresult.ErrWrong(dto.TableId, "table")
	// }
	if dto.PersonCount > seats {
		return nil, appresult.ErrCanSit
	}
	parsedTime, err := time.Parse("2006-01-02 15:04", dto.ReservationDate)
	if err != nil {
		fmt.Println("error parsing reservation date:", err)
		return nil, appresult.ErrInternalServer
	}
	reservationDate := parsedTime.Format("2006-01-02")
	reservationTime := parsedTime.Format("15:04")

	query = `
					SELECT COUNT(*)
				FROM reservations
				WHERE restaurant_id = $1 
				AND reservation_date::date = $2
				AND status IN ('pending','confirmed')
				AND restaurant_table_id = $3
				AND reservation_date::time <= $4::time
				AND id != $5
		 	`
	err = r.client.QueryRow(ctx, query, restaurantId, reservationDate, dto.TableId, reservationTime, reservationId).
		Scan(&countUseTable)
	if err != nil {
		fmt.Println("error :", err)
		return nil, err
	}

	if tableCount-countUseTable <= 0 {
		return nil, appresult.ErrEmpty
	}

	query = `
			INSERT INTO 
		reservations (
			restaurant_id, client_id, count_person, 
			reservation_date, wish_content, restaurant_table_id
			) 
		VALUES ($1, $2, $3, $4, $5, $6) 
		RETURNING id
	`
	err = tx.QueryRow(ctx, query,
		restaurantId, clientId, dto.PersonCount,
		dto.ReservationDate, dto.WishContent,
		dto.TableId).Scan(&reservationId)
	if err != nil {
		fmt.Println("error :", err)
		return nil, err
	}

	if dto.ClientCouponId != nil {
		var (
			life      int
			createdAt time.Time
		)
		checkQuery := `
			SELECT 
		cc.reservation_id, cc.created_at, rc.life
		FROM client_coupons cc
		JOIN restaurant_coupons rc ON rc.id = cc.restaurant_coupon_id
		WHERE cc.id = $1
		`
		err = tx.QueryRow(ctx, checkQuery, dto.ClientCouponId).Scan(
			&reservationID, &createdAt, &life)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, appresult.ErrNotFoundType(*dto.ClientCouponId, "client coupon")
			}
			fmt.Println("error :", err)
			return nil, appresult.ErrInternalServer
		}

		expiration := createdAt.AddDate(0, 0, life)
		if time.Now().After(expiration) {
			return nil, appresult.ErrExpiredCoupon(*dto.ClientCouponId)
		}

		if reservationID != nil {
			return nil, appresult.ErrAlreadyCoupon(*dto.ClientCouponId)
		}

		q := `
		UPDATE client_coupons
		SET reservation_id = $1
		WHERE id = $2;
	`
		_, err = tx.Exec(ctx, q, reservationId, dto.ClientCouponId)
		if err != nil {
			fmt.Println("error updating client coupon:", err)
			return nil, appresult.ErrInternalServer
		}
	}

	if len(dto.Foods) > 0 {
		query := `
				SELECT EXISTS (
				SELECT 1
			FROM foods
			WHERE id = $1 AND restaurant_id = $2
			)
		`
		q := `
			INSERT INTO reservation_foods (reservation_id, food_id, count)
			VALUES ($1, $2, $3)
		`
		for _, food := range dto.Foods {
			var checkFood bool

			err = tx.QueryRow(ctx, query, food.FoodId, restaurantId).Scan(&checkFood)
			if !checkFood {
				return nil, appresult.ErrNotFoundType(food.FoodId, "food")
			}
			if err != nil {
				fmt.Println("error :", err)
				return nil, appresult.ErrInternalServer
			}

			_, err = tx.Exec(ctx, q, reservationId, food.FoodId, food.Count)
			if err != nil {
				fmt.Println("error :", err)
				return nil, appresult.ErrInternalServer
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		fmt.Println("error commit transaction:", err)
		return nil, err
	}

	res, err := r.GetReservationById(ctx, reservationId, baseURL)
	if err != nil {
		fmt.Println("error :", err)
		return nil, err
	}
	return res, nil
}

func (r *Repository) ReservationForCheck(
	ctx context.Context,
	restaurantId, clientId int,
	dto reservation.ReservationReqDTO,
	baseURL string,
) (*reservation.ReservationById, error) {
	var (
		tableCount int
		foods      []reservation.Food
		total      float64
		used       int
	)

	res := &reservation.ReservationById{
		PersonCount: dto.PersonCount,
		WishContent: &dto.WishContent,
	}

	query := `
			SELECT c.id, c.name || ' ' || c.last_name, c.image_path, c.phone_number, 
			       r.expires, rt.id, rt.seats, rt.table_count
		FROM restaurants r
		JOIN clients c ON c.id = $1
		JOIN restaurant_tables rt ON rt.id = $2
		WHERE r.id = $3
	`
	err := r.client.QueryRow(ctx, query, clientId, dto.TableId, restaurantId).Scan(
		&res.Client.Id, &res.Client.FullName,
		&res.Client.ImagePath, &res.Client.PhoneNumber,
		&res.ReservationTime,
		&res.Table.TableId, &res.Table.Seats, &tableCount,
	)
	if err != nil {
		return nil, err
	}

	if dto.PersonCount > res.Table.Seats {
		return nil, appresult.ErrCanSit
	}

	parsedTime, err := time.Parse("2006-01-02 15:04", dto.ReservationDate)
	if err != nil {
		return nil, appresult.ErrInternalServer
	}

	res.ReservationDate = parsedTime.Format("2006-01-02 15:04")

	err = r.client.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM reservations
		WHERE restaurant_id = $1
		  AND reservation_date::date = $2
		  AND status IN ('pending','confirmed')
		  AND restaurant_table_id = $3
		  AND reservation_date::time <= $4::time
	`,
		restaurantId,
		parsedTime.Format("2006-01-02"),
		dto.TableId,
		parsedTime.Format("15:04"),
	).Scan(&used)

	if err != nil {
		return nil, err
	}

	if tableCount-used <= 0 {
		return nil, appresult.ErrEmpty
	}

	for _, f := range dto.Foods {
		var food reservation.Food

		err := r.client.QueryRow(ctx, `
			SELECT f.id, f.value, d.en, d.ru, d.tm, f.image_path
			FROM foods f
			JOIN dictionary d ON d.id = f.name_dictionary_id
			WHERE f.id = $1 AND f.restaurant_id = $2
		`, f.FoodId, restaurantId).Scan(
			&food.FoodId,
			&food.Value,
			&food.FoodName.En,
			&food.FoodName.Ru,
			&food.FoodName.Tm,
			&food.FoodImage,
		)

		if err != nil {
			return nil, appresult.ErrNotFoundType(f.FoodId, "food")
		}

		food.Count = f.Count
		total += float64(food.Value) * float64(f.Count)

		if food.FoodImage != "" {
			food.FoodImage = fmt.Sprintf(
				"%s/%s",
				baseURL,
				strings.ReplaceAll(food.FoodImage, "\\", "/"),
			)
		}

		foods = append(foods, food)
	}
	res.Foods = &foods

	if dto.ClientCouponId != nil && *dto.ClientCouponId != 0 {

		var (
			created time.Time
			life    int
		)

		res.ClientCoupon = &reservation.ClientCoupon{}

		err := r.client.QueryRow(ctx, `
		SELECT cc.created_at, rc.life, d.en, d.ru, d.tm
		FROM client_coupons cc
		JOIN restaurant_coupons rc ON rc.id = cc.restaurant_coupon_id
		JOIN dictionary d ON d.id = rc.coupon_dictionary_id
		WHERE cc.id = $1 AND cc.client_id = $2 AND reservation_id IS NULL
	`, *dto.ClientCouponId, clientId).Scan(
			&created,
			&life,
			&res.ClientCoupon.CouponName.En,
			&res.ClientCoupon.CouponName.Ru,
			&res.ClientCoupon.CouponName.Tm,
		)

		res.ClientCoupon.Id = *dto.ClientCouponId

		if err != nil {
			return nil, appresult.ErrNotFoundType(*dto.ClientCouponId, "coupon")
		}

		if time.Now().After(created.AddDate(0, 0, life)) {
			return nil, appresult.ErrExpiredCoupon(*dto.ClientCouponId)
		}

		re := regexp.MustCompile(`\d+%`)
		match := re.FindString(res.ClientCoupon.CouponName.En)

		if match != "" {
			p, _ := strconv.Atoi(strings.TrimSuffix(match, "%"))
			total -= total * float64(p) / 100
		} else {
			re := regexp.MustCompile(`(\d+(\.\d+)?)`)
			match := re.FindString(res.ClientCoupon.CouponName.En)
			if match != "" {
				v, _ := strconv.Atoi(match)
				total -= float64(v)
			}
			if total < 0 {
				total = 0
			}
		}
	}

	if total != 0 {
		generalBill := math.Round(total*100) / 100
		initialBill := math.Round(total*10) / 100
		res.GeneralBill = &generalBill
		res.InitialBill = &initialBill
	}
	return res, nil
}

func (r *Repository) Update(ctx context.Context, reservationId int, dto reservation.ReservationPatchDTO, clientId int, baseURL string) (*reservation.ReservationById, *int, error) {
	var (
		status        string
		restaurantId  int
		seats         int
		countUseTable int
		tableCount    int
	)
	tx, err := r.client.Begin(ctx)
	if err != nil {
		fmt.Println("error begin tx:", err)
		return nil, nil, appresult.ErrInternalServer
	}
	defer tx.Rollback(ctx)

	q := `SELECT status, restaurant_id FROM reservations WHERE id = $1 AND client_id = $2`
	err = tx.QueryRow(ctx, q, reservationId, clientId).Scan(&status, &restaurantId)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, appresult.ErrNotFoundType(reservationId, "reservation")
		}
		fmt.Println("error:", err)
		return nil, nil, appresult.ErrInternalServer
	}
	if status != "pending" &&
		status != "cancelled_by_restaurant" &&
		status != "cancelled_by_client" {
		return nil, nil, appresult.ErrStatus
	}

	q = `UPDATE client_coupons SET reservation_id = null WHERE reservation_id = $1`
	_, err = tx.Exec(ctx, q, reservationId)
	if err != nil {
		fmt.Println("error :", err)
		return nil, nil, appresult.ErrInternalServer
	}

	if dto.ClientCouponId != nil {
		var (
			life               int
			createdAt          time.Time
			checkReservationId *int
		)
		q = `
			SELECT rc.life, cc.created_at, cc.reservation_id
		FROM restaurant_coupons rc
		JOIN client_coupons cc ON rc.id = cc.restaurant_coupon_id
		WHERE cc.id = $1
`
		err = tx.QueryRow(ctx, q, *dto.ClientCouponId).Scan(&life, &createdAt, &checkReservationId)
		if err != nil {
			fmt.Println("error :", err)
			return nil, nil, appresult.ErrInternalServer
		}
		if checkReservationId != nil {
			return nil, nil, appresult.ErrAlreadyCoupon(*dto.ClientCouponId)
		}

		expiration := createdAt.AddDate(0, 0, life)
		if time.Now().After(expiration) {
			return nil, nil, appresult.ErrExpiredCoupon(*dto.ClientCouponId)
		}

		q = `UPDATE client_coupons SET reservation_id = $1 WHERE id = $2`
		_, err = tx.Exec(ctx, q, reservationId, *dto.ClientCouponId)
		if err != nil {
			fmt.Println("error :", err)
			return nil, nil, appresult.ErrInternalServer
		}
	}
	query := `
			SELECT table_count, seats
				FROM restaurant_tables
				WHERE id = $1
		`
	err = tx.QueryRow(ctx, query, dto.TableId).Scan(&tableCount, &seats)
	if err != nil {
		fmt.Println("error :", err)
		return nil, nil, appresult.ErrInternalServer
	}
	if dto.PersonCount > seats {
		return nil, nil, appresult.ErrCanSit
	}
	parsedTime, err := time.Parse("2006-01-02 15:04", dto.ReservationDate)
	if err != nil {
		fmt.Println("error parsing reservation date:", err)
		return nil, nil, appresult.ErrInternalServer
	}
	reservationDate := parsedTime.Format("2006-01-02")
	reservationTime := parsedTime.Format("15:04")

	query = `
					SELECT COUNT(*)
				FROM reservations
				WHERE restaurant_id = $1 
				AND reservation_date::date = $2
				AND status IN ('pending','confirmed')
				AND restaurant_table_id = $3
				AND reservation_date::time <= $4::time
				AND id != $5
		 	`
	err = r.client.QueryRow(ctx, query, restaurantId, reservationDate, dto.TableId, reservationTime, reservationId).
		Scan(&countUseTable)
	if err != nil {
		fmt.Println("error :", err)
		return nil, nil, err
	}

	if tableCount-countUseTable <= 0 {
		return nil, nil, appresult.ErrEmpty
	}
	_, err = tx.Exec(ctx, `DELETE FROM reservation_foods WHERE reservation_id = $1`, reservationId)
	if err != nil {
		fmt.Println("error :", err)
		return nil, nil, appresult.ErrInternalServer
	}
	if dto.Foods != nil {
		q := `INSERT INTO reservation_foods (reservation_id, food_id, count) VALUES ($1, $2, $3)`
		for _, f := range *dto.Foods {
			_, err = tx.Exec(ctx, q, reservationId, f.FoodId, f.Count)
			if err != nil {
				fmt.Println("error inserting food:", err)
				return nil, nil, appresult.ErrInternalServer
			}
		}
	}
	q = `
			UPDATE reservations SET 
		count_person = $1,
		reservation_date = $2,
		wish_content = $3,
		restaurant_table_id = $4,
		status = $5,
		reason = null
		WHERE id = $6
	`
	_, err = tx.Exec(ctx, q,
		dto.PersonCount, dto.ReservationDate,
		dto.WishContent, dto.TableId, "pending", reservationId)
	if err != nil {
		fmt.Println("error :", err)
		return nil, nil, appresult.ErrInternalServer
	}

	if err := tx.Commit(ctx); err != nil {
		fmt.Println("error commit transaction:", err)
		return nil, nil, appresult.ErrInternalServer
	}

	res, err := r.GetReservationById(ctx, reservationId, baseURL)
	if err != nil {
		fmt.Println("error get reservation:", err)
		return nil, nil, err
	}
	return res, &restaurantId, nil
}

func (r *Repository) GetReservationById(ctx context.Context, reservationId int, baseURL string) (*reservation.ReservationById, error) {
	var (
		reservations    reservation.ReservationById
		reservationDate time.Time
		ClientCoupon    reservation.ClientCoupon
		foods           []reservation.Food
		generalFoods    float64
	)

	query := `
			SELECT r.id, c.id, c.name || ' ' || c.last_name, c.image_path, c.phone_number, 
			       r.reservation_date, rest.expires, r.count_person, rt.id, rt.seats,
				   r.status, r.wish_content, r.reason
		FROM reservations r
		JOIN clients c ON c.id = r.client_id
		JOIN restaurants rest ON rest.id = r.restaurant_id
		JOIN restaurant_tables rt ON rt.id = r.restaurant_table_id
		WHERE r.id = $1
	`
	err := r.client.QueryRow(ctx, query, reservationId).Scan(
		&reservations.Id, &reservations.Client.Id, &reservations.Client.FullName,
		&reservations.Client.ImagePath, &reservations.Client.PhoneNumber, &reservationDate,
		&reservations.ReservationTime, &reservations.PersonCount, &reservations.Table.TableId,
		&reservations.Table.Seats, &reservations.Status, &reservations.WishContent, &reservations.Reason,
	)
	if err != nil {
		return nil, err
	}
	reservations.ReservationDate = reservationDate.Format("2006-01-02 15:04")

	if reservations.Client.ImagePath != "" {
		cleanPath := strings.ReplaceAll(reservations.Client.ImagePath, "\\", "/")
		reservations.Client.ImagePath = fmt.Sprintf("%s/%s", baseURL, cleanPath)
	}

	billQuery := `
				SELECT COALESCE(SUM(f.value * rf.count), 0)
			FROM reservation_foods rf
			JOIN foods f ON f.id = rf.food_id
			WHERE rf.reservation_id = $1
		`
	err = r.client.QueryRow(ctx, billQuery, reservationId).Scan(&generalFoods)
	if err != nil {
		fmt.Println("error:", err)
		return nil, err
	}

	couponQuery := `
			SELECT cc.id, d.en, d.ru, d.tm
		FROM client_coupons cc
		JOIN restaurant_coupons rc ON rc.id = cc.restaurant_coupon_id
		JOIN dictionary d ON d.id = rc.coupon_dictionary_id
		WHERE cc.reservation_id = $1
	`
	err = r.client.QueryRow(ctx, couponQuery, reservationId).Scan(
		&ClientCoupon.Id, &ClientCoupon.CouponName.En,
		&ClientCoupon.CouponName.Ru, &ClientCoupon.CouponName.Tm,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
		} else {
			fmt.Println("error:", err)
			return nil, err
		}
	} else {
		reservations.ClientCoupon = &ClientCoupon

		re := regexp.MustCompile(`\d+%`)
		match := re.FindString(ClientCoupon.CouponName.En)
		if match != "" {
			percentValueStr := strings.TrimSuffix(match, "%")
			percentValue, _ := strconv.Atoi(percentValueStr)
			generalFoods = generalFoods - (generalFoods * float64(percentValue) / 100)
		} else {
			re := regexp.MustCompile(`(\d+(\.\d+)?)`)
			match := re.FindString(ClientCoupon.CouponName.En)
			if match != "" {
				percentValueStr := strings.TrimSuffix(match, " ")
				percentValue, _ := strconv.Atoi(percentValueStr)
				generalFoods = generalFoods - float64(percentValue)
			}
			if generalFoods < 0 {
				generalFoods = 0
			}
		}
	}

	if generalFoods != 0 {
		generalBill := math.Round(generalFoods*100) / 100
		initialBill := math.Round(generalFoods*10) / 100
		reservations.GeneralBill = &generalBill
		reservations.InitialBill = &initialBill
	}

	foodQuery := `
				SELECT rf.id, f.id, d.en, d.ru, d.tm, f.image_path, f.value, rf.count
			FROM reservation_foods rf
			JOIN foods f ON f.id = rf.food_id
			JOIN dictionary d ON d.id = f.name_dictionary_id
			WHERE rf.reservation_id = $1
		`

	foodRows, err := r.client.Query(ctx, foodQuery, reservationId)
	if err != nil {
		fmt.Println("error fetching foods:", err)
		return nil, err
	}
	defer foodRows.Close()

	for foodRows.Next() {
		var f reservation.Food
		if err := foodRows.Scan(
			&f.Id, &f.FoodId, &f.FoodName.En, &f.FoodName.Ru,
			&f.FoodName.Tm, &f.FoodImage, &f.Value, &f.Count); err != nil {
			fmt.Println("error scan food:", err)
			return nil, err
		}

		if f.FoodImage != "" {
			cleanPath := strings.ReplaceAll(f.FoodImage, "\\", "/")
			f.FoodImage = fmt.Sprintf("%s/%s", baseURL, cleanPath)
		}
		foods = append(foods, f)
	}

	reservations.Foods = &foods

	return &reservations, nil
}

func (r *Repository) GetByRestaurantId(ctx context.Context, restaurantId int, filter reservation.ReservationFilter, baseURL string) (*reservation.Reservation, error) {
	var (
		reservationsList []reservation.ReservationById
		count            int
	)
	idx := 2
	args := []any{restaurantId}

	beginQuery := `SELECT r.id `
	beginQueryForCount := `SELECT count(r.id) `
	query := `
		FROM reservations r
		JOIN clients c ON c.id = r.client_id
		WHERE r.restaurant_id = $1
	`

	if filter.Status != "" {
		query += fmt.Sprintf(" AND r.status = $%d", idx)
		args = append(args, filter.Status)
		idx++
	}
	if filter.Day != "" {
		query += fmt.Sprintf(" AND r.reservation_date::date = $%d", idx)
		args = append(args, filter.Day)
		idx++
	}
	if filter.Search != "" {
		query += fmt.Sprintf(` AND (
			LOWER(c.name) LIKE LOWER($%d) OR 
			LOWER(c.last_name) LIKE LOWER($%d) OR 
			c.phone_number LIKE $%d
		)`, idx, idx, idx)
		args = append(args, "%"+filter.Search+"%")
		idx++
	}

	queryForCount := query
	query += " ORDER BY r.created_at DESC"

	offsetInt, err := strconv.Atoi(filter.Offset)
	if err != nil || offsetInt < 1 {
		offsetInt = 1
	}
	limitInt, err := strconv.Atoi(filter.Limit)
	if err != nil || limitInt < 1 {
		limitInt = 10
	}
	offsetInt = (offsetInt - 1) * limitInt
	query += fmt.Sprintf(" LIMIT %d OFFSET %d;", limitInt, offsetInt)

	rows, err := r.client.Query(ctx, beginQuery+query, args...)
	if err != nil {
		fmt.Println("error :", err)
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var reservationId int
		if err := rows.Scan(&reservationId); err != nil {
			fmt.Println("error :", err)
			return nil, err
		}

		reservation, err := r.GetReservationById(ctx, reservationId, baseURL)
		if err != nil {
			fmt.Println("error :", err)
			return nil, err
		}

		reservationsList = append(reservationsList, *reservation)
	}

	err = r.client.QueryRow(ctx, beginQueryForCount+queryForCount, args...).Scan(&count)
	if err != nil {
		fmt.Println("error :", err)
		return nil, err
	}

	reservationsData := reservation.Reservation{
		Count:        count,
		Reservations: reservationsList,
	}

	return &reservationsData, nil
}

func (r *Repository) GetByClientId(
	ctx context.Context,
	clientId int,
	filter reservation.ReservationClientFilter,
	baseURL string,
) (*reservation.Reservations, error) {

	var (
		results []reservation.ReservationClients
		total   int
	)

	offsetInt, _ := strconv.Atoi(filter.Offset)
	if offsetInt < 1 {
		offsetInt = 1
	}
	limitInt, _ := strconv.Atoi(filter.Limit)
	if limitInt < 1 {
		limitInt = 10
	}
	offsetInt = (offsetInt - 1) * limitInt

	query := `
        SELECT count(DISTINCT rest.id)
        FROM restaurants rest
        JOIN reservations r ON r.restaurant_id = rest.id
        WHERE r.client_id = $1
    `
	err := r.client.QueryRow(ctx, query, clientId).Scan(&total)
	if err != nil {
		return nil, err
	}

	query = fmt.Sprintf(`
        SELECT DISTINCT rest.id, ir.image_path, rest.price, rest.rating,
               dn.tm, dn.en, dn.ru,
               (d_province.tm || ', ' || COALESCE(d_district.tm,'')) AS address_tm,
               (d_province.en || ', ' || COALESCE(d_district.en,'')) AS address_en,
               (d_province.ru || ', ' || COALESCE(d_district.ru,'')) AS address_ru
        FROM restaurants rest
        JOIN reservations r ON r.restaurant_id = rest.id
        JOIN image_restaurants ir ON ir.restaurant_id = rest.id AND ir.is_main = true
        JOIN dictionary dn ON rest.name_dictionary_id = dn.id
        JOIN provinces p ON p.id = rest.province_id
        JOIN dictionary d_province ON d_province.id = p.name_dictionary_id
        JOIN dictionary d_district ON d_district.id = rest.district_dictionary_id
        WHERE r.client_id = $1
        LIMIT %d OFFSET %d
    `, limitInt, offsetInt)

	rows, err := r.client.Query(ctx, query, clientId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {

		var rest reservation.Restaurant

		err = rows.Scan(
			&rest.Id,
			&rest.Images,
			&rest.Price,
			&rest.Rating,
			&rest.Name.Tm,
			&rest.Name.En,
			&rest.Name.Ru,
			&rest.Address.Tm,
			&rest.Address.En,
			&rest.Address.Ru,
		)
		if err != nil {
			return nil, err
		}

		if rest.Images != "" {
			cleanPath := strings.ReplaceAll(rest.Images, "\\", "/")
			rest.Images = fmt.Sprintf("%s/%s", baseURL, cleanPath)
		}

		reservationsQuery := `
            SELECT id 
            FROM reservations 
            WHERE restaurant_id = $1 AND client_id = $2
            ORDER BY reservation_date DESC
        `
		reservationRows, err := r.client.Query(ctx, reservationsQuery, rest.Id, clientId)
		if err != nil {
			return nil, err
		}

		var allReservations []reservation.ReservationClient

		for reservationRows.Next() {
			var resId int
			reservationRows.Scan(&resId)

			fullRes, err := r.GetReservationById(ctx, resId, baseURL)
			if err != nil {
				return nil, err
			}

			allReservations = append(allReservations, reservation.ReservationClient{
				Id:              fullRes.Id,
				ReservationDate: fullRes.ReservationDate,
				ReservationTime: fullRes.ReservationTime,
				PersonCount:     fullRes.PersonCount,
				GeneralBill:     fullRes.GeneralBill,
				Status:          fullRes.Status,
				Reason:          fullRes.Reason,
			})
		}
		rest.CountReservation = len(allReservations)

		results = append(results, reservation.ReservationClients{
			Restaurant:   rest,
			Reservations: allReservations,
		})
	}

	reservations := reservation.Reservations{
		Count:                  total,
		RestaurantReservations: results,
	}

	return &reservations, nil
}

func (r *Repository) DeleteClientReservation(ctx context.Context, clientId, reservationId int, reason reservation.Reason) (*int, *int, *string, error) {
	return UpdateStatusReservation(r, ctx, reservationId, clientId, "cancelled_by_client", reason.Reason, true)
}

func (r *Repository) UpdateByRestaurantStatusReservation(ctx context.Context, userId, reservationId int, status reservation.UpdateStatusByRestaurant) (*int, *int, *string, error) {
	return UpdateStatusReservation(r, ctx, reservationId, userId, status.Status, status.Reason, false)
}

func UpdateStatusReservation(r *Repository, ctx context.Context, reservationId, clientId int, status string, reason string, isClient bool) (*int, *int, *string, error) {
	var (
		restaurantId int
		clientID     int
		deviceToken  string
	)
	if (status == "cancelled_by_client" || status == "cancelled_by_restaurant") && reason == "" {
		return nil, nil, nil, appresult.ErrReason
	}

	if isClient {
		query := `
		SELECT restaurant_id
		FROM reservations
		WHERE id = $1 AND client_id = $2
	`
		err := r.client.QueryRow(ctx, query, reservationId, clientId).Scan(&restaurantId)
		if err != nil {
			fmt.Println("error :", err)
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, nil, nil, appresult.ErrNotFoundTypeStr("reservation clients")
			}
			return nil, nil, nil, err
		}

		clientID = clientId
	} else {
		query := `
		SELECT r.restaurant_id, r.client_id
		FROM reservations r
		JOIN users u ON u.id = $2
		WHERE r.id = $1 AND ( u.role = 'admin' OR (u.role = 'manager' AND u.restaurant_id = r.restaurant_id ))
	`
		err := r.client.QueryRow(ctx, query, reservationId, clientId).Scan(&restaurantId, &clientID)
		if err != nil {
			fmt.Println("error :", err)
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, nil, nil, appresult.ErrNotFoundTypeStr("reservation clients")
			}
			return nil, nil, nil, err
		}
	}

	query := `
			UPDATE reservations
        SET status = $1, reason = $2
        WHERE id = $3
	`
	_, err := r.client.Exec(ctx, query, status, reason, reservationId)
	if err != nil {
		fmt.Println("error :", err)
		return nil, nil, nil, err
	}

	if status == "cancelled_by_client" || status == "cancelled_by_restaurant" {
		query = `
			UPDATE client_coupons
	    SET reservation_id = $1
	    WHERE reservation_id = $2
	`
		_, err = r.client.Exec(ctx, query, nil, reservationId)
		if err != nil {
			fmt.Println("error :", err)
			return nil, nil, nil, err
		}

		query := `
		SELECT token
		FROM device_token
		WHERE client_id = $1
	`
		err := r.client.QueryRow(ctx, query, clientId).Scan(&deviceToken)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			fmt.Println("error :", err)
			return nil, nil, nil, err
		}
	}
	return &restaurantId, &clientID, &deviceToken, nil
}

func (r *Repository) FindForNotification(ctx context.Context, restaurantId, clientId int) (*fcm.NotificationDTO, error) {
	var (
		deviceToken string
		name        string
	)
	query := `
		SELECT token
		FROM device_token
		WHERE client_id = $1
	`
	err := r.client.QueryRow(ctx, query, clientId).Scan(&deviceToken)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		fmt.Println("error :", err)
		return nil, err
	}

	title := notification.DictionaryDTO{
		Tm: "Bildiriş",
		En: "Announce",
		Ru: "Объявить",
	}

	content := notification.DictionaryDTO{
		Tm: "Siziň bronuňyz restoran tarapyndan ýatyryldy.",
		Ru: "Ваше бронирование было отменено рестораном.",
		En: "Your reservation was cancelled by the restaurant.",
	}

	query = `
		SELECT r.name
		FROM restaurants r
		WHERE r.id = $1
	`
	err = r.client.QueryRow(ctx, query, clientId).Scan(&name)
	if err != nil {
		fmt.Println("error :", err)
		return nil, err
	}

	restaurant := fcm.Restaurant{
		Id: restaurantId,
		Name: name,
	}

	ntf := fcm.NotificationDTO{
		Restaurant: restaurant,
		Title:      fcm.DictionaryDTO(title),
		Content:    fcm.DictionaryDTO(content),
		CreatedAt:  time.Now().Format("2006-01-02 15:04"),
	}

	return &ntf, nil
}
