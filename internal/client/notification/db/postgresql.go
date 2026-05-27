package db

import (
	"context"
	"database/sql"
	"fmt"
	"restaurants/internal/appresult"
	"restaurants/internal/client/deviceToken"
	"restaurants/internal/client/notification"
	"restaurants/pkg/client/postgresql"
	"restaurants/pkg/fcm"
	"restaurants/pkg/logging"
	"strconv"
	"strings"
	"time"
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

func (r *Repository) CreateNotification(ctx context.Context, dto notification.NotificationReqDTO, userId int) (*fcm.NotificationDTO, error) {
	tx, err := r.client.Begin(ctx)
	if err != nil {
		fmt.Println("error begin tx:", err)
		return nil, appresult.ErrInternalServer
	}
	defer tx.Rollback(ctx)

	var (
		notificationId, titleId, contentId int
		exists, existsUser                 bool
	)
	query := `
			SELECT EXISTS (
				SELECT 1
				FROM restaurants
				WHERE id = $1
			)
		`
	err = tx.QueryRow(ctx, query, dto.RestaurantId).Scan(&exists)
	if err != nil {
		fmt.Println("error :", err)
		return nil, appresult.ErrInternalServer
	}
	if !exists {
		return nil, appresult.ErrNotFoundType(dto.RestaurantId, "restaurant")
	}

	query = `
			SELECT EXISTS (
				SELECT 1
				FROM users
				WHERE id = $1 AND ((role = 'admin' OR
				                   (role = 'manager' AND restaurant_id = $2)))
			)
		`
	err = tx.QueryRow(ctx, query, userId, dto.RestaurantId).Scan(&existsUser)
	if err != nil {
		fmt.Println("error :", err)
		return nil, appresult.ErrInternalServer
	}
	if !existsUser {
		return nil, appresult.ErrForbidden
	}

	q := `
				INSERT INTO dictionary (tm, en, ru)
				VALUES ($1, $2, $3)
				RETURNING id;
			`
	err = r.client.QueryRow(ctx, q, dto.Title.Tm, dto.Title.En, dto.Title.Ru).Scan(&titleId)
	if err != nil {
		fmt.Println("error: ", err)
		return nil, appresult.ErrInternalServer
	}

	err = r.client.QueryRow(ctx, q, dto.Content.Tm, dto.Content.En, dto.Content.Ru).Scan(&contentId)
	if err != nil {
		fmt.Println("error: ", err)
		return nil, appresult.ErrInternalServer
	}

	query = `
			INSERT INTO 
		notifications (
			restaurant_id, title_dictionary_id, content_dictionary_id, life
			) 
		VALUES ($1, $2, $3, $4) 
		RETURNING id
	`
	err = tx.QueryRow(ctx, query,
		dto.RestaurantId, titleId, contentId, dto.LifeDay).Scan(&notificationId)
	if err != nil {
		fmt.Println("error :", err)
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		fmt.Println("error :", err)
		return nil, err
	}

	res, err := r.GetNotificationById(ctx, notificationId)
	if err != nil {
		fmt.Println("error :", err)
		return nil, err
	}

	return res, nil
}

func (r *Repository) CreateClientNotification(
	ctx context.Context,
	tokens *[]deviceToken.DeviceTokens,
	notificationId int,
) error {

	for _, token := range *tokens {

		var clientId sql.NullInt32
		if token.ClientId != nil {
			clientId = sql.NullInt32{
				Int32: int32(*token.ClientId),
				Valid: true,
			}
		} else {
			clientId = sql.NullInt32{
				Valid: false,
			}
		}

		q := `
            INSERT INTO client_notifications (client_id, notification_id, device_token_id)
            VALUES ($1, $2, $3)
        `
		_, err := r.client.Exec(ctx, q, clientId, notificationId, token.Id)
		if err != nil {
			fmt.Println("Error inserting client notification, token ID:", token.Id, "error:", err)
			return appresult.ErrInternalServer
		}
	}

	return nil
}

func (r *Repository) GetNotificationById(ctx context.Context, notificationId int) (*fcm.NotificationDTO, error) {
	var (
		notification fcm.NotificationDTO
		createAt     time.Time
		exists       bool
	)

	query := `
			SELECT EXISTS (
				SELECT 1
				FROM notifications
				WHERE id = $1
			)
		`
	err := r.client.QueryRow(ctx, query, notificationId).Scan(&exists)
	if err != nil {
		fmt.Println("error :", err)
		return nil, appresult.ErrInternalServer
	}
	if !exists {
		return nil, appresult.ErrNotFoundType(notificationId, "notification")
	}

	query = `
			SELECT 
			n.id, 
			d_title.tm, d_title.ru, d_title.en, 
			d_content.tm, d_content.ru, d_content.en, 
			n.life, 
			n.restaurant_id,
			r.name,
			n.created_at
		FROM notifications n
		JOIN restaurants r ON r.id = n.restaurant_id
		JOIN dictionary d_title ON d_title.id = n.title_dictionary_id
		JOIN dictionary d_content ON d_content.id = n.content_dictionary_id
		WHERE n.id = $1
	`
	err = r.client.QueryRow(ctx, query, notificationId).Scan(
		&notification.Id,
		&notification.Title.Tm, &notification.Title.Ru, &notification.Title.En,
		&notification.Content.Tm, &notification.Content.Ru, &notification.Content.En,
		&notification.LifeDay,
		&notification.Restaurant.Id,
		&notification.Restaurant.Name,
		&createAt,
	)
	if err != nil {
		return nil, err
	}

	notification.CreatedAt = createAt.Format("2006-01-02 15:04")

	return &notification, nil
}

func (r *Repository) GetAll(ctx context.Context, filter notification.NotificationFilter) (*notification.Notifications, error) {
	var result notification.Notifications

	var (
		args    []interface{}
		filters = []string{"1=1"}
		idx     = 1
	)

	if filter.RestaurantId != "" {
		filters = append(filters, fmt.Sprintf("n.restaurant_id = $%d", idx))
		id, _ := strconv.Atoi(filter.RestaurantId)
		args = append(args, id)
		idx++
	}

	if filter.Life != "" {
		filters = append(filters, fmt.Sprintf("n.life = $%d", idx))
		val, _ := strconv.Atoi(filter.Life)
		args = append(args, val)
		idx++
	}

	if filter.Day != "" {
		filters = append(filters, fmt.Sprintf("DATE(n.created_at) = $%d", idx))
		day, _ := time.Parse("2006-01-02", filter.Day)
		args = append(args, day)
		idx++
	}

	limit, _ := strconv.Atoi(filter.Limit)
	offset, _ := strconv.Atoi(filter.Offset)

	if limit < 1 {
		limit = 10
	}
	if offset < 1 {
		offset = 1
	}
	offset = (offset - 1) * limit

	args = append(args, limit)
	limitIdx := idx
	idx++
	args = append(args, offset)
	offsetIdx := idx
	idx++

	query := fmt.Sprintf(`
		SELECT 
			n.id, 
			d_title.tm, d_title.ru, d_title.en, 
			d_content.tm, d_content.ru, d_content.en, 
			n.life, 
			n.restaurant_id,
			r.name,
			n.created_at
		FROM notifications n
		JOIN restaurants r ON r.id = n.restaurant_id
		JOIN dictionary d_title ON d_title.id = n.title_dictionary_id
		JOIN dictionary d_content ON d_content.id = n.content_dictionary_id
		WHERE %s
		ORDER BY n.created_at DESC
		LIMIT $%d OFFSET $%d
	`, strings.Join(filters, " AND "), limitIdx, offsetIdx)

	rows, err := r.client.Query(ctx, query, args...)
	if err != nil {
		return nil, appresult.ErrInternalServer
	}
	defer rows.Close()

	for rows.Next() {
		var (
			n        fcm.NotificationDTO
			createAt time.Time
		)

		if err := rows.Scan(
			&n.Id,
			&n.Title.Tm, &n.Title.Ru, &n.Title.En,
			&n.Content.Tm, &n.Content.Ru, &n.Content.En,
			&n.LifeDay,
			&n.Restaurant.Id,
			&n.Restaurant.Name,
			&createAt,
		); err != nil {
			fmt.Println("error: ", err)
			return nil, appresult.ErrInternalServer
		}

		n.CreatedAt = createAt.Format("2006-01-02 15:04")
		result.Notifications = append(result.Notifications, n)
	}

	countQuery := fmt.Sprintf(`
		SELECT COUNT(*)
		FROM notifications n
		JOIN restaurants r ON r.id = n.restaurant_id
		JOIN dictionary d_name ON d_name.id = r.name_dictionary_id
		JOIN dictionary d_title ON d_title.id = n.title_dictionary_id
		JOIN dictionary d_content ON d_content.id = n.content_dictionary_id
		WHERE %s
	`, strings.Join(filters, " AND "))

	var countArgs []interface{}
	for i := 0; i < len(args)-2; i++ {
		countArgs = append(countArgs, args[i])
	}

	if err := r.client.QueryRow(ctx, countQuery, countArgs...).Scan(&result.Count); err != nil {
		fmt.Println("error: ", err)
		return nil, appresult.ErrInternalServer
	}
	return &result, nil
}

func (r *Repository) GetAllByClient(ctx context.Context, clientId int, deviceToken string, filter notification.NotificationClientFilter) (*notification.NotificationsClient, error) {
	var result notification.NotificationsClient

	var (
		args    []interface{}
		filters = []string{"1=1"}
		idx     = 1
	)
	if clientId != -1 {
		filters = append(filters, fmt.Sprintf("cn.client_id = $%d", idx))
		args = append(args, clientId)
	} else {
		filters = append(filters, fmt.Sprintf("dt.token = $%d", idx))
		args = append(args, deviceToken)
	}
	idx++

	if filter.RestaurantId != "" {
		filters = append(filters, fmt.Sprintf("n.restaurant_id = $%d", idx))
		id, _ := strconv.Atoi(filter.RestaurantId)
		args = append(args, id)
		idx++
	}

	limit, _ := strconv.Atoi(filter.Limit)
	offset, _ := strconv.Atoi(filter.Offset)

	if limit < 1 {
		limit = 10
	}
	if offset < 1 {
		offset = 1
	}
	offset = (offset - 1) * limit

	args = append(args, limit)
	limitIdx := idx
	idx++
	args = append(args, offset)
	offsetIdx := idx
	idx++

	body := `
			FROM client_notifications cn
		JOIN notifications n ON n.id = cn.notification_id
		JOIN restaurants r ON r.id = n.restaurant_id
		JOIN dictionary d_title ON d_title.id = n.title_dictionary_id
		JOIN dictionary d_content ON d_content.id = n.content_dictionary_id
		JOIN device_token dt ON cn.device_token_id = dt.id
	`

	query := fmt.Sprintf(`
		SELECT 
			n.id, 
			d_title.tm, d_title.ru, d_title.en, 
			d_content.tm, d_content.ru, d_content.en, 
			n.life, 
			n.restaurant_id,
			r.name,
			n.created_at
			%s
		WHERE %s
		ORDER BY n.created_at ASC
		LIMIT $%d OFFSET $%d
	`, body, strings.Join(filters, " AND "), limitIdx, offsetIdx)

	rows, err := r.client.Query(ctx, query, args...)
	if err != nil {
		fmt.Println("error: ", err)
		return nil, appresult.ErrInternalServer
	}
	defer rows.Close()

	for rows.Next() {
		var (
			n        fcm.NotificationDTO
			createAt time.Time
		)

		if err := rows.Scan(
			&n.Id,
			&n.Title.Tm, &n.Title.Ru, &n.Title.En,
			&n.Content.Tm, &n.Content.Ru, &n.Content.En,
			&n.LifeDay,
			&n.Restaurant.Id,
			&n.Restaurant.Name,
			&createAt,
		); err != nil {
			fmt.Println("error: ", err)
			return nil, appresult.ErrInternalServer
		}

		n.CreatedAt = createAt.Format("2006-01-02 15:04")
		result.Notifications = append(result.Notifications, n)
	}

	countQuery := fmt.Sprintf(`
		SELECT COUNT(*)
		%s
		WHERE %s
	`, body, strings.Join(filters, " AND "))

	var countArgs []interface{}
	for i := 0; i < len(args)-2; i++ {
		countArgs = append(countArgs, args[i])
	}

	if err := r.client.QueryRow(ctx, countQuery, countArgs...).Scan(&result.Count); err != nil {
		fmt.Println("error: ", err)
		return nil, appresult.ErrInternalServer
	}

	if clientId != -1 {
		countQuery = fmt.Sprintf(`
        SELECT COUNT(*)
        %s
        WHERE cn.client_id = %d AND cn.read = FALSE
    `, body, clientId)

	} else {
		countQuery = fmt.Sprintf(`
        SELECT COUNT(*)
        %s
        WHERE dt.token = '%s' AND cn.read = FALSE
    `, body, deviceToken)
	}

	if err := r.client.QueryRow(ctx, countQuery).Scan(&result.UnreadCount); err != nil {
		fmt.Println("error: ", err)
		return nil, appresult.ErrInternalServer
	}

	return &result, nil
}

func FindTokens(ctx context.Context, r *Repository) ([]deviceToken.DeviceTokens, error) {
	var tokens []deviceToken.DeviceTokens
	query := `
        SELECT id, token, client_id
        FROM device_token
    `
	rows, err := r.client.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var token deviceToken.DeviceTokens
		if err := rows.Scan(&token.Id, &token.Token, &token.ClientId); err != nil {
			return nil, err
		}
		tokens = append(tokens, token)
	}

	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return tokens, nil
}

