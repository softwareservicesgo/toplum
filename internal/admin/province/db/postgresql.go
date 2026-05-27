package db

import (
	"context"
	"errors"
	"fmt"
	"restaurants/internal/admin/province"
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

func NewRepository(client postgresql.Client, logger *logging.Logger) province.Repository {
	return &repository{
		client: client,
		logger: logger,
	}
}

func (r *repository) Create(ctx context.Context, dto province.DictionaryDTO) (*province.ProvinceDTO, error) {
	var (
		resp       *province.ProvinceDTO
		id         int
		provinceId int
	)

	q := `SELECT d.id
			FROM dictionary d
			JOIN provinces p ON p.name_dictionary_id = d.id
			WHERE d.tm = $1 OR d.en = $2 OR d.ru = $3;
			`
	err := r.client.QueryRow(ctx, q, dto.Tm, dto.En, dto.Ru).Scan(&id)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			insertDict := `
				INSERT INTO dictionary (tm, en, ru)
				VALUES ($1, $2, $3)
				RETURNING id;
			`
			err = r.client.QueryRow(ctx, insertDict, dto.Tm, dto.En, dto.Ru).Scan(&id)
			if err != nil {
				fmt.Println("error: ", err)
				return nil, appresult.ErrInternalServer
			}

			insertProvince := `
				INSERT INTO provinces (name_dictionary_id)
				VALUES ($1)
				RETURNING id;
			`
			err = r.client.QueryRow(ctx, insertProvince, id).Scan(&provinceId)
			if err != nil {
				return nil, appresult.ErrInternalServer
			}

			resp, err = r.GetOne(ctx, provinceId)
			if err != nil {
				return nil, err
			}
			return resp, nil

		} else {
			fmt.Println("error: ", err)
			return nil, appresult.ErrInternalServer
		}
	}
	return nil, appresult.ErrAlreadyData("province")
}

func (r *repository) GetOne(ctx context.Context, provinceId int) (*province.ProvinceDTO, error) {
	var (
		province province.ProvinceDTO
	)

	q := `SELECT p.id, d.tm, d.en, d.ru
			FROM provinces p 
			JOIN dictionary d ON p.name_dictionary_id = d.id
			WHERE p.id = $1;
			`
	err := r.client.QueryRow(ctx, q, provinceId).Scan(
		&province.Id, &province.Name.Tm,
		&province.Name.En, &province.Name.Ru)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			fmt.Println("error: ", err)
			return nil, appresult.ErrNotFoundType(provinceId, "province")
		} else {
			fmt.Println("error: ", err)
			return nil, appresult.ErrInternalServer
		}
	}
	return &province, nil
}

func (r *repository) GetAll(ctx context.Context, search string, page string, size string) (*[]province.ProvinceDTO, error) {
	var provincess []province.ProvinceDTO

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
		SELECT p.id, d.tm, d.en, d.ru
		FROM provinces p 
		JOIN dictionary d ON p.name_dictionary_id = d.id
		WHERE ($1 = '' OR d.tm ILIKE '%' || $1 || '%' OR d.en ILIKE '%' || $1 || '%' OR d.ru ILIKE '%' || $1 || '%')
		ORDER BY p.created_at DESC
		LIMIT $2 OFFSET $3
		`
	rows, err := r.client.Query(ctx, q, search, sizeInt, offset)
	if err != nil {
		fmt.Println("error: ", err)
		return nil, appresult.ErrInternalServer
	}
	defer rows.Close()

	for rows.Next() {
		var p province.ProvinceDTO
		if err := rows.Scan(
			&p.Id, &p.Name.Tm, &p.Name.En, &p.Name.Ru,
		); err != nil {
			fmt.Println("error: ", err)
			return nil, appresult.ErrInternalServer
		}
		provincess = append(provincess, p)
	}

	return &provincess, nil
}

func (r *repository) Update(ctx context.Context, provinceId int, dto province.DictionaryDTO) (*province.ProvinceDTO, error) {
	var id int
	_, err := r.GetOne(ctx, provinceId)
	if err != nil {
		return nil, appresult.ErrNotFoundType(provinceId, "province")
	}

	q := `SELECT p.id
			FROM dictionary d
			JOIN provinces p ON p.name_dictionary_id = d.id
			WHERE d.tm = $1 OR d.en = $2 OR d.ru = $3;
			`
	err = r.client.QueryRow(ctx, q, dto.Tm, dto.En, dto.Ru).Scan(&id)
	if err == nil && id != provinceId {
		return nil, appresult.ErrAlreadyData("name")
	}

	q = `
		UPDATE dictionary d
		SET tm = $1, en = $2, ru = $3
		FROM provinces p
		WHERE p.name_dictionary_id = d.id
		  AND p.id = $4
	`
	_, err = r.client.Exec(ctx, q, dto.Tm, dto.En, dto.Ru, provinceId)
	if err != nil {
		fmt.Println("error: ", err)
		return nil, appresult.ErrInternalServer
	}

	updated, err := r.GetOne(ctx, provinceId)
	if err != nil {
		fmt.Println("error: ", err)
		return nil, appresult.ErrInternalServer
	}

	return updated, nil
}

func (r *repository) Delete(ctx context.Context, provinceId int) error {
	var nameDictionaryId int

	q := `SELECT p.name_dictionary_id
			FROM provinces p 
			WHERE p.id = $1;
			`
	err := r.client.QueryRow(ctx, q, provinceId).Scan(&nameDictionaryId)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			fmt.Println("error: ", err)
			return appresult.ErrNotFoundType(provinceId, "province")
		} else {
			fmt.Println("error: ", err)
			return appresult.ErrInternalServer
		}
	}

	if err != nil {
		fmt.Println("error: ", err)
		return appresult.ErrNotFoundBase
	}

	tx, err := r.client.Begin(ctx)
	if err != nil {
		fmt.Println("error: ", err)
		return appresult.ErrInternalServer
	}
	defer func() {
		if err != nil {
			tx.Rollback(ctx)
		} else {
			tx.Commit(ctx)
		}
	}()

	delProvince := `DELETE FROM provinces WHERE id = $1`
	_, err = tx.Exec(ctx, delProvince, provinceId)
	if err != nil {
		fmt.Println("error: ", err)
		return appresult.ErrInternalServer
	}

	delDict := `DELETE FROM dictionary WHERE id = $1`
	_, err = tx.Exec(ctx, delDict, nameDictionaryId)
	if err != nil {
		fmt.Println("error: ", err)
		return appresult.ErrInternalServer
	}

	return nil
}
