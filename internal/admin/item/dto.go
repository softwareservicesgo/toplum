package item

type ItemFilter struct {
	Search             string `form:"search"`
	Limit              int    `form:"limit"`
	Offset             int    `form:"offset"`
	BusinessId         int    `form:"businesses_id"`
	ItemCategoryIdsStr string `form:"item_category_ids"`
	IsDiscounted       *bool  `form:"is_discounted"`
	SortByValue        string `form:"sort_by_value" binding:"omitempty,oneof=ASC DESC"`
	ItemCategoryIds    []int
}

type ItemReqDTO struct {
	Name            DictionaryDTO   `json:"name" binding:"required"`
	Ingredient      []DictionaryDTO `json:"ingredients" binding:"required"`
	Value           float32         `json:"value" binding:"required"`
	BusinessId      int             `json:"businesses_id" binding:"required"`
	ItemCategoryIds []int           `json:"item_category_ids" binding:"required"`
}

type ItemGetOneDTO struct {
	Id              int             `json:"id"`
	Name            DictionaryDTO   `json:"name"`
	Ingredient      []DictionaryDTO `json:"ingredients"`
	ItemCategories  []DictionaryDTO `json:"item_categories"`
	ImagePath       string          `json:"image_path"`
	Value           float32         `json:"value"`
	DiscountPercent *int            `json:"discount_percent"`
	DiscountValue   *float32        `json:"discount_value"`
}

type ItemGetAllDTO struct {
	Id              int           `json:"id"`
	Name            DictionaryDTO `json:"name"`
	ImagePath       string        `json:"image_path"`
	Value           float32       `json:"value"`
	DiscountPercent *int          `json:"discount_percent"`
	DiscountValue   *float32      `json:"discount_value"`
}

type GetAllWithCount struct {
	Count int             `json:"count"`
	Items []ItemGetAllDTO `json:"items"`
}

type ItemForUpdateDTO struct {
	NameId          int
	IngredientId    int
	Value           float32
	BusinessId      int
	ItemCategoryIds []int
	ImagePath       string
	DiscountPercent int
}

type DictionaryDTO struct {
	Tm string `json:"tm" binding:"required"`
	Ru string `json:"ru" binding:"required"`
	En string `json:"en" binding:"required"`
}

type ItemResForUpdateDTO struct {
	Id              int             `json:"id"`
	Name            DictionaryDTO   `json:"name"`
	Ingredient      []DictionaryDTO `json:"ingredients"`
	ImagePath       string          `json:"image_path"`
	Value           float32         `json:"value"`
	ItemCategories  []Name          `json:"item_categories"`
	DiscountPercent int             `json:"discount_percent"`
}

type Name struct {
	Id   int           `json:"id"`
	Name DictionaryDTO `json:"name"`
}
