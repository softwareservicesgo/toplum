package db

import (
	"context"
	"errors"
	"fmt"
	"math"
	"restaurants/internal/admin/businesses"
	"restaurants/internal/admin/category"
	"restaurants/internal/admin/item"
	"restaurants/internal/admin/subcategory"
	"restaurants/internal/appresult"
	"restaurants/internal/enum"
	"restaurants/pkg/client/postgresql"
	"restaurants/pkg/logging"
	"strings"
	"time"

	"github.com/jackc/pgx/v4"
)

type repository struct {
	client postgresql.Client
	logger *logging.Logger
}

func NewRepository(client postgresql.Client, logger *logging.Logger) businesses.Repository {
	return &repository{
		client: client,
		logger: logger,
	}
}

var (
	formatTime = "15:04"
)

func (r *repository) Create(ctx context.Context, userId int, dto businesses.BusinessesReqDTO) (*int, error) {
	tx, err := r.client.Begin(ctx)
	if err != nil {
		fmt.Println("error :", err)
		return nil, appresult.ErrInternalServer
	}
	defer tx.Rollback(ctx)

	var (
		districtId, descriptionId, businessId, provinceId, subcategoryID, categoryId int
	)
	queryDictionary := `INSERT INTO dictionary (tm, en, ru) VALUES ($1, $2, $3) RETURNING id`

	q := `SELECT id FROM provinces WHERE id = $1;`
	err = tx.QueryRow(ctx, q, dto.ProvinceId).Scan(&provinceId)
	if err != nil {
		fmt.Println("error:", err)
		return nil, appresult.ErrNotFoundType(provinceId, "provinces")
	}

	if dto.CategoryId != nil {
		q := `SELECT id FROM categories WHERE id = $1;`
		err = tx.QueryRow(ctx, q, dto.CategoryId).Scan(&categoryId)
		if err != nil {
			fmt.Println("error:", err)
			return nil, appresult.ErrNotFoundType(categoryId, "category")
		}
	}

	if dto.SubcategoryIds != nil && len(*dto.SubcategoryIds) > 0 {
		for _, subcategoryId := range *dto.SubcategoryIds {
			q := `SELECT id FROM subcategories
		WHERE id = $1;`
			err = tx.QueryRow(ctx, q, subcategoryId).Scan(&subcategoryID)
			if err != nil {
				fmt.Println("error:", err)
				return nil, appresult.ErrNotFoundType(subcategoryId, "subcategory")
			}
		}
	}

	err = tx.QueryRow(ctx, queryDictionary, dto.District.Tm, dto.District.En, dto.District.Ru).Scan(&districtId)
	if err != nil {
		fmt.Println("error insert district dictionary:", err)
		return nil, appresult.ErrInternalServer
	}

	err = tx.QueryRow(ctx, queryDictionary, dto.Description.Tm, dto.Description.En, dto.Description.Ru).Scan(&descriptionId)
	if err != nil {
		fmt.Println("error insert description dictionary:", err)
		return nil, appresult.ErrInternalServer
	}

	_, err = time.Parse(formatTime, dto.OpensTime)
	if err != nil {
		fmt.Println("error :", err)
		return nil, appresult.ErrTime("opens")
	}

	_, err = time.Parse(formatTime, dto.ClosesTime)
	if err != nil {
		fmt.Println("error :", err)
		return nil, appresult.ErrTime("closes")
	}
	canOrder := false
	if dto.CanOrder != nil {
		canOrder = *dto.CanOrder
	}

	canReserve := false
	if dto.CanReserve != nil {
		canReserve = *dto.CanReserve
	}

	query := `
		INSERT INTO businesses
			(name, province_id, district_dictionary_id, category_id,
			phone, description_dictionary_id, opens_time, closes_time,
			expires, value, can_order, can_reserve)
		VALUES
			($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id
	`
	err = tx.QueryRow(ctx, query,
		dto.Name, dto.ProvinceId, districtId,
		dto.CategoryId, dto.Phone, descriptionId,
		dto.OpensTime, dto.ClosesTime, dto.Expires,
		dto.Value, canOrder, canReserve).Scan(&businessId)
	if err != nil {
		fmt.Println("error insert business:", err)
		return nil, err
	}

	if dto.SubcategoryIds != nil && len(*dto.SubcategoryIds) > 0 {
		query = `INSERT INTO businesses_subcategories (businesses_id, subcategory_id) VALUES ($1, $2)`
		for _, subcategoryId := range *dto.SubcategoryIds {
			if _, err := tx.Exec(ctx, query, businessId, subcategoryId); err != nil {
				fmt.Println("error insert businesses_subcategories:", err)
				return nil, err
			}
		}
	}

	query = `
		INSERT INTO user_businesses
			(user_id, businesses_id, role)
		VALUES
			($1, $2, $3)
	`
	_, err = tx.Exec(ctx, query,
		userId, businessId, enum.RoleManager)
	if err != nil {
		fmt.Println("error insert user in businesses:", err)
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		fmt.Println("error commit transaction:", err)
		return nil, err
	}

	return &businessId, nil
}

