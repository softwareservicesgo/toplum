package db

import (
	"context"
	"errors"
	"fmt"
	"math"
	"restaurants/internal/appresult"
	"restaurants/internal/client/basket"
	"restaurants/internal/client/order"
	"restaurants/internal/client/user"
	"restaurants/internal/enum"
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

const (
	formatDayTime = "2006-01-02 15:04"
)

func (r *repository) Create(ctx context.Context, clientId int, req order.CreateOrderReq) (*[]int, *[]int, error) {
	var (
		orderIds, outStockInBuisnessesId []int
	)

	tx, err := r.client.Begin(ctx)
	if err != nil {
		return nil, nil, appresult.ErrInternalServer
	}
	defer tx.Rollback(ctx)

	parsedTime, err := time.Parse("2006-01-02 15:04", req.OrderTime)
	if err != nil {
		return nil, nil, appresult.ErrInternalServer
	}

	for _, businessesId := range req.BusinessesIds {
		var (
			baskets    []order.Basket
			total      float64
			orderId    int
			stock      *int
			isNotStock bool
		)

		rows, err := tx.Query(ctx, `
			SELECT i.id, i.value, b.count, i.stock, i.discount_percent
			FROM basket b
			JOIN items i ON b.item_id = i.id
			WHERE b.user_id = $1 AND i.businesses_id = $2
		`, clientId, businessesId)
		if err != nil {
			return nil, nil, appresult.ErrNotFoundTypeStr("items in basket")
		}
		defer rows.Close()

		for rows.Next() {
			var basket order.Basket
			if err := rows.Scan(&basket.ItemId, &basket.Price, &basket.Count, &stock, &basket.DiscountPercent); err != nil {
				return nil, nil, appresult.ErrInternalServer
			}

			if stock != nil && basket.Count > *stock {
				isNotStock = true
			} else {

				if basket.Price != 0 && *basket.DiscountPercent != 0 {
					x := math.Round((float64(basket.Price)*float64(*basket.DiscountPercent))/10) / 10
					discountValue := float64(basket.Price) - x

					total += discountValue * float64(basket.Count)
				} else {
					total += basket.Price * float64(basket.Count)
				}

				baskets = append(baskets, basket)
			}
		}

		if err := rows.Err(); err != nil {
			return nil, nil, appresult.ErrInternalServer
		}

		if isNotStock {
			outStockInBuisnessesId = append(outStockInBuisnessesId, businessesId)
		} else {
			if len(baskets) == 0 {
				return nil, nil, appresult.ErrNotFoundType(businessesId, "basket by businesses")
			}

			total = math.Round(total*100) / 100

			err = tx.QueryRow(ctx, `
			INSERT INTO orders (user_id, businesses_id, total_price, place, order_time)
			VALUES ($1, $2, $3, $4, $5)
			RETURNING id
		`, clientId, businessesId, total, req.Place, parsedTime).Scan(&orderId)
			if err != nil {
				return nil, nil, appresult.ErrInternalServer
			}

			for _, basket := range baskets {
				_, err = tx.Exec(ctx, `
				INSERT INTO order_items (order_id, item_id, quantity, price, discount_percent)
				VALUES ($1, $2, $3, $4, $5)
			`, orderId, basket.ItemId, basket.Count, basket.Price, basket.DiscountPercent)
				if err != nil {
					return nil, nil, appresult.ErrInternalServer
				}

				query := `
					UPDATE items
					SET stock = stock - $1
					WHERE id = $2 AND stock IS NOT NULL
				`
				_, err := tx.Exec(ctx, query, basket.Count, basket.ItemId)
				if err != nil {
					return nil, nil, appresult.ErrInternalServer
				}
			}

			_, err = tx.Exec(ctx, `
			DELETE FROM basket
			WHERE user_id = $1
			AND item_id IN (SELECT id FROM items WHERE businesses_id = $2)
		`, clientId, businessesId)
			if err != nil {
				return nil, nil, appresult.ErrInternalServer
			}

			orderIds = append(orderIds, orderId)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, nil, appresult.ErrInternalServer
	}

	return &orderIds, &outStockInBuisnessesId, nil
}

func (r *repository) GetOne(
	ctx context.Context,
	orderId int,
	baseURL string,
) (*order.OrderOne, error) {

	var (
		result       order.OrderOne
		orderTime    time.Time
		approvedById *int
	)

	err := r.client.QueryRow(ctx, `
		SELECT b.id, b.name, img.image_path, o.status, o.place, o.order_time, o.total_price, o.approved_by_id
		FROM orders o
		JOIN businesses b ON b.id = o.businesses_id
		JOIN image_businesses img ON img.businesses_id = b.id AND img.is_main = true
		WHERE o.id = $1
	`, orderId).Scan(
		&result.BusinessesId,
		&result.BusinessesName,
		&result.BusinessesImage,
		&result.Status,
		&result.Place,
		&orderTime,
		&result.GeneralBill,
		&approvedById,
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

	result.GeneralBill = math.Round(result.GeneralBill*10) / 10

	result.OrderTime = orderTime.Format(formatDayTime)
	result.Id = orderId
	items, countItems, err := FindItemsByOrder(ctx, r, orderId, baseURL)
	if err != nil {
		fmt.Println("error2: ", err)
		return nil, err
	}

	result.Items = items
	result.CountItems = countItems

	if approvedById != nil {
		var (
			id       int
			fullName string
		)
		err = r.client.QueryRow(ctx, `
		SELECT id,
		       CONCAT_WS(' ', name, last_name)
		FROM users
		WHERE id = $1
	`, *approvedById).Scan(
			&id,
			&fullName,
		)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, appresult.ErrNotFoundType(*approvedById, "user")
			}
			return nil, appresult.ErrInternalServer
		}
		result.ApprovedBy = &user.User{
			Id:       id,
			FullName: fullName,
		}
	}

	return &result, nil
}

func FindItemsByOrder(
	ctx context.Context,
	r *repository,
	orderId int,
	baseURL string,
) ([]basket.Item, int, error) {
	var (
		items                       []basket.Item
		countItems, discountPercent int
	)

	rows, err := r.client.Query(ctx, `
		SELECT i.id, i.image_path, d.tm, d.en, d.ru, oi.price, oi.quantity, oi.discount_percent
		FROM order_items oi
		JOIN items i ON oi.item_id = i.id
		JOIN dictionary d ON i.name_dictionary_id = d.id
		WHERE oi.order_id = $1
	`, orderId)
	if err != nil {
		return nil, 0, appresult.ErrInternalServer
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
			&discountPercent,
		); err != nil {
			return nil, 0, appresult.ErrInternalServer
		}

		if baseURL != "" {
			item.Image = fmt.Sprintf("%s/%s", baseURL, strings.ReplaceAll(item.Image, "\\", "/"))
		}

		item.Value = math.Round(item.Value*100) / 100

		if item.Value != 0 && discountPercent != 0 {
			x := math.Round((float64(item.Value)*float64(discountPercent))/10) / 10
			discountValue := float64(item.Value) - x

			item.DiscountValue = &discountValue
		}

		countItems += item.Count
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, appresult.ErrInternalServer
	}

	if len(items) == 0 {
		return nil, 0, appresult.ErrNotFoundType(orderId, "items in order")
	}

	return items, countItems, nil
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
			o.status,
			o.place,
			o.order_time
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
		var (
			ord       order.OrderOne
			orderTime time.Time
		)
		if err := rows.Scan(
			&ord.Id,
			&ord.BusinessesId,
			&ord.BusinessesName,
			&ord.BusinessesImage,
			&ord.GeneralBill,
			&ord.CountItems,
			&ord.Status,
			&ord.Place,
			&orderTime,
		); err != nil {
			fmt.Println("error: ", err)
			return nil, appresult.ErrInternalServer
		}

		if ord.BusinessesImage != "" && baseURL != "" {
			cleanPath := strings.ReplaceAll(ord.BusinessesImage, "\\", "/")
			ord.BusinessesImage = fmt.Sprintf("%s/%s", baseURL, cleanPath)
		}
		ord.OrderTime = orderTime.Format(formatDayTime)
		items, _, err := FindItemsByOrder(ctx, r, ord.Id, baseURL)
		if err != nil {
			return nil, err
		}

		ord.GeneralBill = math.Round(ord.GeneralBill*10) / 10
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
		orders []order.OrdersForBusinesses
		count  int
		args   []interface{}
	)

	limitInt, offsetInt, err := utils.ParsePagination(limitStr, offsetStr)
	if err != nil {
		fmt.Println("error: ", err)
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
		fmt.Println("error: ", err)
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
			u.id,
			CASE
				WHEN u.last_name IS NOT NULL AND u.last_name != ''
				THEN u.name || ' ' || u.last_name
				ELSE u.name
			END,
			COALESCE(u.image_path, ''),
			u.phone_number,
			(SELECT COALESCE(SUM(quantity), 0) FROM order_items WHERE order_id = o.id) as count_items,
			o.status
		FROM orders o
		JOIN users u ON o.user_id = u.id
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
			fmt.Println("error: ", err)
			return nil, appresult.ErrInternalServer
		}

		if ord.Client.ImagePath != "" && baseURL != "" {
			ord.Client.ImagePath = fmt.Sprintf("%s/%s", baseURL, strings.ReplaceAll(ord.Client.ImagePath, "\\", "/"))
		}
		ord.GeneralBill = math.Round(ord.GeneralBill*10) / 10
		orders = append(orders, ord)
	}

	if err := rows.Err(); err != nil {
		fmt.Println("error: ", err)
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
	tx, err := r.client.Begin(ctx)
	if err != nil {
		fmt.Println("error1: ", err)
		return nil, err
	}
	defer tx.Rollback(ctx)

	var (
		status        string
		total         float64
		existingItems []order.UpdateItem
	)
	err = tx.QueryRow(ctx, `
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

	rows, err := tx.Query(ctx, `
		SELECT item_id, quantity
		FROM order_items
		WHERE order_id = $1
	`, orderID)
	if err != nil {
		fmt.Println("error: ", err)
		return nil, appresult.ErrInternalServer
	}

	for rows.Next() {
		var oi order.UpdateItem
		if err = rows.Scan(&oi.ItemID, &oi.Quantity); err != nil {
			rows.Close()
			return nil, appresult.ErrInternalServer
		}
		existingItems = append(existingItems, oi)
	}
	rows.Close()

	if err = rows.Err(); err != nil {
		return nil, appresult.ErrInternalServer
	}

	for _, oi := range existingItems {
		_, err = tx.Exec(ctx, `
        UPDATE items
        SET stock = stock + $1
        WHERE id = $2
        AND stock IS NOT NULL
    `, oi.Quantity, oi.ItemID)
		if err != nil {
			return nil, appresult.ErrInternalServer
		}
	}

	_, err = tx.Exec(ctx, `
		DELETE FROM order_items WHERE order_id = $1
	`, orderID)
	if err != nil {
		fmt.Println("error: ", err)
		return nil, appresult.ErrInternalServer
	}

	for _, item := range dto.Items {
		var (
			discountPercent int
			value           float64
			stock           *int
		)

		err := tx.QueryRow(ctx, `
			SELECT value, discount_percent, stock
			FROM items
			WHERE id = $1
		`, item.ItemID).Scan(
			&value,
			&discountPercent,
			&stock,
		)

		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, appresult.ErrNotFoundType(item.ItemID, "item")
			}
			fmt.Println("error: ", err)
			return nil, appresult.ErrInternalServer
		}

		if stock != nil {
			res, err := tx.Exec(ctx, `
			UPDATE items
			SET stock = stock - $1
			WHERE id = $2 AND stock IS NOT NULL AND stock >= $1
		`, item.Quantity, item.ItemID)

			if err != nil {
				fmt.Println("error: ", err)
				return nil, appresult.ErrInternalServer
			}

			if res.RowsAffected() == 0 {
				return nil, appresult.ErrStock(item.ItemID)
			}
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO order_items (order_id, item_id, quantity, price, discount_percent)
			VALUES ( $1, $2, $3, $4, $5 )
		`, orderID, item.ItemID, item.Quantity, value, discountPercent)

		if err != nil {
			fmt.Println("error: ", err)
			return nil, appresult.ErrInternalServer
		}

		if value != 0 && discountPercent != 0 {
			x := math.Round((float64(value)*float64(discountPercent))/10) / 10
			discountValue := float64(value) - x
			total += discountValue * float64(item.Quantity)
		} else {
			total += value * float64(item.Quantity)
		}
	}

	_, err = tx.Exec(ctx, `
		UPDATE orders
		SET place = $1,
		    order_time = $2,
			total_price = $3,
		    updated_at = now()
		WHERE id = $4
	`, dto.Place, orderTime, total, orderID)

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
) (*int, *int, error) {

	tx, err := r.client.Begin(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback(ctx)

	var (
		status        string
		businessesId  int
		existingItems []order.UpdateItem
	)

	err = tx.QueryRow(ctx, `
		SELECT status, businesses_id
		FROM orders
		WHERE id = $1 AND user_id = $2
	`, orderID, clientID).Scan(&status, &businessesId)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, appresult.ErrNotFoundType(orderID, "order")
		}
		return nil, nil, appresult.ErrInternalServer
	}

	if status != "PENDING" {
		return nil, nil, appresult.ErrStatus
	}

	rows, err := tx.Query(ctx, `
		SELECT item_id, quantity
		FROM order_items
		WHERE order_id = $1
	`, orderID)

	for rows.Next() {
		var oi order.UpdateItem
		if err = rows.Scan(&oi.ItemID, &oi.Quantity); err != nil {
			rows.Close()
			return nil, nil, appresult.ErrInternalServer
		}
		existingItems = append(existingItems, oi)
	}
	rows.Close()

	if err = rows.Err(); err != nil {
		return nil, nil, appresult.ErrInternalServer
	}

	for _, oi := range existingItems {
		_, err = tx.Exec(ctx, `
        UPDATE items
        SET stock = stock + $1
        WHERE id = $2
		AND stock IS NOT NULL
    `, oi.Quantity, oi.ItemID)
		if err != nil {
			return nil, nil, appresult.ErrInternalServer
		}
	}

	_, err = tx.Exec(ctx, `
		DELETE FROM order_items WHERE order_id = $1
	`, orderID)
	if err != nil {
		return nil, nil, appresult.ErrInternalServer
	}

	_, err = tx.Exec(ctx, `
		DELETE FROM orders WHERE id = $1
	`, orderID)
	if err != nil {
		return nil, nil, appresult.ErrInternalServer
	}

	return &businessesId, &clientID, tx.Commit(ctx)
}

