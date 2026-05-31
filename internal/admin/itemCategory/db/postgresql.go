package db

import (
	"context"
	"errors"
	"fmt"
	"restaurants/internal/admin/itemCategory"
	"restaurants/internal/appresult"
	"restaurants/pkg/client/postgresql"
	"restaurants/pkg/logging"
	"strconv"

	"github.com/jackc/pgx/v4"
)

type repository struct {
	client postgresql.Client
	logger *logging.Logger
}

func NewRepository(client postgresql.Client, logger *logging.Logger) itemCategory.Repository {
	return &repository{
		client: client,
		logger: logger,
	}
}

func (r *repository) Create(
	ctx context.Context,
	dto itemCategory.ItemCategoryCreateDTO,
) (*itemCategory.ItemCategoryDTO, error) {

	var (
		dictId     int
		itemCategoryId int
		exists     int
	)

	q := `
		SELECT ic.id
		FROM dictionary d
		JOIN item_categories ic ON ic.name_dictionary_id = d.id
		WHERE (d.tm = $1 OR d.en = $2 OR d.ru = $3)
		AND ic.businesses_id = $4;
	`
	err := r.client.QueryRow(ctx, q, dto.Name.Tm, dto.Name.En, dto.Name.Ru, dto.BusinessID).Scan(&itemCategoryId)

	if err == nil {
		fmt.Println("error: ", err)
		return nil, appresult.ErrAlreadyData("item_category")
	}

	if !errors.Is(err, pgx.ErrNoRows) {
		fmt.Println("error: ", err)
		return nil, appresult.ErrInternalServer
	}

	q = `SELECT 1 FROM businesses WHERE id = $1`
	err = r.client.QueryRow(ctx, q, dto.BusinessID).Scan(&exists)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, appresult.ErrNotFoundType(dto.BusinessID, "businesses")
		}
		fmt.Println("error: ", err)
		return nil, appresult.ErrInternalServer
	}

	insertDict := `
		INSERT INTO dictionary (tm, en, ru)
		VALUES ($1, $2, $3)
		RETURNING id;
	`

	err = r.client.QueryRow(ctx, insertDict,
		dto.Name.Tm, dto.Name.En, dto.Name.Ru,
	).Scan(&dictId)

	if err != nil {
		fmt.Println("error: ", err)
		return nil, appresult.ErrInternalServer
	}

	q = `
		INSERT INTO item_categories (name_dictionary_id, businesses_id)
		VALUES ($1, $2)
		RETURNING id;
	`
	err = r.client.QueryRow(ctx, q, dictId, dto.BusinessID).Scan(&itemCategoryId)

	if err != nil {
		fmt.Println("error: ", err)
		return nil, appresult.ErrInternalServer
	}

	return r.GetOne(ctx, itemCategoryId)
}

func (r *repository) GetOne(
	ctx context.Context,
	itemCategoryId int,
) (*itemCategory.ItemCategoryDTO, error) {

	var itemCategoryData itemCategory.ItemCategoryDTO

	q := `
		SELECT ic.id, d.tm, d.en, d.ru, ic.businesses_id
		FROM item_categories ic
		JOIN dictionary d ON ic.name_dictionary_id = d.id
		WHERE ic.id = $1;
	`

	err := r.client.QueryRow(ctx, q, itemCategoryId).Scan(
		&itemCategoryData.ID,
		&itemCategoryData.Name.Tm,
		&itemCategoryData.Name.En,
		&itemCategoryData.Name.Ru,
		&itemCategoryData.BusinessID,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			fmt.Println("error: ", err)
			return nil, appresult.ErrNotFoundType(itemCategoryId, "item_category")
		}
		return nil, appresult.ErrInternalServer
	}

	return &itemCategoryData, nil
}

