package db

import (
	"context"
	"errors"
	"fmt"
	"restaurants/internal/admin/category"
	"restaurants/internal/appresult"
	"restaurants/pkg/client/postgresql"
	"restaurants/pkg/logging"
	"restaurants/pkg/utils"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v4"
)

type repository struct {
	client postgresql.Client
	logger *logging.Logger
}

func NewRepository(client postgresql.Client, logger *logging.Logger) category.Repository {
	return &repository{
		client: client,
		logger: logger,
	}
}

func (r *repository) Create(ctx context.Context, dto category.CategoryReqDTO, imagePath string, baseURL string) (*category.CategoryDTO, error) {

	var (
		resp       *category.CategoryDTO
		id         int
		categoryId int
	)

	q := `
		SELECT d.id
		FROM dictionary d
		JOIN categories c ON c.name_dictionary_id = d.id
		WHERE d.tm = $1 OR d.en = $2 OR d.ru = $3
	`

	err := r.client.QueryRow(ctx, q, dto.Name.Tm, dto.Name.En, dto.Name.Ru).Scan(&id)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {

			insertDict := `
				INSERT INTO dictionary (tm, en, ru)
				VALUES ($1,$2,$3)
				RETURNING id
			`

			err = r.client.QueryRow(ctx, insertDict, dto.Name.Tm, dto.Name.En, dto.Name.Ru).Scan(&id)
			if err != nil {
				fmt.Println("error: ", err)
				return nil, appresult.ErrInternalServer
			}

			hasDetail := false
			if dto.HasDetail != nil {
				hasDetail = *dto.HasDetail
			}

			insertCategory := `
				INSERT INTO categories (name_dictionary_id, image_path, has_detail)
				VALUES ($1, $2, $3)
				RETURNING id
			`

			err = r.client.QueryRow(ctx, insertCategory, id, imagePath, hasDetail).Scan(&categoryId)
			if err != nil {
				fmt.Println("error: ", err)
				return nil, appresult.ErrInternalServer
			}

			resp, err = r.GetOne(ctx, categoryId, baseURL)
			if err != nil {
				fmt.Println("error: ", err)
				return nil, err
			}

			return resp, nil
		}
		fmt.Println("error: ", err)
		return nil, err
	}

	return nil, appresult.ErrAlreadyData("category")
}

func (r *repository) GetOne(ctx context.Context, categoryId int, baseURL string) (*category.CategoryDTO, error) {

	var categoryObj category.CategoryDTO

	q := `
		SELECT c.id, d.tm, d.en, d.ru, c.image_path, c.has_detail
		FROM categories c
		JOIN dictionary d ON c.name_dictionary_id = d.id
		WHERE c.id = $1
	`

	err := r.client.QueryRow(ctx, q, categoryId).Scan(
		&categoryObj.Id,
		&categoryObj.Name.Tm,
		&categoryObj.Name.En,
		&categoryObj.Name.Ru,
		&categoryObj.ImagePath,
		&categoryObj.HasDetail,
	)

	if err != nil {

		if errors.Is(err, pgx.ErrNoRows) {
			return nil, appresult.ErrNotFoundType(categoryId, "category")
		}

		return nil, appresult.ErrInternalServer
	}

	if baseURL != "" {
		cleanPath := strings.ReplaceAll(categoryObj.ImagePath, "\\", "/")
		categoryObj.ImagePath = fmt.Sprintf("%s/%s", baseURL, cleanPath)
	}

	return &categoryObj, nil
}

