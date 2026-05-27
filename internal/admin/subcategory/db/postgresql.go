package db

import (
	"context"
	"errors"
	"fmt"
	"restaurants/internal/admin/subcategory"
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

func NewRepository(client postgresql.Client, logger *logging.Logger) subcategory.Repository {
	return &repository{
		client: client,
		logger: logger,
	}
}

func (r *repository) Create(ctx context.Context, dto subcategory.SubcategoryDTO, imagePath string, baseURL string) (*subcategory.SubcategoryOneDTO, error) {

	var (
		id            int
		subcategoryId int
	)

	q := `
		SELECT d.id
		FROM subcategories s 
		JOIN dictionary d ON s.name_dictionary_id = d.id
		WHERE ( d.tm = $1 OR d.en = $2 OR d.ru = $3 ) AND s.category_id = $4
	`

	err := r.client.QueryRow(ctx, q, dto.Name.Tm, dto.Name.En, dto.Name.Ru, dto.CategoryID).Scan(&id)

	if err != nil {

		if errors.Is(err, pgx.ErrNoRows) {
			q = `
				INSERT INTO dictionary (tm, en, ru)
				VALUES ($1,$2,$3)
				RETURNING id
			`

			err = r.client.QueryRow(ctx, q, dto.Name.Tm, dto.Name.En, dto.Name.Ru).Scan(&id)
			if err != nil {
				fmt.Println("error: ", err)
				return nil, appresult.ErrInternalServer
			}

			q := `
				INSERT INTO subcategories (name_dictionary_id, category_id, image_path)
				VALUES ($1, $2, $3)
				RETURNING id
			`

			err = r.client.QueryRow(ctx, q, id, dto.CategoryID, imagePath).Scan(&subcategoryId)
			if err != nil {
				fmt.Println("error: ", err)
				return nil, appresult.ErrInternalServer
			}

			resp, err := r.GetOne(ctx, subcategoryId, baseURL)
			if err != nil {
				fmt.Println("error: ", err)
				return nil, err
			}

			return resp, nil
		}
		fmt.Println("error: ", err)
		return nil, err
	}

	return nil, appresult.ErrAlreadyData("subcategory")
}

func (r *repository) GetOne(ctx context.Context, subcategoryId int, baseURL string) (*subcategory.SubcategoryOneDTO, error) {

	var resp subcategory.SubcategoryOneDTO

	q := `
		SELECT 
			s.id, ds.tm, ds.en, ds.ru, s.image_path,
			c.id, dc.tm, dc.en, dc.ru, c.image_path
		FROM subcategories s
		JOIN categories c ON s.category_id = c.id
		JOIN dictionary ds ON s.name_dictionary_id = ds.id
		JOIN dictionary dc ON c.name_dictionary_id = dc.id
		WHERE s.id = $1
	`

	err := r.client.QueryRow(ctx, q, subcategoryId).Scan(
		&resp.Id,
		&resp.Name.Tm,
		&resp.Name.En,
		&resp.Name.Ru,
		&resp.ImagePath,
		&resp.Category.Id,
		&resp.Category.Name.Tm,
		&resp.Category.Name.En,
		&resp.Category.Name.Ru,
		&resp.Category.ImagePath,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			fmt.Println("error: ", err)
			return nil, appresult.ErrNotFoundType(subcategoryId, "subcategory")
		}

		fmt.Println("error: ", err)
		return nil, err
	}

	if baseURL != "" {
		cleanPath := strings.ReplaceAll(resp.ImagePath, "\\", "/")
		resp.ImagePath = fmt.Sprintf("%s/%s", baseURL, cleanPath)

		cleanPathC := strings.ReplaceAll(resp.Category.ImagePath, "\\", "/")
		resp.Category.ImagePath = fmt.Sprintf("%s/%s", baseURL, cleanPathC)
	}

	return &resp, nil
}

