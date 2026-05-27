package db

import (
	"context"
	"errors"
	"fmt"
	"math"
	"regexp"
	"restaurants/internal/appresult"
	"restaurants/internal/client/basket"
	"restaurants/internal/client/order"
	"restaurants/pkg/client/postgresql"
	"restaurants/pkg/logging"
	"restaurants/pkg/utils"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v4"
)

type repository struct {
	client           postgresql.Client
	logger           *logging.Logger
	basketRepository basket.Repository
}

func NewRepository(client postgresql.Client, logger *logging.Logger, basketRepository basket.Repository) order.Repository {
	return &repository{
		client:           client,
		logger:           logger,
		basketRepository: basketRepository,
	}
}

func (r *repository) Create(ctx context.Context, clientId int, req order.CreateOrderReq) (*int, error) {

	tx, err := r.client.Begin(ctx)
	if err != nil {
		return nil, appresult.ErrInternalServer
	}
	defer tx.Rollback(ctx)

	var (
		baskets []order.Basket
		total   float64
		orderId int
	)
	q := `
		SELECT i.id, i.value, b.count
		FROM basket b
		JOIN items i ON b.item_id = i.id
		WHERE b.client_id = $1 AND i.businesses_id = $2
	`
	rows, err := tx.Query(ctx, q, clientId, req.BusinessesId)
	if err != nil {
		fmt.Println("error: ", err)
		return nil, appresult.ErrInternalServer
	}
	defer rows.Close()

	for rows.Next() {
		var basket order.Basket
		if err := rows.Scan(&basket.ItemId, &basket.Price, &basket.Count); err != nil {
			fmt.Println("error: ", err)
			return nil, appresult.ErrInternalServer
		}
		total += basket.Price * float64(basket.Count)
		baskets = append(baskets, basket)
	}

	if len(baskets) == 0 {
		return nil, appresult.ErrNotFoundType(req.BusinessesId, "basket by businesses")
	}

	if req.ClientCouponId != nil {
		totalWithCoupon, err := ApplyCoupon(ctx, r, clientId, *req.ClientCouponId, total)
		if err != nil {
			fmt.Println("error: ", err)
			return nil, err
		}
		if totalWithCoupon != nil {
			total = *totalWithCoupon
		}
	}

	if total != 0 {
		total = math.Round(total*100) / 100
	}

	parsedTime, err := time.Parse("2006-01-02 15:04", req.OrderTime)
	if err != nil {
		fmt.Println("error:", err)
		return nil, appresult.ErrInternalServer
	}

	q = `
	INSERT INTO orders 
		(client_id, businesses_id, client_coupon_id, total_price, place, order_time)
	VALUES ($1, $2, $3, $4, $5, $6)
	RETURNING id
	`
	err = tx.QueryRow(ctx, q,
		clientId,
		req.BusinessesId,
		req.ClientCouponId,
		total,
		req.Place,
		parsedTime,
	).Scan(&orderId)
	if err != nil {
		fmt.Println("error: ", err)
		return nil, appresult.ErrInternalServer
	}

	for _, basket := range baskets {
		_, err = tx.Exec(ctx, `
			INSERT INTO order_items (order_id, item_id, quantity, price)
			VALUES ($1,$2,$3,$4)
		`, orderId, basket.ItemId, basket.Count, basket.Price)
		if err != nil {
			fmt.Println("error: ", err)
			return nil, appresult.ErrInternalServer
		}
	}
	q = `
			DELETE FROM basket
		WHERE client_id = $1
		AND item_id IN (
		SELECT id FROM items WHERE businesses_id = $2
		)
	`
	_, err = tx.Exec(ctx, q, clientId, req.BusinessesId)
	if err != nil {
		fmt.Println("error: ", err)
		return nil, appresult.ErrInternalServer
	}

	if req.ClientCouponId != nil {
		_, err = tx.Exec(ctx, `
		UPDATE client_coupons
		SET booking_id = $1,
		    booking_type = $2,
		    updated_at = now()
		WHERE id = $3
	`, orderId, "ORDER", *req.ClientCouponId)
		if err != nil {
			fmt.Println("error: ", err)
			return nil, appresult.ErrInternalServer
		}
	}

	if err := tx.Commit(ctx); err != nil {
		fmt.Println("error: ", err)
		return nil, appresult.ErrInternalServer
	}
	return &orderId, nil
}

