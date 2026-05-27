package db

import (
	"context"
	"errors"
	"fmt"
	"math"
	"restaurants/internal/admin/item"
	"restaurants/internal/appresult"
	"restaurants/pkg/client/postgresql"
	"restaurants/pkg/logging"
	"restaurants/pkg/utils"
	"strings"

	"github.com/jackc/pgx/v4"
	"github.com/lib/pq"
)

type repository struct {
	client postgresql.Client
	logger *logging.Logger
}

func NewRepository(client postgresql.Client, logger *logging.Logger) item.Repository {
	return &repository{
		client: client,
		logger: logger,
	}
}

func (r *repository) Create(ctx context.Context, dto item.ItemReqDTO, imagePath string, baseURL string) (*item.ItemGetOneDTO, error) {
	var (
		businessExists                       bool
		itemId, nameDictId, ingredientDictId int
		ingrTm, ingrEn, ingrRu               string
	)

	tx, err := r.client.Begin(ctx)
	if err != nil {
		fmt.Println("error: ", err)
		return nil, fmt.Errorf("begin tx: %w", err)
	}

	defer func() {
		if rbErr := tx.Rollback(ctx); rbErr != nil && !errors.Is(rbErr, pgx.ErrTxClosed) {
			fmt.Println("rollback error:", rbErr)
		}
	}()

	q := `SELECT EXISTS(SELECT 1 FROM businesses WHERE id=$1)`
	err = tx.QueryRow(ctx, q, dto.BusinessId).Scan(&businessExists)
	if err != nil || !businessExists {
		return nil, appresult.ErrNotFoundType(dto.BusinessId, "business")
	}

	q = `INSERT INTO dictionary (tm, en, ru) VALUES ($1,$2,$3) RETURNING id`
	err = tx.QueryRow(ctx, q, dto.Name.Tm, dto.Name.En, dto.Name.Ru).Scan(&nameDictId)
	if err != nil {
		fmt.Println("error: ", err)
		return nil, err
	}

	ingrTm, ingrRu, ingrEn = toString(dto.Ingredient)

	q = `INSERT INTO dictionary (tm, en, ru) VALUES ($1,$2,$3) RETURNING id`
	err = tx.QueryRow(ctx, q, ingrTm, ingrEn, ingrRu).Scan(&ingredientDictId)
	if err != nil {
		fmt.Println("error: ", err)
		return nil, err
	}

	dto.Value = float32(math.Round(float64(dto.Value)*10) / 10)

	q = `
		INSERT INTO items (name_dictionary_id, ingredient_dictionary_id, 
							image_path, value, businesses_id )
		VALUES ($1,$2,$3,$4,$5)
		RETURNING id
	`
	err = tx.QueryRow(ctx, q,
		nameDictId, ingredientDictId, imagePath, dto.Value, dto.BusinessId,
	).Scan(&itemId)
	if err != nil {
		fmt.Println("error: ", err)
		return nil, err
	}

	if len(dto.ItemCategoryIds) > 0 {

		var count int

		checkQuery := `
		SELECT COUNT(*)
		FROM item_categories
		WHERE id = ANY($1)
		AND businesses_id = $2
	`

		err = tx.QueryRow(
			ctx,
			checkQuery,
			dto.ItemCategoryIds,
			dto.BusinessId,
		).Scan(&count)

		if err != nil {
			return nil, appresult.ErrInternalServer
		}

		if count != len(dto.ItemCategoryIds) {
			return nil, appresult.ErrNotFoundTypeStr("item_category in this business")
		}

		q = `INSERT INTO items_item_categories (item_id, item_category_id) VALUES ($1,$2)`
		for _, catId := range dto.ItemCategoryIds {
			_, err = tx.Exec(ctx, q, itemId, catId)
			if err != nil {
				fmt.Println("error: ", err)
				return nil, fmt.Errorf("failed to link category: %w", err)
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}

	resp, err := r.GetOne(ctx, itemId, baseURL)
	if err != nil {
		fmt.Println("error: ", err)
		return nil, err
	}

	return resp, nil
}

func (r *repository) GetOne(ctx context.Context, itemId int, baseURL string) (*item.ItemGetOneDTO, error) {
	var (
		dto                 item.ItemGetOneDTO
		ingredient          item.DictionaryDTO
		itemCategories      []item.DictionaryDTO
		ingTm, ingRu, ingEn *string
	)
	dto.ItemCategories = []item.DictionaryDTO{}
	dto.Ingredient = []item.DictionaryDTO{}

	q := `
		SELECT 
			i.id,
			dn.tm, dn.ru, dn.en,
			di.tm, di.ru, di.en,
			i.value,
			i.image_path,
			i.discount_percent
		FROM items i
		JOIN dictionary dn ON i.name_dictionary_id = dn.id
		LEFT JOIN dictionary di ON i.ingredient_dictionary_id = di.id
		WHERE i.id = $1
	`
	err := r.client.QueryRow(ctx, q, itemId).Scan(
		&dto.Id,
		&dto.Name.Tm, &dto.Name.Ru, &dto.Name.En,
		&ingTm, &ingRu, &ingEn,
		&dto.Value,
		&dto.ImagePath,
		&dto.DiscountPercent,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, appresult.ErrNotFoundType(itemId, "item")
		}
		return nil, err
	}

	if ingTm != nil && ingRu != nil && ingEn != nil {
		ingredient.Tm = *ingTm
		ingredient.Ru = *ingRu
		ingredient.En = *ingEn
		dto.Ingredient = SplitDictionary(ingredient)
	}

	if dto.Value != 0 && dto.DiscountPercent != nil && *dto.DiscountPercent != 0 {
		x := math.Round((float64(dto.Value)*float64(*dto.DiscountPercent))/10) / 10
		discountValue := float32(float64(dto.Value) - x)

		dto.DiscountValue = &discountValue
	} else if *dto.DiscountPercent == 0 {
		dto.DiscountPercent = nil
	}

	q = `
        SELECT d.tm, d.ru, d.en
        FROM items_item_categories iic
		JOIN item_categories ic ON ic.id = iic.item_category_id
        JOIN dictionary d ON ic.name_dictionary_id = d.id
        WHERE iic.item_id = $1
    `
	rows, err := r.client.Query(ctx, q, itemId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var category item.DictionaryDTO
		if err := rows.Scan(&category.Tm, &category.Ru, &category.En); err != nil {
			return nil, err
		}

		itemCategories = append(itemCategories, category)
	}

	if len(itemCategories) > 0 {
		dto.ItemCategories = itemCategories
	}

	if dto.ImagePath != "" && baseURL != "" {
		cleanPath := strings.ReplaceAll(dto.ImagePath, "\\", "/")
		dto.ImagePath = fmt.Sprintf("%s/%s", baseURL, cleanPath)
	}
	return &dto, nil
}

func (r *repository) GetAll(ctx context.Context, filter item.ItemFilter, baseURL string) (*item.GetAllWithCount, error) {
	var result item.GetAllWithCount

	if filter.Offset < 1 {
		filter.Offset = 1
	}
	if filter.Limit < 1 {
		filter.Limit = 10
	}

	args, q, qCount := makeItemFilter(filter)

	if err := r.client.QueryRow(ctx, qCount, args...).Scan(&result.Count); err != nil {
		fmt.Println("countQuery error: ", err)
		return nil, appresult.ErrInternalServer
	}

	offset := (filter.Offset - 1) * filter.Limit
	q += fmt.Sprintf(" LIMIT %d OFFSET %d", filter.Limit, offset)

	rows, err := r.client.Query(ctx, q, args...)
	if err != nil {
		fmt.Println("error: ", err)
		return nil, appresult.ErrInternalServer
	}
	defer rows.Close()

	for rows.Next() {
		var it item.ItemGetAllDTO
		if err := rows.Scan(
			&it.Id,
			&it.Name.Tm, &it.Name.Ru, &it.Name.En,
			&it.Value,
			&it.ImagePath,
			&it.DiscountPercent,
		); err != nil {
			fmt.Println("error: ", err)
			return nil, appresult.ErrInternalServer
		}

		if it.Value != 0 && it.DiscountPercent != nil && *it.DiscountPercent != 0 {
			x := math.Round((float64(it.Value)*float64(*it.DiscountPercent))/10) / 10
			discountValue := float32(float64(it.Value) - x)

			it.DiscountValue = &discountValue
		} else if *it.DiscountPercent == 0 {
			it.DiscountPercent = nil
		}

		if it.ImagePath != "" && baseURL != "" {
			cleanPath := strings.ReplaceAll(it.ImagePath, "\\", "/")
			it.ImagePath = fmt.Sprintf("%s/%s", baseURL, cleanPath)
		}

		result.Items = append(result.Items, it)
	}

	if result.Items == nil {
		result.Items = []item.ItemGetAllDTO{}
	}

	return &result, nil
}

func makeItemFilter(filter item.ItemFilter) ([]interface{}, string, string) {
	var args []interface{}
	idx := 1

	filters := []string{"1=1"}

	if filter.BusinessId != 0 {
		filters = append(filters, fmt.Sprintf("i.businesses_id = $%d", idx))
		args = append(args, filter.BusinessId)
		idx++
	}

	if len(filter.ItemCategoryIds) > 0 {
		filters = append(filters, fmt.Sprintf("EXISTS (SELECT 1 FROM items_item_categories bs WHERE bs.businesses_id = b.id AND bs.subcategory_id = ANY($%d))", idx))
		args = append(args, pq.Array(filter.ItemCategoryIds))
		idx++
	}

	if filter.IsDiscounted != nil && *filter.IsDiscounted {
		filters = append(filters, "i.discount_percent != 0")
	}

	if filter.Search != "" {
		filters = append(filters, fmt.Sprintf(`(
			dn.tm ILIKE '%%' || $%d || '%%'
			OR dn.en ILIKE '%%' || $%d || '%%'
			OR dn.ru ILIKE '%%' || $%d || '%%'
		)`, idx, idx, idx))
		args = append(args, filter.Search)
		idx++
	}

	sort := "ORDER BY i.created_at DESC"
	if filter.SortByValue != "" {
		filters = append(filters, "i.value IS NOT NULL")
		sort = fmt.Sprintf("ORDER BY i.value %s", filter.SortByValue)
	}

	baseQuery := `
		FROM items i
		JOIN dictionary dn ON i.name_dictionary_id = dn.id
		WHERE %s
	`

	q := fmt.Sprintf(`
		SELECT
			i.id,
			dn.tm, dn.ru, dn.en,
			i.value,
			i.image_path,
			i.discount_percent
		`+baseQuery+` %s`, strings.Join(filters, " AND "), sort)

	qCount := fmt.Sprintf(`
		SELECT COUNT(DISTINCT i.id)
		`+baseQuery, strings.Join(filters, " AND "))

	return args, q, qCount
}

func (r *repository) Update(ctx context.Context, itemId int, dto item.ItemReqDTO, imagePath string, baseURL string) (*item.ItemResForUpdateDTO, error) {
	var itm item.ItemForUpdateDTO

	tx, err := r.client.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		if rbErr := tx.Rollback(ctx); rbErr != nil && !errors.Is(rbErr, pgx.ErrTxClosed) {
			fmt.Println("rollback error:", rbErr)
		}
	}()

	q := `SELECT 
		name_dictionary_id, ingredient_dictionary_id, 
		image_path, value, businesses_id
		FROM items WHERE id = $1`
	err = tx.QueryRow(ctx, q, itemId).Scan(
		&itm.NameId, &itm.IngredientId, &itm.ImagePath,
		&itm.Value, &itm.BusinessId,
	)
	if err != nil {
		return nil, appresult.ErrNotFoundType(itemId, "item")
	}

	if dto.Name.En != "" && dto.Name.Ru != "" && dto.Name.Tm != "" {
		_, err = tx.Exec(ctx, `UPDATE dictionary SET tm=$1, en=$2, ru=$3 WHERE id=$4`,
			dto.Name.Tm, dto.Name.En, dto.Name.Ru, itm.NameId)
		if err != nil {
			return nil, appresult.ErrInternalServer
		}
	}

	if len(dto.Ingredient) > 0 {
		ingrTm, ingrRu, ingrEn := toString(dto.Ingredient)
		_, err = tx.Exec(ctx, `UPDATE dictionary SET tm=$1, en=$2, ru=$3 WHERE id=$4`,
			ingrTm, ingrEn, ingrRu, itm.IngredientId)
		if err != nil {
			return nil, appresult.ErrInternalServer
		}
	}

	if dto.BusinessId != 0 {
		var exists bool
		err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM businesses WHERE id=$1)`, dto.BusinessId).Scan(&exists)
		if err != nil || !exists {
			return nil, appresult.ErrNotFoundType(dto.BusinessId, "business")
		}
		itm.BusinessId = dto.BusinessId
	}

	newImage := itm.ImagePath
	if imagePath != "" {
		utils.DropFiles(&[]string{itm.ImagePath})
		newImage = imagePath
	}

	if dto.Value != 0 || imagePath != "" {
		_, err = tx.Exec(ctx, `UPDATE items SET value=$1, image_path=$2 WHERE id=$3`,
			dto.Value, newImage, itemId)
		if err != nil {
			return nil, appresult.ErrInternalServer
		}
	}

	if len(dto.ItemCategoryIds) > 0 {
		var count int
		checkQuery := `SELECT COUNT(*) FROM item_categories WHERE id = ANY($1) AND businesses_id = $2`
		err = tx.QueryRow(ctx, checkQuery, dto.ItemCategoryIds, dto.BusinessId).Scan(&count)
		if err != nil {
			return nil, appresult.ErrInternalServer
		}
		if count != len(dto.ItemCategoryIds) {
			return nil, appresult.ErrNotFoundTypeStr("item_category in this business")
		}
		_, err = tx.Exec(ctx, `DELETE FROM items_item_categories WHERE item_id=$1`, itemId)
		if err != nil {
			return nil, appresult.ErrInternalServer
		}
		for _, catId := range dto.ItemCategoryIds {
			_, err = tx.Exec(ctx, `INSERT INTO items_item_categories (item_id, item_category_id) VALUES ($1, $2)`, itemId, catId)
			if err != nil {
				return nil, appresult.ErrInternalServer
			}
		}
	}

	if err = tx.Commit(ctx); err != nil {
		fmt.Println("error: ", err)
		return nil, appresult.ErrInternalServer
	}

	updated, err := r.GetForUpdate(ctx, itemId, baseURL)
	if err != nil {
		return nil, appresult.ErrInternalServer
	}

	return updated, nil
}

func (r *repository) Delete(ctx context.Context, itemId int) error {
	var (
		nameDictionaryId       int
		ingredientDictionaryId int
		imagePath              string
	)

	q := `
		SELECT name_dictionary_id, ingredient_dictionary_id, image_path
		FROM items
		WHERE id = $1
	`

	err := r.client.QueryRow(ctx, q, itemId).Scan(&nameDictionaryId, &ingredientDictionaryId, &imagePath)
	if err != nil {
		fmt.Println("error: ", err)
		if errors.Is(err, pgx.ErrNoRows) {
			return appresult.ErrNotFoundType(itemId, "item")
		}
		return appresult.ErrInternalServer
	}

	if imagePath != "" {
		utils.DropFiles(&[]string{imagePath})
	}

	_, err = r.client.Exec(ctx, `DELETE FROM items_item_categories WHERE item_id=$1`, itemId)
	if err != nil {
		fmt.Println("error: ", err)
		return appresult.ErrInternalServer
	}

	_, err = r.client.Exec(ctx, `DELETE FROM items WHERE id=$1`, itemId)
	if err != nil {
		fmt.Println("error: ", err)
		return appresult.ErrInternalServer
	}

	if nameDictionaryId != 0 {
		_, err = r.client.Exec(ctx, `DELETE FROM dictionary WHERE id=$1`, nameDictionaryId)
		if err != nil {
			fmt.Println("error: ", err)
			return appresult.ErrInternalServer
		}
	}

	if ingredientDictionaryId != 0 {
		_, err = r.client.Exec(ctx, `DELETE FROM dictionary WHERE id=$1`, ingredientDictionaryId)
		if err != nil {
			fmt.Println("error: ", err)
			return appresult.ErrInternalServer
		}
	}

	return nil
}

func (r *repository) GetForUpdate(ctx context.Context, itemId int, baseURL string) (*item.ItemResForUpdateDTO, error) {
	var (
		dto  item.ItemResForUpdateDTO
		ingr item.DictionaryDTO
	)

	q := `
		SELECT 
			i.id,
			dn.tm, dn.ru, dn.en,
			di.tm, di.ru, di.en,
			i.value,
			i.image_path
		FROM items i
		JOIN dictionary dn ON i.name_dictionary_id = dn.id
		JOIN dictionary di ON i.ingredient_dictionary_id = di.id
		WHERE i.id = $1
	`
	err := r.client.QueryRow(ctx, q, itemId).Scan(
		&dto.Id,
		&dto.Name.Tm, &dto.Name.Ru, &dto.Name.En,
		&ingr.Tm, &ingr.Ru, &ingr.En,
		&dto.Value,
		&dto.ImagePath,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, appresult.ErrNotFoundType(itemId, "item")
		}
		return nil, err
	}

	dto.Ingredient = SplitDictionary(ingr)

	q = `
        SELECT d.id, d.tm, d.ru, d.en
        FROM items_item_categories iic
        JOIN item_categories ic ON iic.item_category_id = ic.id
        JOIN dictionary d ON ic.name_dictionary_id = d.id
        WHERE iic.item_id = $1
    `
	rows, err := r.client.Query(ctx, q, itemId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var cat item.Name
		if err := rows.Scan(&cat.Id, &cat.Name.Tm, &cat.Name.Ru, &cat.Name.En); err != nil {
			return nil, err
		}
		dto.ItemCategories = append(dto.ItemCategories, cat)
	}

	if dto.ImagePath != "" && baseURL != "" {
		cleanPath := strings.ReplaceAll(dto.ImagePath, "\\", "/")
		dto.ImagePath = fmt.Sprintf("%s/%s", baseURL, cleanPath)
	}

	return &dto, nil
}

func (r *repository) GetItemsByBusiness(ctx context.Context, businessId int, baseURL string) (*[]item.ItemGetAllDTO, error) {
	var result []item.ItemGetAllDTO

	q := `
		(
			SELECT 
				i.id,
				dn.tm, dn.ru, dn.en,
				i.value,
				i.image_path
			FROM items i
			JOIN dictionary dn ON i.name_dictionary_id = dn.id
			WHERE i.businesses_id = $1
			ORDER BY i.created_at DESC
			LIMIT 5
		)
		UNION ALL
		(
			SELECT 
				i.id,
				dn.tm, dn.ru, dn.en,
				i.value,
				i.image_path
			FROM items i
			JOIN dictionary dn ON i.name_dictionary_id = dn.id
			WHERE i.businesses_id = $1
			ORDER BY i.created_at DESC
			LIMIT 5
		)
		LIMIT 5;
	`

	rows, err := r.client.Query(ctx, q, businessId)
	if err != nil {
		fmt.Println("error: ", err)
		return nil, appresult.ErrInternalServer
	}
	defer rows.Close()

	for rows.Next() {
		var it item.ItemGetAllDTO
		if err := rows.Scan(
			&it.Id,
			&it.Name.Tm, &it.Name.Ru, &it.Name.En,
			&it.Value,
			&it.ImagePath,
		); err != nil {
			fmt.Println("error: ", err)
			return nil, appresult.ErrInternalServer
		}

		if it.ImagePath != "" && baseURL != "" {
			cleanPath := strings.ReplaceAll(it.ImagePath, "\\", "/")
			it.ImagePath = fmt.Sprintf("%s/%s", baseURL, cleanPath)
		}

		result = append(result, it)
	}

	return &result, nil
}

func SplitDictionary(dict item.DictionaryDTO) []item.DictionaryDTO {
	var result []item.DictionaryDTO
	tmParts := strings.Split(dict.Tm, "/")
	ruParts := strings.Split(dict.Ru, "/")
	enParts := strings.Split(dict.En, "/")

	for i, _ := range tmParts {
		var item item.DictionaryDTO

		item.Tm = strings.TrimSpace(tmParts[i])
		item.Ru = strings.TrimSpace(ruParts[i])
		item.En = strings.TrimSpace(enParts[i])

		result = append(result, item)
	}
	return result
}

func toString(dto []item.DictionaryDTO) (string, string, string) {
	var ingrTm, ingrEn, ingrRu string
	for indext, ing := range dto {
		if indext != 0 {
			ingrEn = ingrEn + " / " + ing.En
			ingrRu = ingrRu + " / " + ing.Ru
			ingrTm = ingrTm + " / " + ing.Tm
		} else {
			ingrEn = ingrEn + ing.En
			ingrRu = ingrRu + ing.Ru
			ingrTm = ingrTm + ing.Tm
		}
	}

	return ingrTm, ingrRu, ingrEn
}