func (r *repository) AddImages(
	ctx context.Context,
	businessId int,
	mainImage string,
	images []string,
	baseURL string,
) (*businesses.BusinessesResDTO, error) {

	tx, err := r.client.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if rbErr := tx.Rollback(ctx); rbErr != nil && !errors.Is(rbErr, pgx.ErrTxClosed) {
			fmt.Println("rollback error:", rbErr)
		}
	}()

	insertImage := func(path string, isMain bool) error {
		query := `INSERT INTO image_businesses (businesses_id, image_path, is_main) VALUES ($1, $2, $3)`
		_, err := tx.Exec(ctx, query, businessId, path, isMain)
		if err != nil {
			return fmt.Errorf("insert image %q: %w", path, err)
		}
		return nil
	}

	if mainImage != "" {
		if err := insertImage(mainImage, true); err != nil {
			return nil, err
		}
	}

	for _, path := range images {
		if err := insertImage(path, false); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}

	createdBusiness, err := r.GetOne(ctx, businessId, baseURL)
	if err != nil {
		return nil, fmt.Errorf("get business: %w", err)
	}

	return createdBusiness, nil
}

func (r *repository) GetOne(ctx context.Context, businessId int, baseURL string) (*businesses.BusinessesResDTO, error) {
	var (
		res                   businesses.BusinessesResDTO
		opensTime, closesTime time.Time
		categoryId            *int
	)
	res.Items = []item.ItemGetAllDTO{}

	qBusiness := `
			SELECT 
				b.id,
				b.name, 
				(p_name.tm || ', ' || d_district.tm)  AS addr_tm,
				(p_name.en || ', ' || d_district.en)  AS addr_en,
				(p_name.ru || ', ' || d_district.ru)  AS addr_ru,
				b.phone,
				p.id,
				p_name.tm, p_name.en, p_name.ru,
				COALESCE(d_district.tm, ''), COALESCE(d_district.en, ''), COALESCE(d_district.ru, ''),
				b.opens_time,
				b.closes_time,
				b.expires,
				b.category_id,
				b.value,
				b.discount_percent,
				b.status,
				b.can_order,
				b.can_reserve
			FROM businesses b
			JOIN provinces p            ON b.province_id = p.id
			JOIN dictionary p_name      ON p.name_dictionary_id = p_name.id
			JOIN dictionary d_district  ON b.district_dictionary_id = d_district.id
			JOIN dictionary d_desc      ON b.description_dictionary_id = d_desc.id
			WHERE b.id = $1;
		`
	err := r.client.QueryRow(ctx, qBusiness, businessId).Scan(
		&res.Id,
		&res.Name,
		&res.Address.Tm, &res.Address.En, &res.Address.Ru,
		&res.Phone,
		&res.Province.Id, &res.Province.Name.Tm, &res.Province.Name.En, &res.Province.Name.Ru,
		&res.Description.Tm, &res.Description.En, &res.Description.Ru,
		&opensTime,
		&closesTime,
		&res.Expires,
		&categoryId,
		&res.Value,
		&res.DiscountPercent,
		&res.Status,
		&res.CanOrder,
		&res.CanReserve,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			fmt.Println("error1: ", err)
			return nil, appresult.ErrNotFoundType(businessId, "businesses")
		}
		fmt.Println("error2: ", err)
		return nil, appresult.ErrInternalServer
	}

	res.OpensTime = opensTime.Format(formatTime)
	res.ClosesTime = closesTime.Format(formatTime)

	res.Subcategory, err = FindSubcategories(r, ctx, businessId, baseURL)
	if err != nil {
		fmt.Println("error3: ", err)
		return nil, err
	}

	if len(res.Subcategory) == 0 {
		res.Subcategory = []subcategory.SubcategoriesDTO{}
	}

	res.Images, err = getImagesByBusinessId(r, ctx, businessId, baseURL)
	if err != nil {
		fmt.Println("error4", err)
		return nil, err
	}

	if categoryId != nil {
		var resC category.CategoryDTO
		q := `
			SELECT
				c.id, d.tm, d.en, d.ru, c.image_path
			FROM categories c
			JOIN dictionary d ON d.id = c.name_dictionary_id
			WHERE c.id = $1;
		`
		err := r.client.QueryRow(ctx, q, *categoryId).Scan(
			&resC.Id,
			&resC.Name.Tm,
			&resC.Name.En,
			&resC.Name.Ru,
			&resC.ImagePath,
		)
		if err != nil {
			fmt.Println("error5", err)
			return nil, err
		}
		if baseURL != "" {
			cleanPath := strings.ReplaceAll(resC.ImagePath, "\\", "/")
			resC.ImagePath = fmt.Sprintf("%s/%s", baseURL, cleanPath)
		}
		res.Category = &resC
	}

	if res.Value != nil && res.DiscountPercent != nil && *res.DiscountPercent != 0 {
		x := math.Round((float64(*res.Value)*float64(*res.DiscountPercent))/10) / 10
		discountValue := float32(float64(*res.Value) - x)

		res.DiscountValue = &discountValue
	} else if *res.DiscountPercent == 0 {
		res.DiscountPercent = nil
	}

	return &res, nil
}

