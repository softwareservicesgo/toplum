package db

import (
	"context"
	"errors"
	"fmt"
	"math"
	"restaurants/internal/appresult"
	"restaurants/internal/client/basket"
	"restaurants/internal/client/order"
	"restaurants/pkg/client/postgresql"
	"restaurants/pkg/logging"
	"restaurants/pkg/utils"
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

func (r *repository) Create(ctx context.Context, clientId int, req order.CreateOrderReq) (*[]int, error) {
	var orderIds []int

	tx, err := r.client.Begin(ctx)
	if err != nil {
		return nil, appresult.ErrInternalServer
	}
	defer tx.Rollback(ctx)

	parsedTime, err := time.Parse("2006-01-02 15:04", req.OrderTime)
	if err != nil {
		return nil, appresult.ErrInternalServer
	}

	for _, businessesId := range req.BusinessesIds {
		var (
			baskets []order.Basket
			total   float64
			orderId int
		)

		rows, err := tx.Query(ctx, `
			SELECT i.id, i.value, b.count
			FROM basket b
			JOIN items i ON b.item_id = i.id
			WHERE b.user_id = $1 AND i.businesses_id = $2
		`, clientId, businessesId)
		if err != nil {
			return nil, appresult.ErrNotFoundTypeStr("items in basket")
		}

		for rows.Next() {
			var basket order.Basket
			if err := rows.Scan(&basket.ItemId, &basket.Price, &basket.Count); err != nil {
				rows.Close()
				return nil, appresult.ErrInternalServer
			}
			total += basket.Price * float64(basket.Count)
			baskets = append(baskets, basket)
		}
		rows.Close()

		if err := rows.Err(); err != nil {
			return nil, appresult.ErrInternalServer
		}

		if len(baskets) == 0 {
			return nil, appresult.ErrNotFoundType(businessesId, "basket by businesses")
		}

		total = math.Round(total*100) / 100

		err = tx.QueryRow(ctx, `
			INSERT INTO orders (user_id, businesses_id, total_price, place, order_time)
			VALUES ($1, $2, $3, $4, $5)
			RETURNING id
		`, clientId, businessesId, total, req.Place, parsedTime).Scan(&orderId)
		if err != nil {
			return nil, appresult.ErrInternalServer
		}

		for _, basket := range baskets {
			_, err = tx.Exec(ctx, `
				INSERT INTO order_items (order_id, item_id, quantity, price)
				VALUES ($1, $2, $3, $4)
			`, orderId, basket.ItemId, basket.Count, basket.Price)
			if err != nil {
				return nil, appresult.ErrInternalServer
			}
		}

		_, err = tx.Exec(ctx, `
			DELETE FROM basket
			WHERE user_id = $1
			AND item_id IN (SELECT id FROM items WHERE businesses_id = $2)
		`, clientId, businessesId)
		if err != nil {
			return nil, appresult.ErrInternalServer
		}

		orderIds = append(orderIds, orderId)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, appresult.ErrInternalServer
	}

	return &orderIds, nil
}

func (r *repository) GetOne(
	ctx context.Context,
	orderId int,
	baseURL string,
) (*order.OrderOne, error) {

	var result order.OrderOne

	err := r.client.QueryRow(ctx, `
		SELECT b.id, b.name, img.image_path, o.status
		FROM orders o
		JOIN businesses b ON b.id = o.businesses_id
		JOIN image_businesses img ON img.businesses_id = b.id AND img.is_main = true
		WHERE o.id = $1
	`, orderId).Scan(
		&result.BusinessesId,
		&result.BusinessesName,
		&result.BusinessesImage,
		&result.Status,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, appresult.ErrNotFoundType(orderId, "order")
		}
		return nil, appresult.ErrInternalServer
	}

	if result.BusinessesImage != "" && baseURL != "" {
		cleanPath := strings.ReplaceAll(result.BusinessesImage, "\\", "/")
		result.BusinessesImage = fmt.Sprintf("%s/%s", baseURL, cleanPath)
	}

	result.Id = orderId

	items, total, countItems, err := FindItemsByOrder(ctx, r, orderId, baseURL)
	if err != nil {
		return nil, err
	}

	result.Items = items
	result.CountItems = countItems
	result.GeneralBill = math.Round(total*100) / 100

	return &result, nil
}