func (r *repository) GetAll(
	ctx context.Context,
	businessId,
	search,
	limit,
	offset string,
) (*itemCategory.ItemCategoryAllDTO, error) {

	var result itemCategory.ItemCategoryAllDTO
	result.Categories = []itemCategory.ItemCategoryDTO{}

	offsetInt, err := strconv.Atoi(offset)
	if err != nil || offsetInt < 1 {
		offsetInt = 1
	}
	limitInt, err := strconv.Atoi(limit)
	if err != nil || limitInt < 1 {
		limitInt = 10
	}
	offsetInt = (offsetInt - 1) * limitInt

	businessIdInt, err := strconv.Atoi(businessId)
	if err != nil {
		businessIdInt = 0
	}

	q := `
		SELECT 
			ic.id,
			d.tm, d.ru, d.en,
			ic.businesses_id
		FROM item_categories ic
		JOIN dictionary d ON ic.name_dictionary_id = d.id
		WHERE 
			($1 = '' OR 
				d.tm ILIKE '%' || $1 || '%' OR 
				d.en ILIKE '%' || $1 || '%' OR 
				d.ru ILIKE '%' || $1 || '%'
			)
			AND ($2 = 0 OR ic.businesses_id = $2)
		ORDER BY ic.created_at DESC
		LIMIT $3 OFFSET $4;
	`

	rows, err := r.client.Query(
		ctx,
		q,
		search,
		businessIdInt,
		limitInt,
		offsetInt,
	)
	if err != nil {
		fmt.Println("error: ", err)
		return nil, appresult.ErrInternalServer
	}
	defer rows.Close()

	for rows.Next() {
		var c itemCategory.ItemCategoryDTO

		err := rows.Scan(
			&c.ID,
			&c.Name.Tm,
			&c.Name.Ru,
			&c.Name.En,
			&c.BusinessID,
		)
		if err != nil {
			fmt.Println("error: ", err)
			return nil, appresult.ErrInternalServer
		}
		result.Categories = append(result.Categories, c)
	}

	q = `
		SELECT count(*)
		FROM item_categories ic
		JOIN dictionary d ON ic.name_dictionary_id = d.id
		WHERE 
			($1 = '' OR 
				d.tm ILIKE '%' || $1 || '%' OR 
				d.en ILIKE '%' || $1 || '%' OR 
				d.ru ILIKE '%' || $1 || '%'
			)
			AND ($2 = 0 OR ic.businesses_id = $2);
	`

	err = r.client.QueryRow(ctx, q, search, businessIdInt).Scan(&result.Count)
	if err != nil {
		fmt.Println("error: ", err)
		return nil, appresult.ErrInternalServer
	}

	return &result, nil
}

func (r *repository) Update(
	ctx context.Context,
	itemCategoryId int,
	dto itemCategory.ItemCategoryNameDTO,
) (*itemCategory.ItemCategoryDTO, error) {

	var (
		dictId     int
		businessId int
	)

	tx, err := r.client.Begin(ctx)
	if err != nil {
		return nil, err
	}

	defer func() {
		if rbErr := tx.Rollback(ctx); rbErr != nil && !errors.Is(rbErr, pgx.ErrTxClosed) {
			fmt.Println("rollback error:", rbErr)
		}
	}()

	q := `
		SELECT name_dictionary_id, businesses_id
		FROM item_categories
		WHERE id = $1
	`
	err = tx.QueryRow(ctx, q, itemCategoryId).Scan(&dictId, &businessId)

	if err != nil {
		return nil, appresult.ErrNotFoundType(itemCategoryId, "item_category")
	}

	if dto.Name.En != "" && dto.Name.Ru != "" && dto.Name.Tm != "" {
		var existsId int

		checkQuery := `
			SELECT ic.id
			FROM item_categories ic
			JOIN dictionary d ON d.id = ic.name_dictionary_id
			WHERE ic.businesses_id = $1
			AND ic.id != $2
			AND (
				LOWER(d.tm) = LOWER($3)
				OR LOWER(d.en) = LOWER($4)
				OR LOWER(d.ru) = LOWER($5)
			)
		`

		err = tx.QueryRow(ctx, checkQuery, businessId, itemCategoryId, dto.Name.Tm, dto.Name.En, dto.Name.Ru).Scan(&existsId)
		if err == nil {
			return nil, appresult.ErrAlreadyData("item_category")
		}

		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, appresult.ErrInternalServer
		}

		_, err = tx.Exec(ctx,
			`UPDATE dictionary SET tm=$1, en=$2, ru=$3 WHERE id=$4`,
			dto.Name.Tm,
			dto.Name.En,
			dto.Name.Ru,
			dictId,
		)

		if err != nil {
			return nil, appresult.ErrInternalServer
		}
	}

	if err = tx.Commit(ctx); err != nil {
		return nil, appresult.ErrInternalServer
	}

	return r.GetOne(ctx, itemCategoryId)
}

func (r *repository) Delete(ctx context.Context, itemCategory int) error {
	var dictId int

	q := `SELECT name_dictionary_id FROM item_categories WHERE id = $1`
	err := r.client.QueryRow(ctx, q, itemCategory).Scan(&dictId)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return appresult.ErrNotFoundType(itemCategory, "item_category")
		}
		return appresult.ErrInternalServer
	}

	_, err = r.client.Exec(ctx, `DELETE FROM item_categories WHERE id = $1`, itemCategory)
	if err != nil {
		return appresult.ErrInternalServer
	}

	_, err = r.client.Exec(ctx, `DELETE FROM dictionary WHERE id = $1`, dictId)
	if err != nil {
		return appresult.ErrInternalServer
	}

	return nil
}