func getImagesByBusinessId(r *repository, ctx context.Context, businessId int, baseURL string) ([]string, error) {
	var images []string
	qImages := `
       		SELECT image_path
        FROM image_businesses
        WHERE businesses_id = $1
        ORDER BY is_main DESC
    `
	imgRows, err := r.client.Query(ctx, qImages, businessId)
	if err != nil {
		fmt.Println("error: ", err)
		return nil, appresult.ErrInternalServer
	}
	defer imgRows.Close()

	for imgRows.Next() {
		var path string
		if err := imgRows.Scan(&path); err != nil {
			fmt.Println("error: ", err)
			return nil, appresult.ErrInternalServer
		}
		if baseURL != "" {
			cleanPath := strings.ReplaceAll(path, "\\", "/")
			path = fmt.Sprintf("%s/%s", baseURL, cleanPath)
		}
		images = append(images, path)
	}

	return images, nil
}

func FindSubcategories(r *repository, ctx context.Context, businessId int, baseURL string) ([]subcategory.SubcategoriesDTO, error) {
	var res []subcategory.SubcategoriesDTO

	q := `
        SELECT 
			s.id, ds.tm, ds.en, ds.ru, s.image_path
		FROM businesses_subcategories bs
		JOIN subcategories s ON s.id = bs.subcategory_id
		JOIN dictionary ds ON s.name_dictionary_id = ds.id
		WHERE bs.businesses_id = $1
    `
	rows, err := r.client.Query(ctx, q, businessId)
	if err != nil {
		return res, appresult.ErrInternalServer
	}
	defer rows.Close()

	for rows.Next() {
		var subc subcategory.SubcategoriesDTO
		if err := rows.Scan(
			&subc.Id,
			&subc.Name.Tm,
			&subc.Name.En,
			&subc.Name.Ru,
			&subc.ImagePath,
		); err != nil {
			return res, appresult.ErrInternalServer
		}

		if baseURL != "" {
			cleanPath := strings.ReplaceAll(subc.ImagePath, "\\", "/")
			subc.ImagePath = fmt.Sprintf("%s/%s", baseURL, cleanPath)
		}

		res = append(res, subc)
	}
	return res, nil
}