func (r *repository) GetOne(
	ctx context.Context,
	orderId int,
	baseURL string,
) (*order.OrderOne, error) {

	var (
		result         order.OrderOne
		clientId       int
		clientCouponId *int
	)

	q := `
			SELECT b.id, b.name, o.client_id, o.client_coupon_id, o.status
        FROM orders o
        JOIN businesses b ON b.id = o.businesses_id
        WHERE o.id = $1
	`

	result.Id = orderId
	err := r.client.QueryRow(ctx, q, orderId).Scan(
		&result.BusinessesId,
		&result.BusinessesName,
		&clientId,
		&clientCouponId,
		&result.Status,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, appresult.ErrNotFoundType(orderId, "order")
		}
		fmt.Println("Error: ", err)
		return nil, appresult.ErrInternalServer
	}

	items, total, countItems, err := FindItemsByBusinesses(ctx, r, orderId, result.BusinessesId, baseURL)
	if err != nil {
		fmt.Println("Error: ", err)
		return nil, err
	}

	result.Items = items
	result.CountItems = countItems
	result.GeneralBill = math.Round(total*100) / 100

	if clientCouponId != nil {
		if err = ApplyCouponWithGeneral(ctx, r, clientId, *clientCouponId, &result); err != nil {
			fmt.Println("Error: ", err)
			return nil, err
		}
	}

	return &result, nil
}

func CalculateDiscount(total float64, couponText string) float64 {
	re := regexp.MustCompile(`\d+%`)
	match := re.FindString(couponText)

	if match != "" {
		p, _ := strconv.Atoi(strings.TrimSuffix(match, "%"))
		total -= total * float64(p) / 100
	} else {
		re := regexp.MustCompile(`(\d+(\.\d+)?)`)
		match := re.FindString(couponText)
		if match != "" {
			v, _ := strconv.Atoi(match)
			total -= float64(v)
		}
		if total < 0 {
			total = 0
		}
	}
	return total
}

func FetchCoupon(ctx context.Context, r *repository, clientCouponId, clientId int) (*order.CouponData, *int, error) {
	var (
		c         order.CouponData
		bookingId *int
	)

	err := r.client.QueryRow(ctx, `
        SELECT cc.created_at, bc.life, d.tm, d.ru, d.en, cc.booking_id
        FROM client_coupons cc
        JOIN businesses_coupons bc ON bc.id = cc.businesses_coupon_id
        JOIN dictionary d ON d.id = bc.coupon_dictionary_id
        WHERE cc.id = $1 AND cc.client_id = $2 
    `, clientCouponId, clientId).Scan(
		&c.Created, &c.Life, &c.Tm, &c.Ru, &c.En,
		&bookingId,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, appresult.ErrNotFoundType(clientCouponId, "client coupon")
		}
		return nil, nil, err
	}

	if time.Now().After(c.Created.AddDate(0, 0, c.Life)) {
		return  &order.CouponData{}, nil, nil
	}
	return &c, bookingId, nil
}

func ApplyCoupon(ctx context.Context, r *repository, clientId, clientCouponId int, total float64) (*float64, error) {
	coupon, bookingId, err := FetchCoupon(ctx, r, clientCouponId, clientId)
	if err != nil {
		return nil, err
	}
	if bookingId != nil {
		return nil, appresult.ErrAlreadyCoupon(clientCouponId)
	}

	result := CalculateDiscount(total, coupon.Tm)
	return &result, nil
}

func ApplyCouponWithGeneral(ctx context.Context, r *repository, clientId int, clientCouponId int, rest *order.OrderOne) error {

	coupon, bookingId, err := FetchCoupon(ctx, r, clientCouponId, clientId)
	if err != nil {
		return err
	}

	if bookingId != nil {
		rest.Coupon = &order.DictionaryDTO{}
		rest.Coupon.Tm = coupon.Tm
		rest.Coupon.Ru = coupon.Ru
		rest.Coupon.En = coupon.En

		total := CalculateDiscount(rest.GeneralBill, coupon.En)

		if total != 0 {
			rest.ClientCouponId = &clientCouponId
			generalBill := math.Round(total*100) / 100
			rest.BillWithCoupon = &generalBill
		}
	}
	return nil
}