func (r *Repository) ReadClient(ctx context.Context, clientId int, deviceToken string) error {
	if clientId != -1 {
		var exists bool
		err := r.client.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM clients WHERE id = $1)`, clientId).Scan(&exists)
		if err != nil {
			fmt.Println("error:", err)
			return appresult.ErrInternalServer
		}
		if !exists {
			return appresult.ErrNotFoundType(clientId, "client")
		}
	}

	var deviceTokenId int
	if clientId == -1 && deviceToken != "" {
		err := r.client.QueryRow(ctx,
			`SELECT id FROM device_token WHERE token = $1`, deviceToken).Scan(&deviceTokenId)
		if err != nil {
			return appresult.ErrNotFoundTypeStr("device token = " + deviceToken)
		}
	}

	var (
		query string
		args  []interface{}
	)

	if clientId != -1 {
		query = `
			UPDATE client_notifications AS cn
			SET read = TRUE
			WHERE cn.read = FALSE
			  AND cn.client_id = $1
		`
		args = append(args, clientId)

	} else {
		query = `
			UPDATE client_notifications AS cn
			SET read = TRUE
			FROM device_token AS dt
			WHERE cn.read = FALSE
			  AND dt.id = cn.device_token_id
			  AND dt.token = $1
		`
		args = append(args, deviceToken)
	}

	_, err := r.client.Exec(ctx, query, args...)
	if err != nil {
		fmt.Println("update error:", err)
		return appresult.ErrInternalServer
	}

	return nil
}

func (r *Repository) Delete(ctx context.Context, notificationId, userId int) error {
	var (
		restaurantId int
		existsUser   bool
	)
	query := `
        SELECT restaurant_id
        FROM notifications
        WHERE id = $1
    `
	err := r.client.QueryRow(ctx, query, notificationId).Scan(&restaurantId)
	if err != nil {
		if err == sql.ErrNoRows {
			return appresult.ErrNotFoundType(notificationId, "notification")
		}
		return appresult.ErrInternalServer
	}

	query = `
			SELECT EXISTS (
				SELECT 1
				FROM users
				WHERE id = $1 AND ((role = 'admin' OR
				                   (role = 'manager' AND restaurant_id = $2)))
			)
		`
	err = r.client.QueryRow(ctx, query, userId, restaurantId).Scan(&existsUser)
	if err != nil {
		fmt.Println("error :", err)
		return appresult.ErrInternalServer
	}
	if !existsUser {
		return appresult.ErrForbidden
	}

	query = `
        DELETE FROM notifications
        WHERE id = $1
    `
	_, err = r.client.Exec(ctx, query, notificationId)
	if err != nil {
		return appresult.ErrInternalServer
	}
	return nil
}