func (r *repository) GetAll(ctx context.Context, filter businesses.BusinessesFilter, baseURL string) (*[]businesses.BusinessesAllDTO, *int, error) {
	var (
		total int
	)
	business := []businesses.BusinessesAllDTO{}
	args, queryAll, countQuery := makeFilter(filter)
	if err := r.client.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		fmt.Println("countQuery, error: ", err)
		return nil, nil, err
	}

	Limit := 10
	Offset := 1
	if filter.Limit > 0 {
		Limit = filter.Limit
	}
	if filter.Offset > 0 {
		Offset = filter.Offset
	}
	Offset = (Offset - 1) * Limit

	queryAll += fmt.Sprintf(" LIMIT %d OFFSET %d", Limit, Offset)

	rows, err := r.client.Query(ctx, queryAll, args...)
	if err != nil {
		fmt.Println("error,queryAll : ", err)
		return nil, nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			bDTO businesses.BusinessesAllDTO
		)
		if err := rows.Scan(
			&bDTO.Id,
			&bDTO.Name,
			&bDTO.Image,
			&bDTO.Status,
			&bDTO.Value,
			&bDTO.DiscountPercent,
		); err != nil {
			fmt.Println("error: ", err)
			return nil, nil, err
		}

		if bDTO.Value != nil && bDTO.DiscountPercent != nil && *bDTO.DiscountPercent != 0 {
			x := math.Round((float64(*bDTO.Value)*float64(*bDTO.DiscountPercent))/10) / 10
			discountValue := float32(float64(*bDTO.Value) - x)

			bDTO.DiscountValue = &discountValue
		} else if *bDTO.DiscountPercent == 0 {
			bDTO.DiscountPercent = nil
		}

		cleanPath := strings.ReplaceAll(bDTO.Image, "\\", "/")
		bDTO.Image = fmt.Sprintf("%s/%s", baseURL, cleanPath)
		business = append(business, bDTO)
	}

	return &business, &total, nil
}

