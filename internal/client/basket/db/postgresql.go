package db

import (
	"context"
	"errors"
	"fmt"
	"math"
	"restaurants/internal/appresult"
	"restaurants/internal/client/basket"
	"restaurants/pkg/client/postgresql"
	"restaurants/pkg/logging"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v4"
)

type repository struct {
	client postgresql.Client
	logger *logging.Logger
}

func NewRepository(client postgresql.Client, logger *logging.Logger) basket.Repository {
	return &repository{
		client: client,
		logger: logger,
	}
}

func (r *repository) Create(ctx context.Context, userId int, item basket.BasketReq) error {
	var (
		basketId int
		exists   bool
	)

	err := r.client.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM items WHERE id=$1)`,
		item.ItemId,
	).Scan(&exists)

	if err != nil {
		fmt.Println("error:", err)
		return appresult.ErrInternalServer
	}

	if !exists {
		return appresult.ErrNotFoundType(item.ItemId, "item")
	}
	qSelect := `
			SELECT id
			FROM basket
			WHERE user_id = $1 AND item_id = $2;
		`
	err = r.client.QueryRow(ctx, qSelect, userId, item.ItemId).Scan(&basketId)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			qInsert := `
					INSERT INTO basket (user_id, item_id, count)
					VALUES ($1, $2, $3);
				`
			_, err = r.client.Exec(ctx, qInsert, userId, item.ItemId, 1)
			if err != nil {
				fmt.Println("error: ", err)
				return appresult.ErrInternalServer
			}
		} else {
			fmt.Println("error: ", err)
			return appresult.ErrInternalServer
		}
	} else {
		qUpdate := `
				UPDATE basket
				SET count = count + 1
				WHERE id = $1;
			`
		_, err = r.client.Exec(ctx, qUpdate, basketId)
		if err != nil {
			fmt.Println("error: ", err)
			return appresult.ErrInternalServer
		}
	}
	return nil
}

func (r *repository) GetAll(ctx context.Context, userId int, page string, size string, baseURL string) (*basket.BasketsAll, error) {
	var (
		baskets []basket.Baskets
		count   int
	)

	pageInt, err := strconv.Atoi(page)
	if err != nil || pageInt < 1 {
		pageInt = 1
	}

	sizeInt, err := strconv.Atoi(size)
	if err != nil || sizeInt < 1 {
		sizeInt = 10
	}
	offset := (pageInt - 1) * sizeInt

	q := `
		FROM businesses bs
		JOIN items i ON i.businesses_id = bs.id
		JOIN basket b ON b.item_id = i.id
		JOIN image_businesses img ON img.businesses_id = bs.id AND img.is_main = true
		WHERE b.user_id = $1
		`

	qCount := fmt.Sprintf(`SELECT count(DISTINCT bs.id) 
							%s
							`, q)

	err = r.client.QueryRow(ctx, qCount, userId).Scan(&count)
	if err != nil {
		fmt.Println("error: ", err)
		return nil, appresult.ErrInternalServer
	}

	qRes := fmt.Sprintf(`SELECT DISTINCT bs.id, bs.name, img.image_path
					     %s
						 LIMIT $2 OFFSET $3
						`, q)

	rows, err := r.client.Query(ctx, qRes, userId, sizeInt, offset)
	if err != nil {
		fmt.Println("error: ", err)
		return nil, appresult.ErrInternalServer
	}
	defer rows.Close()

	for rows.Next() {
		var (
			businesses basket.Businesses
		)
		if err := rows.Scan(
			&businesses.Id, &businesses.Name, &businesses.Image,
		); err != nil {
			fmt.Println("error: ", err)
			return nil, appresult.ErrInternalServer
		}

		cleanPath := strings.ReplaceAll(businesses.Image, "\\", "/")
		businesses.Image = fmt.Sprintf("%s/%s", baseURL, cleanPath)

		items, generalBill, generalCount, err := finditemsBybusinesses(r, ctx, userId, businesses.Id, baseURL)
		if err != nil {
			fmt.Println("error: ", err)
			return nil, appresult.ErrInternalServer
		}

		*generalBill = math.Round(*generalBill*100) / 100
		businesses.CountItems = *generalCount
		businesses.GeneralBill = *generalBill

		basketOne := basket.Baskets{
			Businesses: businesses,
			Items:      *items,
		}
		baskets = append(baskets, basketOne)
	}

	allBasket := basket.BasketsAll{
		Count:               count,
		BasketsByBusinesses: baskets,
	}

	return &allBasket, nil
}

func (r *repository) GetCount(ctx context.Context, userId, itemId int) (int, error) {
	var count int

	err := r.client.QueryRow(
		ctx,
		`SELECT count
		 FROM basket
		 WHERE user_id = $1 AND item_id = $2`,
		userId,
		itemId,
	).Scan(&count)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, nil
		}
		return 0, appresult.ErrInternalServer
	}

	return count, nil
}

func finditemsBybusinesses(r *repository, ctx context.Context, userId int, businessesId int, baseURL string) (*[]basket.Item, *float64, *int, error) {
	var (
		items                         []basket.Item
		generalBill                   float64
		generalCount, discountPercent int
	)
	qitem := `
		    SELECT i.id, i.image_path, d.tm, d.en, d.ru, i.value, b.count, i.discount_percent
		FROM basket b
		JOIN items i ON b.item_id = i.id
		JOIN businesses r ON i.businesses_id = r.id
		JOIN dictionary d ON i.name_dictionary_id = d.id
		WHERE b.user_id = $1 AND r.id = $2
		`

	rowsF, err := r.client.Query(ctx, qitem, userId, businessesId)
	if err != nil {
		fmt.Println("error: ", err)
		return nil, nil, nil, appresult.ErrInternalServer
	}
	defer rowsF.Close()

	for rowsF.Next() {
		var (
			item basket.Item
		)
		if err := rowsF.Scan(
			&item.Id, &item.Image, &item.Name.Tm, &item.Name.En, &item.Name.Ru, &item.Value, &item.Count, &discountPercent,
		); err != nil {
			fmt.Println("error: ", err)
			return nil, nil, nil, appresult.ErrInternalServer
		}

		if baseURL != "" {
			cleanPath := strings.ReplaceAll(item.Image, "\\", "/")
			item.Image = fmt.Sprintf("%s/%s", baseURL, cleanPath)
		}

		item.Value = math.Round(item.Value*100) / 100

		if item.Value != 0 && discountPercent != 0 {
			x := math.Round((float64(item.Value)*float64(discountPercent))/10) / 10
			discountValue := float64(item.Value) - x
			generalBill += discountValue * float64(item.Count)
			item.DiscountValue = &discountValue

		} else {
			generalBill += item.Value * float64(item.Count)
		}

		generalCount += item.Count
		items = append(items, item)
	}
	return &items, &generalBill, &generalCount, nil
}

func (r *repository) Delete(ctx context.Context, userId, itemId int) error {
	var (
		basketId int
		count    int
	)
	q := `		
			SELECT id, count
			FROM basket 
			WHERE user_id = $1 AND item_id = $2;
			`
	err := r.client.QueryRow(ctx, q, userId, itemId).Scan(&basketId, &count)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			fmt.Println("error: ", err)
			errr := fmt.Sprintf("basket with user_id = %d and item_id = %d", userId, itemId)
			return appresult.ErrNotFoundTypeStr(errr)
		} else {
			fmt.Println("error: ", err)
			return appresult.ErrInternalServer
		}
	}

	if count > 1 {
		qUpdate := `
				UPDATE basket
				SET count = count - 1
				WHERE id = $1;
			`
		_, err = r.client.Exec(ctx, qUpdate, basketId)
		if err != nil {
			return appresult.ErrInternalServer
		}
	} else {
		qDelete := `DELETE FROM basket WHERE id = $1`
		_, err = r.client.Exec(ctx, qDelete, basketId)
		if err != nil {
			fmt.Println("error: ", err)
			return appresult.ErrInternalServer
		}
	}
	return nil
}

func (r *repository) DeleteFullById(ctx context.Context, userId, itemId int) error {
	var basketId int
	q := `
        SELECT id
        FROM basket 
        WHERE user_id = $1 AND item_id = $2;
    `
	err := r.client.QueryRow(ctx, q, userId, itemId).Scan(&basketId)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			errStr := fmt.Sprintf("basket with user_id = %d and item_id = %d", userId, itemId)
			return appresult.ErrNotFoundTypeStr(errStr)
		}
		fmt.Println("error: ", err)
		return appresult.ErrInternalServer
	}

	qDelete := `DELETE FROM basket WHERE id = $1`
	_, err = r.client.Exec(ctx, qDelete, basketId)
	if err != nil {
		fmt.Println("error: ", err)
		return appresult.ErrInternalServer
	}

	return nil
}

func (r *repository) DeleteFull(ctx context.Context, userId int) error {
	var basketId int
	q := `
        SELECT id
        FROM basket 
        WHERE user_id = $1;
    `
	err := r.client.QueryRow(ctx, q, userId).Scan(&basketId)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			errStr := fmt.Sprintf("basket with user_id = %d", userId)
			return appresult.ErrNotFoundTypeStr(errStr)
		}
		fmt.Println("error: ", err)
		return appresult.ErrInternalServer
	}

	qDelete := `DELETE FROM basket WHERE user_id = $1`
	_, err = r.client.Exec(ctx, qDelete, userId)
	if err != nil {
		fmt.Println("error: ", err)
		return appresult.ErrInternalServer
	}

	return nil
}
