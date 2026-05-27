package db

import (
	"context"
	"errors"
	"fmt"
	"math"
	"regexp"
	"restaurants/internal/appresult"
	"restaurants/internal/client/basket"
	"restaurants/pkg/client/postgresql"
	"restaurants/pkg/logging"
	"strconv"
	"strings"
	"time"

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

func (r *repository) Create(ctx context.Context, clientId int, item basket.BasketReq) error {
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
			WHERE client_id = $1 AND item_id = $2;
		`
	err = r.client.QueryRow(ctx, qSelect, clientId, item.ItemId).Scan(&basketId)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			qInsert := `
					INSERT INTO basket (client_id, item_id, count)
					VALUES ($1, $2, $3);
				`
			_, err = r.client.Exec(ctx, qInsert, clientId, item.ItemId, item.Count)
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
				SET count = count + $1
				WHERE id = $2;
			`
		_, err = r.client.Exec(ctx, qUpdate, item.Count, basketId)
		if err != nil {
			fmt.Println("error: ", err)
			return appresult.ErrInternalServer
		}
	}
	return nil
}

func (r *repository) GetOne(
	ctx context.Context,
	clientId int,
	businessesId int,
	clientCouponId string,
	baseURL string,
) (*basket.Basket, error) {

	var (
		result basket.Basket
	)

	err := r.client.QueryRow(ctx, `
		SELECT r.id, r.name
		FROM businesses r
		WHERE r.id = $1
	`, businessesId).Scan(
		&result.Businesses.Id,
		&result.Businesses.Name,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			fmt.Println("error: ", err)
			return nil, appresult.ErrNotFoundType(businessesId, "businesses")
		}
		fmt.Println("error: ", err)
		return nil, appresult.ErrInternalServer
	}

	itemss, total, countItems, err := FindItemsByBusinesses(ctx, r, clientId, businessesId, baseURL)
	result.Items = *itemss
	result.Businesses.CountItems = *countItems
	result.Businesses.GeneralBill = math.Round((*total)*100) / 100

	if clientCouponId != "" {
		if result.Businesses.Coupon == nil {
			result.Businesses.Coupon = &basket.DictionaryDTO{}
		}

		err = applyCoupon(ctx, r, clientId, clientCouponId, &result.Businesses)
		if err != nil {
			fmt.Println("error: ", err)
			return nil, err
		}
	}

	return &result, nil
}

func FindItemsByBusinesses(
	ctx context.Context,
	r *repository,
	clientId int,
	businessesId int,
	baseURL string,
) (*[]basket.Item, *float64, *int, error) {
	var (
		itemss     []basket.Item
		total      float64
		countitems int
	)
	rows, err := r.client.Query(ctx, `
		SELECT f.id, f.image_path, d.tm, d.en, d.ru, f.value, b.count
		FROM basket b
		JOIN items f ON b.item_id = f.id
		JOIN dictionary d ON f.name_dictionary_id = d.id
		WHERE b.client_id = $1 AND f.businesses_id = $2
	`, clientId, businessesId)
	if err != nil {
		fmt.Println("error: ", err)
		return nil, nil, nil, appresult.ErrInternalServer
	}
	defer rows.Close()

	for rows.Next() {
		var f basket.Item
		if err := rows.Scan(
			&f.Id,
			&f.Images,
			&f.Name.Tm,
			&f.Name.En,
			&f.Name.Ru,
			&f.Value,
			&f.Count,
		); err != nil {
			fmt.Println("error: ", err)
			return nil, nil, nil, appresult.ErrInternalServer
		}

		if baseURL != "" {
			f.Images = fmt.Sprintf("%s/%s", baseURL, strings.ReplaceAll(f.Images, "\\", "/"))
		}

		f.Value = math.Round(f.Value*100) / 100

		total += f.Value * float64(f.Count)
		countitems += f.Count
		itemss = append(itemss, f)
	}

	if len(itemss) == 0 {
		return nil, nil, nil, appresult.ErrNotFoundType(businessesId, "basket in businesses")
	}

	return &itemss, &total, &countitems, nil
}