func makeFilter(filter businesses.BusinessesFilter) ([]interface{}, string, string) {
	var args []interface{}
	idx := 1

	filters := []string{"1=1"}

	if filter.SubcategoryId != 0 {
		filters = append(filters, fmt.Sprintf("EXISTS (SELECT 1 FROM businesses_subcategories bs WHERE bs.businesses_id = b.id AND bs.subcategory_id = $%d)", idx))
		args = append(args, filter.SubcategoryId)
		idx++
	}

	if filter.ProvinceID != 0 {
		filters = append(filters, fmt.Sprintf("b.province_id = $%d", idx))
		args = append(args, filter.ProvinceID)
		idx++
	}

	if filter.CategoryId != 0 {
		filters = append(filters, fmt.Sprintf("b.category_id = $%d", idx))
		args = append(args, filter.CategoryId)
		idx++
	}

	if filter.IsDiscounted != nil && *filter.IsDiscounted == true {
		filters = append(filters, "b.discount_percent != 0")
	}

	if filter.Status != "" {
		filters = append(filters, fmt.Sprintf("b.status = $%d ", idx))
		args = append(args, filter.Status)
		idx++
	}

	if filter.OpenTime != "" && filter.ClosesTime != "" {
		filters = append(filters, fmt.Sprintf("$%d <= b.opens_time  AND b.closes_time <= $%d", idx, idx+1))
		args = append(args, filter.OpenTime)
		args = append(args, filter.ClosesTime)
		idx += idx + 2
	}

	if filter.Search != "" {
		filters = append(filters, fmt.Sprintf(`
		(LOWER(b.name) LIKE LOWER('%%' || $%d || '%%') 
		OR LOWER(d_province.tm || ', ' || COALESCE(d_district.tm,'')) LIKE LOWER('%%' || $%d || '%%')
		OR LOWER(d_province.en || ', ' || COALESCE(d_district.en,'')) LIKE LOWER('%%' || $%d || '%%')
		OR LOWER(d_province.ru || ', ' || COALESCE(d_district.ru,'')) LIKE LOWER('%%' || $%d || '%%')
		)`, idx, idx, idx, idx))
		args = append(args, filter.Search)
		idx++
	}

	sort := "ORDER BY b.created_at DESC"
	if filter.SortByValue != "" {
		filters = append(filters, "b.value IS NOT NULL")
		sort = fmt.Sprintf("ORDER BY b.value %s", filter.SortByValue)
	}

	query := fmt.Sprintf(`
		SELECT
			b.id,
			b.name,
			img.image_path,
			b.status,
			b.value,
			b.discount_percent
		FROM businesses b
		JOIN provinces p ON p.id = b.province_id
		JOIN dictionary d_province ON d_province.id = p.name_dictionary_id
		LEFT JOIN image_businesses img ON img.businesses_id = b.id AND img.is_main = true
		WHERE %s
		%s
		`, strings.Join(filters, " AND "), sort)

	countQuery := fmt.Sprintf(`
		SELECT COUNT(*) 
		FROM businesses b
		JOIN provinces p ON p.id = b.province_id
		JOIN dictionary d_province ON d_province.id = p.name_dictionary_id
		WHERE %s
		`, strings.Join(filters, " AND "))
	return args, query, countQuery
}