func FindItemsByBusinesses(
	ctx context.Context,
	r *repository,
	orderId int,
	businessesId int,
	baseURL string,
) ([]basket.Item, float64, int, error) {
	var (
		items      []basket.Item
		total      float64
		countItems int
	)

	rows, err := r.client.Query(ctx, `
        SELECT i.id, i.image_path, d.tm, d.en, d.ru, oi.price, oi.quantity
        FROM order_items oi
        JOIN items i ON oi.item_id = i.id
        JOIN dictionary d ON i.name_dictionary_id = d.id
        WHERE oi.order_id = $1 AND i.businesses_id = $2
    `, orderId, businessesId)
	if err != nil {
		return nil, 0, 0, appresult.ErrInternalServer
	}
	defer rows.Close()

	for rows.Next() {
		var item basket.Item
		if err := rows.Scan(
			&item.Id,
			&item.Images,
			&item.Name.Tm,
			&item.Name.En,
			&item.Name.Ru,
			&item.Value,
			&item.Count,
		); err != nil {
			return nil, 0, 0, appresult.ErrInternalServer
		}

		if baseURL != "" {
			item.Images = fmt.Sprintf("%s/%s", baseURL, strings.ReplaceAll(item.Images, "\\", "/"))
		}

		item.Value = math.Round(item.Value*100) / 100
		total += item.Value * float64(item.Count)
		countItems += item.Count
		items = append(items, item)
	}

	if len(items) == 0 {
		return nil, 0, 0, appresult.ErrNotFoundType(businessesId, "basket in businesses")
	}

	return items, total, countItems, nil
}

func (r *repository) GetAllForClient(ctx context.Context, clientId int, limitStr, offsetStr, status, search, baseURL string) (*order.OrderAllForClient, error) {
	var (
		orders []order.OrdersForClient
		count  int
		args   []interface{}
	)

	limitInt, offsetInt, err := utils.ParsePagination(limitStr, offsetStr)

	args = append(args, clientId)
	whereClause := " WHERE o.client_id = $1"
	argCount := 1

	if status != "" {
		argCount++
		whereClause += fmt.Sprintf(" AND o.status = $%d", argCount)
		args = append(args, status)
	}

	if search != "" {
		argCount++
		whereClause += fmt.Sprintf(" AND b.name ILIKE $%d", argCount)
		args = append(args, "%"+search+"%")
	}

	countQuery := `
        SELECT count(*) 
        FROM orders o 
        JOIN businesses b ON o.businesses_id = b.id 
        ` + whereClause

	err = r.client.QueryRow(ctx, countQuery, args...).Scan(&count)
	if err != nil {
		fmt.Println("error: ", err)
		return nil, err
	}

	args = append(args, limitInt, offsetInt)
	qRes := fmt.Sprintf(`
        SELECT 
            o.id, 
            b.name, 
            o.total_price,
            d.tm, d.ru, d.en,
            (SELECT COALESCE(SUM(quantity), 0) FROM order_items WHERE order_id = o.id) as count_items,
			status
        FROM orders o
        JOIN businesses b ON o.businesses_id = b.id
        LEFT JOIN businesses_coupons bc ON o.client_coupon_id = bc.id
        LEFT JOIN dictionary d ON bc.coupon_dictionary_id = d.id
        %s
        ORDER BY o.created_at DESC
        LIMIT $%d OFFSET $%d
    `, whereClause, argCount+1, argCount+2)

	rows, err := r.client.Query(ctx, qRes, args...)
	if err != nil {
		fmt.Println("error: ", err)
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var ord order.OrdersForClient
		var tm, ru, en *string

		err := rows.Scan(
			&ord.Id,
			&ord.BusinessesName,
			&ord.GeneralBill,
			&tm, &ru, &en,
			&ord.CountItems,
			&ord.Status,
		)
		if err != nil {
			fmt.Println("error: ", err)
			return nil, err
		}

		if tm != nil {
			ord.Coupon = &order.DictionaryDTO{
				Tm: *tm,
				Ru: *ru,
				En: *en,
			}
		}
		orders = append(orders, ord)
	}

	return &order.OrderAllForClient{
		Count:  count,
		Orders: orders,
	}, nil
}