func FindItemsByOrder(
	ctx context.Context,
	r *repository,
	orderId int,
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
		WHERE oi.order_id = $1
	`, orderId)
	if err != nil {
		return nil, 0, 0, appresult.ErrInternalServer
	}
	defer rows.Close()

	for rows.Next() {
		var item basket.Item
		if err := rows.Scan(
			&item.Id,
			&item.Image,
			&item.Name.Tm,
			&item.Name.En,
			&item.Name.Ru,
			&item.Value,
			&item.Count,
		); err != nil {
			return nil, 0, 0, appresult.ErrInternalServer
		}

		if baseURL != "" {
			item.Image = fmt.Sprintf("%s/%s", baseURL, strings.ReplaceAll(item.Image, "\\", "/"))
		}

		item.Value = math.Round(item.Value*100) / 100
		total += item.Value * float64(item.Count)
		countItems += item.Count
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, 0, appresult.ErrInternalServer
	}

	if len(items) == 0 {
		return nil, 0, 0, appresult.ErrNotFoundType(orderId, "items in order")
	}

	return items, total, countItems, nil
}

func (r *repository) GetAllForClient(
	ctx context.Context,
	clientId int,
	limitStr, offsetStr, status, search, baseURL string,
) (*order.OrderAllForClient, error) {
	var (
		orders []order.OrderOne
		count  int
		args   []interface{}
	)

	limitInt, offsetInt, err := utils.ParsePagination(limitStr, offsetStr)
	if err != nil {
		fmt.Println("1error: ", err)
		return nil, err
	}

	args = append(args, clientId)
	whereClause := " WHERE o.user_id = $1"
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
		fmt.Println("2error: ", err)
		return nil, appresult.ErrInternalServer
	}

	if count == 0 {
		return &order.OrderAllForClient{
			Count:  0,
			Orders: []order.OrderOne{},
		}, nil
	}

	args = append(args, limitInt, offsetInt)
	qRes := fmt.Sprintf(`
		SELECT
			o.id,
			b.id,
			b.name,
            img.image_path,
			o.total_price,
			(SELECT COALESCE(SUM(quantity), 0) FROM order_items WHERE order_id = o.id) as count_items,
			o.status
		FROM orders o
		JOIN businesses b ON o.businesses_id = b.id
		JOIN image_businesses img ON img.businesses_id = b.id AND img.is_main = true
		%s
		ORDER BY o.created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argCount+1, argCount+2)

	rows, err := r.client.Query(ctx, qRes, args...)
	if err != nil {
		fmt.Println("error: ", err)
		return nil, appresult.ErrInternalServer
	}
	defer rows.Close()

	for rows.Next() {
		var ord order.OrderOne
		if err := rows.Scan(
			&ord.Id,
			&ord.BusinessesId,
			&ord.BusinessesName,
			&ord.BusinessesImage,
			&ord.GeneralBill,
			&ord.CountItems,
			&ord.Status,
		); err != nil {
			fmt.Println("3error: ", err)
			return nil, appresult.ErrInternalServer
		}

		if ord.BusinessesImage != "" && baseURL != "" {
			cleanPath := strings.ReplaceAll(ord.BusinessesImage, "\\", "/")
			ord.BusinessesImage = fmt.Sprintf("%s/%s", baseURL, cleanPath)
		}

		items, _, _, err := FindItemsByOrder(ctx, r, ord.Id, baseURL)
		if err != nil {
			return nil, err
		}

		ord.Items = items

		orders = append(orders, ord)
	}

	if err := rows.Err(); err != nil {
		fmt.Println("4error: ", err)
		return nil, appresult.ErrInternalServer
	}

	return &order.OrderAllForClient{
		Count:  count,
		Orders: orders,
	}, nil
}

func (r *repository) GetAllForBusinesses(
	ctx context.Context,
	businessesId, userId int,
	limitStr, offsetStr, status, baseURL string,
) (*order.OrderAllForBusinesses, error) {
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
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, appresult.ErrNotFoundType(userId, "user")
		}
		return nil, appresult.ErrInternalServer
	}

	if role == "MANAGER" {
		if userBusinessesId == nil || *userBusinessesId != businessesId {
			return nil, appresult.ErrForbidden
		}
	}

	limitInt, offsetInt, err := utils.ParsePagination(limitStr, offsetStr)
	if err != nil {
		return nil, err
	}

	args = append(args, businessesId)
	whereClause := " WHERE o.businesses_id = $1"
	argCount := 1

	if status != "" {
		argCount++
		whereClause += fmt.Sprintf(" AND o.status = $%d", argCount)
		args = append(args, status)
	}

	err = r.client.QueryRow(ctx,
		`SELECT count(*) FROM orders o `+whereClause,
		args...,
	).Scan(&count)
	if err != nil {
		return nil, appresult.ErrInternalServer
	}

	if count == 0 {
		return &order.OrderAllForBusinesses{
			Count:  0,
			Orders: []order.OrdersForBusinesses{},
		}, nil
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
			(SELECT COALESCE(SUM(quantity), 0) FROM order_items WHERE order_id = o.id) as count_items,
			o.status
		FROM orders o
		JOIN clients c ON o.user_id = c.id
		%s
		ORDER BY o.created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argCount+1, argCount+2)

	rows, err := r.client.Query(ctx, qRes, args...)
	if err != nil {
		return nil, appresult.ErrInternalServer
	}
	defer rows.Close()

	for rows.Next() {
		var ord order.OrdersForBusinesses
		if err := rows.Scan(
			&ord.Id,
			&ord.GeneralBill,
			&ord.Client.Id,
			&ord.Client.FullName,
			&ord.Client.ImagePath,
			&ord.Client.PhoneNumber,
			&ord.CountItems,
			&ord.Status,
		); err != nil {
			return nil, appresult.ErrInternalServer
		}

		if ord.Client.ImagePath != "" && baseURL != "" {
			ord.Client.ImagePath = fmt.Sprintf("%s/%s", baseURL, strings.ReplaceAll(ord.Client.ImagePath, "\\", "/"))
		}
		ord.GeneralBill = math.Round(ord.GeneralBill*100) / 100
		orders = append(orders, ord)
	}

	if err := rows.Err(); err != nil {
		return nil, appresult.ErrInternalServer
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
		WHERE id = $1 AND user_id = $2
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
		    updated_at = now()
		WHERE id = $3
	`, dto.Place, orderTime, orderID)

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
		WHERE id = $1 AND user_id = $2
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
				SELECT status, user_id, businesses_id
			FROM orders 
			WHERE id = $1 AND user_id = $2
		`
	} else {
		query = `
				SELECT o.status, o.user_id, o.businesses_id
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