func (r *repository) Update(ctx context.Context, businessId int, dto businesses.BusinessesReqDTO) error {
	var (
		res                   businesses.BusinessForUpdateDTO
		opensTime, closesTime time.Time
	)

	tx, err := r.client.Begin(ctx)
	if err != nil {
		fmt.Println("error: ", err)
		return appresult.ErrInternalServer
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)
			panic(p)
		}
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	err = tx.QueryRow(ctx, `
			SELECT 
			name, province_id, district_dictionary_id, 
			phone, description_dictionary_id,
			opens_time, closes_time, expires,
			category_id, value, can_order, can_reserve
		FROM businesses
		WHERE id = $1
	`, businessId).Scan(
		&res.Name, &res.ProvinceId,
		&res.DistrictId, &res.Phone, &res.DescriptionId,
		&opensTime, &closesTime, &res.Expires,
		&res.CategoryId, &res.Value, &res.CanOrder, &res.CanReserve,
	)
	if err != nil {
		fmt.Println("error: ", err)
		return appresult.ErrNotFoundType(businessId, "businesses")
	}

	res.OpensTime = opensTime.Format(formatTime)
	res.ClosesTime = closesTime.Format(formatTime)

	updateOrCreateDictionary := func(idPtr *int, dict businesses.DictionaryDTO) (int, error) {
		if dict.Tm == "" && dict.En == "" && dict.Ru == "" {
			return 0, nil
		}

		var currentId int
		if idPtr != nil {
			currentId = *idPtr
		}

		if currentId != 0 {
			_, err := tx.Exec(ctx, `
            UPDATE dictionary
            SET tm = $1, en = $2, ru = $3
            WHERE id = $4
        `, dict.Tm, dict.En, dict.Ru, currentId)
			fmt.Println("error: ", err)
			return 0, err
		} else {
			var newId int
			err := tx.QueryRow(ctx, `
            INSERT INTO dictionary (tm, en, ru)
            VALUES ($1, $2, $3)
            RETURNING id
        `, dict.Tm, dict.En, dict.Ru).Scan(&newId)
			if err != nil {
				fmt.Println("error: ", err)
				return 0, err
			}
			return newId, nil
		}
	}

	if dto.Name != "" {
		res.Name = dto.Name
	}

	if newId, err := updateOrCreateDictionary(&res.DistrictId, dto.District); err != nil {
		return appresult.ErrInternalServer
	} else if newId != 0 {
		res.DistrictId = newId
	}

	if newId, err := updateOrCreateDictionary(&res.DescriptionId, dto.Description); err != nil {
		return appresult.ErrInternalServer
	} else if newId != 0 {
		res.DescriptionId = newId
	}

	if dto.ProvinceId != 0 {
		var exists bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM provinces WHERE id = $1)`, dto.ProvinceId).Scan(&exists); err != nil || !exists {
			return appresult.ErrNotFoundType(dto.ProvinceId, "province")
		}
		res.ProvinceId = dto.ProvinceId
	}

	if dto.Phone != "" {
		res.Phone = dto.Phone
	}

	if dto.Expires != nil {
		res.Expires = *dto.Expires
	}

	if dto.ClosesTime != "" {
		_, err = time.Parse(formatTime, dto.ClosesTime)
		if err != nil {
			return appresult.ErrTime("closes")
		}
		res.ClosesTime = dto.ClosesTime
	}

	if dto.OpensTime != "" {
		_, err = time.Parse(formatTime, dto.OpensTime)
		if err != nil {
			return appresult.ErrTime("opens")
		}
		res.OpensTime = dto.OpensTime
	}

	if dto.CategoryId != nil {

		var exists bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM categories WHERE id = $1)`, dto.ProvinceId).Scan(&exists); err != nil || !exists {
			return appresult.ErrNotFoundType(*dto.CategoryId, "category")
		}
		res.CategoryId = dto.CategoryId
	}

	if dto.Value != nil {
		res.Value = dto.Value
	}

	if dto.CanOrder != nil {
		res.CanOrder = *dto.CanOrder
	}

	if dto.CanReserve != nil {
		res.CanReserve = *dto.CanReserve
	}

	if _, err = tx.Exec(ctx, `
		UPDATE businesses
		SET province_id = $1, phone = $2, name = $3,
			opens_time = $4, closes_time = $5,
			expires = $6, value = $7, category_id = $8,
			can_order = $9, can_reserve = $10
		WHERE id = $11
	`, res.ProvinceId, res.Phone, res.Name,
		res.OpensTime, res.ClosesTime, res.Expires,
		*res.Value, *res.CategoryId,
		res.CanOrder, res.CanReserve,
		businessId); err != nil {
		fmt.Println("error: ", err)
		return appresult.ErrInternalServer
	}

	if dto.SubcategoryIds != nil {
		if _, err = tx.Exec(ctx, `DELETE FROM businesses_subcategories WHERE businesses_id = $1`, businessId); err != nil {
			fmt.Println("error: ", err)
			return appresult.ErrInternalServer
		}

		valueStrings := []string{}
		valueArgs := []interface{}{}
		argIndex := 1
		for _, subcategoruId := range *dto.SubcategoryIds {
			valueStrings = append(valueStrings, fmt.Sprintf("($%d, $%d)", argIndex, argIndex+1))
			valueArgs = append(valueArgs, businessId, subcategoruId)
			argIndex += 2
		}
		query := `INSERT INTO businesses_subcategories (businesses_id, subcategory_id) VALUES ` + strings.Join(valueStrings, ", ")
		if _, err = tx.Exec(ctx, query, valueArgs...); err != nil {
			fmt.Println("error: ", err)
			return err
		}
	}

	if err = tx.Commit(ctx); err != nil {
		fmt.Println("error: ", err)
		return appresult.ErrInternalServer
	}

	return nil
}