func (r *repository) GetAllForBusinesses(ctx context.Context, businessesId, userId int, limitStr, offsetStr, status, baseURL string) (*order.OrderAllForBusinesses, error) {
	var (
		orders           []order.OrdersForBusinesses
		count            int
		args             []interface{}
		role             string
		userBusinessesId *int
	)

	err := r.client.QueryRow(ctx, `
        SELECT role, businesses_id FROM users WHERE id = $1
    `, userId).Scan(&role, &userBusinessesId)
	if err != nil {
		return nil, appresult.ErrInternalServer
	}

	if role == "MANAGER" {
		if userBusinessesId == nil || *userBusinessesId != businessesId {
			return nil, appresult.ErrForbidden
		}
	}

	limitInt, offsetInt, err := utils.ParsePagination(limitStr, offsetStr)

	args = append(args, businessesId)
	whereClause := " WHERE o.businesses_id = $1"
	argCount := 1

	if status != "" {
		argCount++
		whereClause += fmt.Sprintf(" AND o.status = $%d", argCount)
		args = append(args, status)
	}

	countQuery := `SELECT count(*) FROM orders o ` + whereClause
	err = r.client.QueryRow(ctx, countQuery, args...).Scan(&count)
	if err != nil {
		fmt.Println("error: ", err)
		return nil, err
	}

	args = append(args, limitInt, offsetInt)
	qRes := fmt.Sprintf(`
        SELECT 
            o.id, 
            o.total_price, 
            c.id, 
			CASE 
                WHEN c.last_name IS NOT NULL AND c.last_name != '' 
                THEN c.name || ' ' || c.last_name 
                ELSE c.name 
            END,
			COALESCE(c.image_path, ''), 
			c.phone_number,
            d.tm, d.ru, d.en,
            (SELECT COALESCE(SUM(quantity), 0) FROM order_items WHERE order_id = o.id) as count_items,
			o.status
        FROM orders o
        JOIN clients c ON o.client_id = c.id
        LEFT JOIN businesses_coupons bc ON o.client_coupon_id = bc.id
        LEFT JOIN dictionary d ON bc.coupon_dictionary_id = d.id
        %s
        ORDER BY o.created_at DESC
        LIMIT $%d OFFSET $%d
    `, whereClause, argCount+1, argCount+2)

	rows, err := r.client.Query(ctx, qRes, args...)
	if err != nil {
		fmt.Println("error: ", err)
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var ord order.OrdersForBusinesses
		var tm, ru, en *string

		err := rows.Scan(
			&ord.Id,
			&ord.GeneralBill,
			&ord.Client.Id,
			&ord.Client.FullName,
			&ord.Client.ImagePath,
			&ord.Client.PhoneNumber,
			&tm, &ru, &en,
			&ord.CountItems,
			&ord.Status,
		)
		if err != nil {
			fmt.Println("error: ", err)
			return nil, err
		}

		if ord.Client.ImagePath != "" && baseURL != "" {
			ord.Client.ImagePath = fmt.Sprintf("%s/%s", baseURL, strings.ReplaceAll(ord.Client.ImagePath, "\\", "/"))
		}

		if tm != nil {
			ord.Coupon = &order.DictionaryDTO{
				Tm: *tm,
				Ru: *ru,
				En: *en,
			}
		}

		ord.GeneralBill = math.Round(ord.GeneralBill*100) / 100
		orders = append(orders, ord)
	}

	return &order.OrderAllForBusinesses{
		Count:  count,
		Orders: orders,
	}, nil
}

func (r *repository) Update(
	ctx context.Context,
	clientID int,
	orderID int,
	dto order.UpdateOrderReq,
	baseURL string,
) (*order.OrderOne, error) {

	var (
		status string
	)
	err := r.client.QueryRow(ctx, `
		SELECT status FROM orders
		WHERE id = $1 AND client_id = $2
	`, orderID, clientID).Scan(&status)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, appresult.ErrNotFoundType(orderID, "order")
		}
		fmt.Println("error: ", err)
		return nil, appresult.ErrInternalServer
	}

	if status != "PENDING" {
		return nil, appresult.ErrStatus
	}

	orderTime, err := time.Parse("2006-01-02 15:04", dto.OrderTime)
	if err != nil {
		return nil, appresult.ErrTimee
	}

	tx, err := r.client.Begin(ctx)
	if err != nil {
		fmt.Println("error: ", err)
		return nil, err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `
		UPDATE orders
		SET place = $1,
		    order_time = $2,
		    client_coupon_id = $3,
		    updated_at = now()
		WHERE id = $4
	`, dto.Place, orderTime, dto.ClientCouponId, orderID)

	if err != nil {
		fmt.Println("error: ", err)
		return nil, appresult.ErrInternalServer
	}

	_, err = tx.Exec(ctx, `
		DELETE FROM order_items WHERE order_id = $1
	`, orderID)
	if err != nil {
		fmt.Println("error: ", err)
		return nil, appresult.ErrInternalServer
	}

	for _, item := range dto.Items {

		_, err = tx.Exec(ctx, `
			INSERT INTO order_items (order_id, item_id, quantity, price)
			SELECT $1, $2, $3, i.value
			FROM items i
			WHERE i.id = $2
		`, orderID, item.ItemID, item.Quantity)

		if err != nil {
			fmt.Println("error: ", err)
			return nil, appresult.ErrInternalServer
		}
	}

	_, err = tx.Exec(ctx, `
		UPDATE orders
		SET total_price = sub.total
		FROM (
			SELECT COALESCE(SUM(quantity * price), 0) AS total
			FROM order_items
			WHERE order_id = $1
		) sub
		WHERE id = $1
	`, orderID)

	if err != nil {
		fmt.Println("error: ", err)
		return nil, appresult.ErrInternalServer
	}

	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}

	return r.GetOne(ctx, orderID, baseURL)
}