func (r *repository) GetAll(ctx context.Context, search string, limit string, offset string, categoryID string, baseURL string) (*subcategory.SubcategoryAllDTO, error) {

	var (
		subcategories subcategory.SubcategoryAllDTO
		count         int
	)
	subcategories.Subcategories = []subcategory.SubcategoriesDTO{}

	offsetInt, err := strconv.Atoi(offset)
	if err != nil || offsetInt < 1 {
		offsetInt = 1
	}
	limitInt, err := strconv.Atoi(limit)
	if err != nil || limitInt < 1 {
		limitInt = 10
	}

	offsetInt = (offsetInt - 1) * limitInt

	categoryFilter := ""
	if categoryID != "" {
		categoryFilter = fmt.Sprintf("AND s.category_id = %s", categoryID)
	}

	q := fmt.Sprintf(`
		SELECT s.id, d.tm, d.en, d.ru, s.image_path
		FROM subcategories s
		JOIN dictionary d ON s.name_dictionary_id = d.id
		WHERE ($1 = '' 
		OR d.tm ILIKE '%%' || $1 || '%%' 
		OR d.en ILIKE '%%' || $1 || '%%' 
		OR d.ru ILIKE '%%' || $1 || '%%')
		%s
		ORDER BY s.created_at DESC
		LIMIT $2 OFFSET $3
	`, categoryFilter)

	rows, err := r.client.Query(ctx, q, search, limitInt, offsetInt)

	if err != nil {
		fmt.Println("error: ", err)
		return nil, appresult.ErrInternalServer
	}
	defer rows.Close()

	for rows.Next() {

		var s subcategory.SubcategoriesDTO

		if err := rows.Scan(
			&s.Id,
			&s.Name.Tm,
			&s.Name.En,
			&s.Name.Ru,
			&s.ImagePath,
		); err != nil {
			fmt.Println("error: ", err)
			return nil, appresult.ErrInternalServer
		}

		cleanPath := strings.ReplaceAll(s.ImagePath, "\\", "/")
		s.ImagePath = fmt.Sprintf("%s/%s", baseURL, cleanPath)

		subcategories.Subcategories = append(subcategories.Subcategories, s)
	}

	qCount := fmt.Sprintf(`
		SELECT count(*)
		FROM subcategories s
		JOIN dictionary d ON s.name_dictionary_id = d.id
		WHERE ($1 = '' 
		OR d.tm ILIKE '%%' || $1 || '%%' 
		OR d.en ILIKE '%%' || $1 || '%%' 
		OR d.ru ILIKE '%%' || $1 || '%%')
		%s
	`, categoryFilter)

	err = r.client.QueryRow(ctx, qCount, search).Scan(&count)

	if err != nil {
		return nil, appresult.ErrInternalServer
	}

	subcategories.Count = count

	return &subcategories, nil
}

func (r *repository) Update(ctx context.Context, subcategoryId int, dto subcategory.SubcategoryDTO, imagePath string, baseURL string) (*subcategory.SubcategoryOneDTO, error) {

	subcategoryResp, err := r.GetOne(ctx, subcategoryId, "")
	if err != nil {
		return nil, err
	}

	if dto.CategoryID != 0 && dto.CategoryID != subcategoryResp.Category.Id {
		var catExists int
		err := r.client.QueryRow(ctx, `SELECT id FROM categories WHERE id=$1`, dto.CategoryID).Scan(&catExists)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, appresult.ErrNotFoundType(dto.CategoryID, "category")
			}
			return nil, appresult.ErrInternalServer
		}

		subcategoryResp.Category.Id = dto.CategoryID
	}

	if dto.Name.Tm != "" || dto.Name.En != "" || dto.Name.Ru != "" {
		var id int
		q := `
			SELECT s.id
			FROM dictionary d
			JOIN subcategories s ON s.name_dictionary_id = d.id
			WHERE (d.tm=$1 OR d.en=$2 OR d.ru=$3)
			AND s.category_id = $4
		`
		err := r.client.QueryRow(ctx, q, dto.Name.Tm, dto.Name.En, dto.Name.Ru, subcategoryResp.Category.Id).Scan(&id)
		if err == nil && id != subcategoryId {
			return nil, appresult.ErrAlreadyData("name")
		}

		_, err = r.client.Exec(ctx, `
			UPDATE dictionary d
			SET tm=$1,en=$2,ru=$3
			FROM subcategories s
			WHERE s.name_dictionary_id=d.id AND s.id=$4
		`, dto.Name.Tm, dto.Name.En, dto.Name.Ru, subcategoryId)
		if err != nil {
			return nil, appresult.ErrInternalServer
		}
	}

	if imagePath != "" {
		var img []string
		img = append(img, subcategoryResp.ImagePath)
		utils.DropFiles(&img)
		subcategoryResp.ImagePath = imagePath
	}

	q := `UPDATE subcategories 
			SET 
		category_id = $1,
		image_path = $2 
		WHERE id = $3`
	_, err = r.client.Exec(ctx, q, dto.CategoryID, subcategoryResp.ImagePath, subcategoryId)
	if err != nil {
		return nil, appresult.ErrInternalServer
	}

	return r.GetOne(ctx, subcategoryId, baseURL)
}