func (r *repository) GetAndDeleteImage(ctx context.Context, businessId int, isMain bool) (*[]string, error) {
	var images []string
	query := `
			DELETE FROM image_businesses
		WHERE businesses_id = $1 AND is_main = $2
		RETURNING image_path
	`
	rows, err := r.client.Query(ctx, query, businessId, isMain)
	if err != nil {
		fmt.Println("error: ", err)
		return nil, appresult.ErrInternalServer
	}
	defer rows.Close()

	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			fmt.Println("error: ", err)
			return nil, appresult.ErrInternalServer
		}
		images = append(images, path)
	}

	return &images, nil
}

func (r *repository) Delete(ctx context.Context, businessId int) error {
	var exist bool

	tx, err := r.client.Begin(ctx)
	if err != nil {
		return appresult.ErrInternalServer
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)
			panic(p)
		}
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	q := `
	SELECT EXISTS (
		SELECT 1 FROM businesses WHERE id = $1
	)
`

	err = tx.QueryRow(ctx, q, businessId).Scan(&exist)
	if err != nil {
		return err
	}

	if !exist {
		return appresult.ErrNotFoundType(businessId, "businesses")
	}

	queries := []string{
		`DELETE FROM businesses_subcategories WHERE businesses_id = $1`,
		`DELETE FROM image_businesses WHERE businesses_id = $1`,
		`DELETE FROM user_businesses WHERE businesses_id = $1`,
		`DELETE FROM item_categories WHERE businesses_id = $1`,
		`DELETE FROM orders WHERE businesses_id = $1`,
		`DELETE FROM reservations WHERE businesses_id = $1`,
		`DELETE FROM items WHERE businesses_id = $1`,
		`DELETE FROM notifications WHERE businesses_id = $1`,

		`DELETE FROM businesses WHERE id = $1`,
	}

	for _, q := range queries {
		if _, err = tx.Exec(ctx, q, businessId); err != nil {
			fmt.Println("error: ", err)
			return err
		}
	}

	if err = tx.Commit(ctx); err != nil {
		return err
	}

	return nil
}

func (r *repository) UpdateStatus(ctx context.Context, businessId int, status businesses.UpdateStatus) error {

	var (
		existsBusinesses bool
	)

	query := `
		SELECT EXISTS (
			SELECT 1
			FROM businesses
			WHERE id = $1 AND status = 'PENDING'
		);
	`
	err := r.client.QueryRow(ctx, query, businessId).Scan(&existsBusinesses)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return appresult.ErrNotFoundType(businessId, "businesses")
		}
		return err
	}

	if !existsBusinesses {
		return appresult.ErrStatus
	}

	if strings.HasPrefix(status.Status, "CANCELED") && status.Reason == "" {
		return appresult.ErrReason
	}

	_, err = r.client.Exec(ctx, `
		UPDATE businesses
		SET status = $1, reason = $2, updated_at = now()
		WHERE id = $3
	`, status.Status, status.Reason, businessId)

	if err != nil {
		fmt.Println(err)
		return appresult.ErrInternalServer
	}

	return nil
}

func (r *repository) Index(ctx context.Context, filter businesses.IndexFilter, baseURL string) (*businesses.Index, error) {
	var (
		index          businesses.Index
		subcategoryIds []int
		categoryIds    []int
		err            error
	)

	subcategoryIds = append(subcategoryIds, filter.SubcategoryIds...)

	for _, catId := range filter.CategoryIds {
		var subs []int
		if err = r.client.QueryRow(ctx,
			`SELECT ARRAY(SELECT id FROM subcategories WHERE category_id = $1)`,
			catId,
		).Scan(&subs); err != nil {
			return nil, err
		}

		if len(subs) > 0 {
			subcategoryIds = append(subcategoryIds, subs...)
		} else {
			categoryIds = append(categoryIds, catId)
		}
	}

	if len(subcategoryIds) == 0 {
		subcategoryIds = nil
	}
	if len(categoryIds) == 0 {
		categoryIds = nil
	}

	baseQ := `
		SELECT DISTINCT ON (b.id)
			b.id,
			b.name,
			COALESCE(img.image_path, '') AS image_path,
			b.discount_percent,
			b.value
		FROM businesses b
		LEFT JOIN businesses_subcategories bs
			ON bs.businesses_id = b.id
		LEFT JOIN image_businesses img
			ON img.businesses_id = b.id AND img.is_main = true
		WHERE
			b.status = 'APPROVED'
			AND ($1 = 0 OR b.province_id = $1)
			%s
		ORDER BY b.id DESC
		LIMIT %d
	`

	const discountCondition = `
		AND (
			b.discount_percent != 0
			OR EXISTS (
				SELECT 1 FROM items i
				WHERE i.businesses_id = b.id
				AND i.discount_percent != 0
			)
		)`

	index.NewCreated, err = r.fetchWithFallback(ctx, baseQ, "", subcategoryIds, categoryIds, filter.ProvinceID, baseURL)
	if err != nil {
		return nil, err
	}

	index.WithDiscounts, err = r.fetchWithFallback(ctx, baseQ, discountCondition, subcategoryIds, categoryIds, filter.ProvinceID, baseURL)
	if err != nil {
		return nil, err
	}

	return &index, nil
}