func (r *repository) Delete(
	ctx context.Context,
	clientID int,
	orderID int,
) (int, int, error) {

	tx, err := r.client.Begin(ctx)
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback(ctx)

	var status string
	var businessesId int
	err = tx.QueryRow(ctx, `
		SELECT status, businesses_id
		FROM orders
		WHERE id = $1 AND client_id = $2
	`, orderID, clientID).Scan(&status, &businessesId)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, 0, appresult.ErrNotFoundType(orderID, "order")
		}
		return 0, 0, appresult.ErrInternalServer
	}

	if status != "PENDING" {
		return 0, 0, appresult.ErrStatus
	}

	_, err = tx.Exec(ctx, `
		DELETE FROM order_items WHERE order_id = $1
	`, orderID)
	if err != nil {
		return 0, 0, appresult.ErrInternalServer
	}

	_, err = tx.Exec(ctx, `
		DELETE FROM orders WHERE id = $1
	`, orderID)
	if err != nil {
		return 0, 0, appresult.ErrInternalServer
	}

	return businessesId, clientID, tx.Commit(ctx)
}

func (r *repository) UpdateStatusByClient(
	ctx context.Context,
	clientID int,
	orderID int,
	req order.UpdateOrderStatusReq,
) (int, int, error) {
	return r.updateOrderStatus(ctx, orderID, clientID, "client", req.Status, req.Reason)
}

func (r *repository) UpdateStatusByBusinesses(
	ctx context.Context,
	userID int,
	orderID int,
	req order.UpdateOrderStatusReq,
) (int, int, error) {
	return r.updateOrderStatus(ctx, orderID, userID, "businesses", req.Status, req.Reason)
}

func (r *repository) updateOrderStatus(
	ctx context.Context,
	orderID int,
	ownerID int,
	role string,
	newStatus string,
	reason string,
) (int, int, error) {

	var currentStatus string
	var clientId, businessesId int

	query := ""
	if role == "client" {
		query = `
				SELECT status, client_id, businesses_id
			FROM orders 
			WHERE id = $1 AND client_id = $2
		`
	} else {
		query = `
				SELECT o.status, o.client_id, o.businesses_id
			FROM orders o
			JOIN users u ON u.id = $2
			WHERE o.id = $1 AND o.businesses_id = u.businesses_id
		`
	}

	err := r.client.QueryRow(ctx, query, orderID, ownerID).Scan(&currentStatus, &clientId, &businessesId)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, 0, appresult.ErrNotFoundType(orderID, "order")
		}
		return 0, 0, appresult.ErrInternalServer
	}

	allowed := false
	if role == "client" {
		if currentStatus == "APPROVED" && newStatus == "COMPLETED_BY_CLIENT" {
			allowed = true
		}
		if currentStatus == "PENDING" && newStatus == "CANCELED_BY_CLIENT" {
			allowed = true
		}
	} else {
		if currentStatus == "PENDING" && newStatus == "APPROVED" {
			allowed = true
		}
		if currentStatus == "APPROVED" && newStatus == "COMPLETED_BY_BUSINESSES" {
			allowed = true
		}
		if currentStatus == "PENDING" && newStatus == "CANCELED_BY_BUSINESSES" {
			allowed = true
		}
	}

	if !allowed {
		return 0, 0, appresult.ErrStatus
	}

	if strings.HasPrefix(newStatus, "CANCELED") && reason == "" {
		return 0, 0, appresult.ErrReason
	}

	_, err = r.client.Exec(ctx, `
		UPDATE orders
		SET status=$1, reason=$2, updated_at=now()
		WHERE id=$3
	`, newStatus, reason, orderID)

	if err != nil {
		fmt.Println(err)
		return 0, 0, appresult.ErrInternalServer
	}

	return businessesId, clientId, nil
}