func (r *repository) Delete(ctx context.Context, subcategoryId int) error {
	var (
		nameDictionaryId int
		imagePath        string
	)

	q := `
		SELECT name_dictionary_id, image_path
		FROM subcategories
		WHERE id=$1
	`

	err := r.client.QueryRow(ctx, q, subcategoryId).Scan(&nameDictionaryId, &imagePath)

	if err != nil {

		if errors.Is(err, pgx.ErrNoRows) {
			return appresult.ErrNotFoundType(subcategoryId, "subcategory")
		}

		return appresult.ErrInternalServer
	}

	if imagePath != "" {

		var img []string
		img = append(img, imagePath)

		utils.DropFiles(&img)
	}

	_, err = r.client.Exec(ctx, `DELETE FROM subcategories WHERE id=$1`, subcategoryId)

	if err != nil {
		return appresult.ErrInternalServer
	}

	_, err = r.client.Exec(ctx, `DELETE FROM dictionary WHERE id=$1`, nameDictionaryId)

	if err != nil {
		return appresult.ErrInternalServer
	}

	return nil
}

func (r *repository) ByCategory(ctx context.Context, categoryIDs []int, baseURL string) (*[]subcategory.ByCategory, error) {
	var categories []subcategory.ByCategory

	for _, id := range categoryIDs {

		var category subcategory.ByCategory

		category.Subcategories = []subcategory.SubcategoriesDTO{}

		q := `
			SELECT dc.tm, dc.en, dc.ru
			FROM categories c
			JOIN dictionary dc ON c.name_dictionary_id = dc.id
			WHERE c.id = $1
		`

		err := r.client.QueryRow(ctx, q, id).Scan(
			&category.CategoryName.Tm,
			&category.CategoryName.En,
			&category.CategoryName.Ru,
		)

		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, appresult.ErrNotFoundType(id, "category")
			}
			return nil, err
		}

		q = `
			SELECT 
				s.id, ds.tm, ds.en, ds.ru, s.image_path
			FROM subcategories s
			JOIN dictionary ds ON s.name_dictionary_id = ds.id
			WHERE s.category_id = $1
		`

		rows, err := r.client.Query(ctx, q, id)
		if err != nil {
			return nil, appresult.ErrInternalServer
		}

		for rows.Next() {
			var s subcategory.SubcategoriesDTO

			if err := rows.Scan(
				&s.Id,
				&s.Name.Tm,
				&s.Name.En,
				&s.Name.Ru,
				&s.ImagePath,
			); err != nil {
				rows.Close()
				return nil, appresult.ErrInternalServer
			}

			category.Subcategories = append(category.Subcategories, s)
		}

		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, err
		}

		rows.Close()

		categories = append(categories, category)
	}

	return &categories, nil
}