func (r *repository) GetAll(ctx context.Context, search string, limit string, offset string, baseURL string) (*category.CategoryAllDTO, error) {

	var (
		categories category.CategoryAllDTO
		count      int
	)

	offsetInt, err := strconv.Atoi(offset)
	if err != nil || offsetInt < 1 {
		offsetInt = 1
	}
	limitInt, err := strconv.Atoi(limit)
	if err != nil || limitInt < 1 {
		limitInt = 10
	}
	offsetInt = (offsetInt - 1) * limitInt

	q := `
		SELECT c.id, d.tm, d.en, d.ru, image_path, c.has_detail
		FROM categories c
		JOIN dictionary d ON c.name_dictionary_id = d.id
		WHERE ($1 = '' 
		OR d.tm ILIKE '%' || $1 || '%' 
		OR d.en ILIKE '%' || $1 || '%' 
		OR d.ru ILIKE '%' || $1 || '%')
		ORDER BY c.created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.client.Query(ctx, q, search, limitInt, offsetInt)
	if err != nil {
		return nil, appresult.ErrInternalServer
	}

	defer rows.Close()

	for rows.Next() {

		var c category.CategoryDTO

		if err := rows.Scan(
			&c.Id,
			&c.Name.Tm,
			&c.Name.En,
			&c.Name.Ru,
			&c.ImagePath,
			&c.HasDetail,
		); err != nil {
			return nil, appresult.ErrInternalServer
		}

		cleanPath := strings.ReplaceAll(c.ImagePath, "\\", "/")
		c.ImagePath = fmt.Sprintf("%s/%s", baseURL, cleanPath)

		categories.Categories = append(categories.Categories, c)
	}

	q = `
		SELECT count(*)
		FROM categories c
		JOIN dictionary d ON c.name_dictionary_id = d.id
		WHERE ($1 = '' 
		OR d.tm ILIKE '%' || $1 || '%' 
		OR d.en ILIKE '%' || $1 || '%' 
		OR d.ru ILIKE '%' || $1 || '%')
	`

	err = r.client.QueryRow(ctx, q, search).Scan(&count)
	if err != nil {
		return nil, appresult.ErrInternalServer
	}

	categories.Count = count

	return &categories, nil
}

func (r *repository) Update(ctx context.Context, categoryId int, dto category.CategoryReqDTO, imagePath string, baseURL string) (*category.CategoryDTO, error) {

	categoryObj, err := r.GetOne(ctx, categoryId, "")
	if err != nil {
		return nil, err
	}

	if dto.Name.En != "" && dto.Name.Ru != "" && dto.Name.Tm != "" {

		var id int

		q := `
			SELECT c.id
			FROM dictionary d
			JOIN categories c ON c.name_dictionary_id = d.id
			WHERE d.tm=$1 OR d.en=$2 OR d.ru=$3
		`

		err := r.client.QueryRow(ctx, q, dto.Name.Tm, dto.Name.En, dto.Name.Ru).Scan(&id)

		if err == nil && id != categoryId {
			return nil, appresult.ErrAlreadyData("name")
		}

		categoryObj.Name.En = dto.Name.En
		categoryObj.Name.Ru = dto.Name.Ru
		categoryObj.Name.Tm = dto.Name.Tm
	}

	if imagePath != "" {

		var img []string
		img = append(img, categoryObj.ImagePath)

		utils.DropFiles(&img)

		categoryObj.ImagePath = imagePath
	}

	if dto.HasDetail != nil {
		if dto.HasDetail != nil {
			categoryObj.HasDetail = *dto.HasDetail
		}
	}

	q := `
		UPDATE dictionary d
		SET tm=$1,en=$2,ru=$3
		FROM categories c
		WHERE c.name_dictionary_id=d.id
		AND c.id=$4
	`

	_, err = r.client.Exec(ctx, q,
		categoryObj.Name.Tm,
		categoryObj.Name.En,
		categoryObj.Name.Ru,
		categoryId,
	)

	if err != nil {
		return nil, appresult.ErrInternalServer
	}

	q = `
		UPDATE categories
		SET image_path=$1, has_detail=$2
		WHERE id=$3
	`

	_, err = r.client.Exec(ctx, q, categoryObj.ImagePath, categoryObj.HasDetail, categoryId)

	if err != nil {
		return nil, appresult.ErrInternalServer
	}

	return r.GetOne(ctx, categoryId, baseURL)
}

func (r *repository) Delete(ctx context.Context, categoryId int) error {

	var (
		nameDictionaryId int
		imagePath        string
	)

	q := `
		SELECT name_dictionary_id, image_path
		FROM categories
		WHERE id=$1
	`

	err := r.client.QueryRow(ctx, q, categoryId).Scan(&nameDictionaryId, &imagePath)

	if err != nil {

		if errors.Is(err, pgx.ErrNoRows) {
			return appresult.ErrNotFoundType(categoryId, "category")
		}

		return appresult.ErrInternalServer
	}

	if imagePath != "" {

		var img []string
		img = append(img, imagePath)

		utils.DropFiles(&img)
	}

	_, err = r.client.Exec(ctx, `DELETE FROM categories WHERE id=$1`, categoryId)

	if err != nil {
		return appresult.ErrInternalServer
	}

	_, err = r.client.Exec(ctx, `DELETE FROM dictionary WHERE id=$1`, nameDictionaryId)

	if err != nil {
		return appresult.ErrInternalServer
	}

	return nil
}