func (r *repository) fetchWithFallback(
	ctx context.Context,
	baseQ string,
	extraCondition string,
	subcategoryIds []int,
	categoryIds []int,
	provinceID int,
	baseURL string,
) ([]businesses.IndexBusinesses, error) {
	const limit = 5
	var result []businesses.IndexBusinesses

	hasCategoryFilter := len(subcategoryIds) > 0 || len(categoryIds) > 0

	if hasCategoryFilter {
		q := fmt.Sprintf(baseQ, extraCondition+`
		AND (
			($2::int[] IS NOT NULL AND bs.subcategory_id = ANY($2::int[]))
			OR
			($3::int[] IS NOT NULL AND b.category_id = ANY($3::int[]))
		)`, limit)

		var err error
		result, err = findBusinessesForIndex(ctx, r, q, provinceID, subcategoryIds, categoryIds, nil, baseURL)
		if err != nil {
			return nil, err
		}
	}

	if len(result) < limit {
		excludeIDs := make([]int, 0, len(result))
		for _, b := range result {
			excludeIDs = append(excludeIDs, b.Id)
		}

		q := fmt.Sprintf(baseQ, extraCondition+`
		AND ($2::int[] IS NULL OR b.id != ALL($2::int[]))`, limit-len(result))

		extra, err := findBusinessesForIndex(ctx, r, q, provinceID, nil, nil, excludeIDs, baseURL)
		if err != nil {
			return nil, err
		}

		result = append(result, extra...)
	}

	return result, nil
}

func findBusinessesForIndex(
	ctx context.Context,
	r *repository,
	q string,
	provinceId int,
	subcategoryIds []int,
	categoryIds []int,
	excludeIds []int,
	baseURL string,
) ([]businesses.IndexBusinesses, error) {
	var (
		rows pgx.Rows
		err  error
	)

	if len(subcategoryIds) > 0 || len(categoryIds) > 0 {
		rows, err = r.client.Query(ctx, q, provinceId, subcategoryIds, categoryIds)
	} else {
		rows, err = r.client.Query(ctx, q, provinceId, excludeIds)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []businesses.IndexBusinesses
	for rows.Next() {
		var data businesses.IndexBusinesses
		if err := rows.Scan(&data.Id, &data.Name, &data.Image, &data.DiscountPercent, &data.Value); err != nil {
			return nil, err
		}

		if data.Image != "" {
			data.Image = fmt.Sprintf("%s/%s",
				baseURL,
				strings.ReplaceAll(data.Image, "\\", "/"),
			)
		}

		if data.Value != nil && data.DiscountPercent != nil && *data.DiscountPercent != 0 {
			x := math.Round((float64(*data.Value)*float64(*data.DiscountPercent))/10) / 10
			discountValue := float32(float64(*data.Value) - x)

			data.DiscountValue = &discountValue
		} else if *data.DiscountPercent == 0 {
			data.DiscountPercent = nil
		}

		result = append(result, data)
	}

	return result, rows.Err()
}