func (r *repository) UpdateStatusByClient(
	ctx context.Context,
	clientID int,
	orderID int,
	req order.UpdateOrderStatusReq,
) (*int, error) {
	var exist bool

	query := `
		SELECT EXISTS (
			SELECT 1
			FROM orders
			WHERE id = $1
			  AND user_id = $2
		)
	`

	err := r.client.QueryRow(ctx, query, orderID, clientID).Scan(&exist)
	if err != nil {
		fmt.Println("error:", err)
		return nil, appresult.ErrInternalServer
	}

	if !exist {
		return nil, appresult.ErrNotFoundType(clientID, " for user")
	}

	return r.updateOrderStatus(ctx, nil, orderID, "client", req.Status, req.Reason)
}

func (r *repository) UpdateStatusByBusinesses(
	ctx context.Context,
	userId int,
	orderID int,
	req order.UpdateOrderStatusReq,
) (*int, error) {
	return r.updateOrderStatus(ctx, &userId, orderID, "businesses", req.Status, req.Reason)
}

func (r *repository) updateOrderStatus(
	ctx context.Context,
	userID *int,
	orderID int,
	role string,
	newStatus string,
	reason string,
) (*int, error) {

	var currentStatus string
	var clientId int

	query := `
				SELECT status, user_id
			FROM orders 
			WHERE id = $1
		`

	err := r.client.QueryRow(ctx, query, orderID).Scan(&currentStatus, &clientId)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, appresult.ErrNotFoundType(orderID, "order")
		}
		fmt.Println("error: ", err)
		return nil, appresult.ErrInternalServer
	}

	allowed := false
	if role == "client" {
		if currentStatus == enum.APPROVED && newStatus == enum.COMPLETED_BY_CLIENT {
			allowed = true
		}
		if currentStatus == enum.PENDING && newStatus == enum.CANCELED_BY_CLIENT {
			allowed = true
		}
	} else {
		if currentStatus == enum.PENDING && newStatus == enum.APPROVED {
			allowed = true
		}
		if currentStatus == enum.APPROVED && newStatus == enum.COMPLETED_BY_BUSINESSES {
			allowed = true
		}
		if currentStatus == enum.PENDING && newStatus == enum.CANCELED_BY_BUSINESSES {
			allowed = true
		}
	}

	if !allowed {
		return nil, appresult.ErrStatus
	}

	if strings.HasPrefix(newStatus, enum.CANCELED) {
		if reason == "" {
			return nil, appresult.ErrRequired("reason")
		} else if len(reason) > 150 {
			return nil, appresult.ErrOverLimit(150, "reason")
		}
	}

	q := `
		UPDATE orders
			SET status=$1, reason=$2, updated_at=now() %s
			WHERE id=$3
		`

	if newStatus == enum.APPROVED && userID != nil {
		x := fmt.Sprintf(", approved_by_id = %d", *userID)
		q = fmt.Sprintf(q, x)
	} else {
		q = fmt.Sprintf(q, "")
	}

	_, err = r.client.Exec(ctx, q, newStatus, reason, orderID)

	if err != nil {
		fmt.Println("error: ", err)
		return nil, appresult.ErrInternalServer
	}

	return &clientId, nil
}

func (r *repository) GetBusinessesById(
	ctx context.Context,
	orderId int,
) (*int, error) {

	var businessesId int

	q := `
		SELECT businesses_id
		FROM orders
		WHERE id = $1;
	`

	err := r.client.QueryRow(ctx, q, orderId).Scan(&businessesId)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			fmt.Println("error: ", err)
			return nil, appresult.ErrNotFoundType(orderId, "order")
		}
		return nil, appresult.ErrInternalServer
	}

	return &businessesId, nil
}