func applyCoupon(
	ctx context.Context,
	r *repository,
	clientId int,
	clientCouponId string,
	rest *basket.Businesses) error {

	var (
		created time.Time
		life    int
	)

	id, _ := strconv.Atoi(clientCouponId)

	err := r.client.QueryRow(ctx, `
		SELECT cc.created_at, rc.life, d.en, d.ru, d.tm
		FROM client_coupons cc
		JOIN businesses_coupons rc ON rc.id = cc.businesses_coupon_id
		JOIN dictionary d ON d.id = rc.coupon_dictionary_id
		WHERE cc.id = $1 AND cc.client_id = $2 AND reservation_id IS NULL
	`, id, clientId).Scan(
		&created,
		&life,
		&rest.Coupon.En,
		&rest.Coupon.Ru,
		&rest.Coupon.Tm,
	)
	if err != nil {
		return appresult.ErrNotFoundType(id, "coupon")
	}

	if time.Now().After(created.AddDate(0, 0, life)) {
		return appresult.ErrExpiredCoupon(id)
	}

	total := rest.GeneralBill

	re := regexp.MustCompile(`\d+%`)
	match := re.FindString(rest.Coupon.En)

	if match != "" {
		p, _ := strconv.Atoi(strings.TrimSuffix(match, "%"))
		total -= total * float64(p) / 100
	} else {
		re := regexp.MustCompile(`(\d+(\.\d+)?)`)
		match := re.FindString(rest.Coupon.En)
		if match != "" {
			v, _ := strconv.Atoi(match)
			total -= float64(v)
		}
		if total < 0 {
			total = 0
		}
	}

	if total != 0 {
		rest.ClientCouponId = &id
		generalBill := math.Round(total*100) / 100
		rest.BillWithCoupon = &generalBill
	}
	return nil
}

func (r *repository) GetAll(ctx context.Context, clientId int, page string, size string, baseURL string) (*basket.BasketsAll, error) {
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
		FROM businesses r
		JOIN items f ON f.businesses_id = r.id
		JOIN basket b ON b.item_id = f.id
		WHERE b.client_id = $1
		`

	qCount := fmt.Sprintf(`SELECT count(*) 
							%s
							`, q)

	err = r.client.QueryRow(ctx, qCount, clientId).Scan(&count)
	if err != nil {
		fmt.Println("error: ", err)
		return nil, appresult.ErrInternalServer
	}

	qRes := fmt.Sprintf(`SELECT r.id, r.name
					     %s
						 LIMIT $2 OFFSET $3
						`, q)

	rows, err := r.client.Query(ctx, qRes, clientId, sizeInt, offset)
	if err != nil {
		fmt.Println("error: ", err)
		return nil, appresult.ErrInternalServer
	}
	defer rows.Close()

	for rows.Next() {
		var (
			rest basket.Businessess
		)
		if err := rows.Scan(
			&rest.Id, &rest.Name,
		); err != nil {
			fmt.Println("error: ", err)
			return nil, appresult.ErrInternalServer
		}

		items, generalBill, err := finditemsBybusinesses(r, ctx, clientId, rest.Id, baseURL)
		if err != nil {
			fmt.Println("error: ", err)
			return nil, appresult.ErrInternalServer
		}
		*generalBill = math.Round(*generalBill*100) / 100
		rest.CountItems = len(*items)
		rest.GeneralBill = *generalBill

		basketOne := basket.Baskets{
			Businesses: rest,
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

func finditemsBybusinesses(r *repository, ctx context.Context, clientId int, restId int, baseURL string) (*[]basket.Item, *float64, error) {
	var (
		items       []basket.Item
		generalBill float64
	)
	qitem := `
		    SELECT f.id, f.image_path, d.tm, d.en, d.ru, f.value, b.count
		FROM basket b
		JOIN items f ON b.item_id = f.id
		JOIN businesses r ON f.businesses_id = r.id
		JOIN dictionary d ON f.name_dictionary_id = d.id
		WHERE b.client_id = $1 AND r.id = $2
		`

	rowsF, err := r.client.Query(ctx, qitem, clientId, restId)
	if err != nil {
		fmt.Println("error: ", err)
		return nil, nil, appresult.ErrInternalServer
	}
	defer rowsF.Close()

	for rowsF.Next() {
		var (
			f basket.Item
		)
		if err := rowsF.Scan(
			&f.Id, &f.Images, &f.Name.Tm, &f.Name.En, &f.Name.Ru, &f.Value, &f.Count,
		); err != nil {
			fmt.Println("error: ", err)
			return nil, nil, appresult.ErrInternalServer
		}
		if baseURL != "" {
			cleanPath := strings.ReplaceAll(f.Images, "\\", "/")
			f.Images = fmt.Sprintf("%s/%s", baseURL, cleanPath)
		}
		f.Value = math.Round(f.Value*100) / 100

		generalBill += f.Value * float64(f.Count)
		items = append(items, f)
	}
	return &items, &generalBill, nil
}

func (r *repository) Delete(ctx context.Context, clientId, itemId int) error {
	var (
		basketId int
		count    int
	)
	q := `		
			SELECT id, count
			FROM basket 
			WHERE client_id = $1 AND item_id = $2;
			`
	err := r.client.QueryRow(ctx, q, clientId, itemId).Scan(&basketId, &count)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			fmt.Println("error: ", err)
			errr := fmt.Sprintf("basket with client_is = %d and item_id = %d", clientId, itemId)
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
