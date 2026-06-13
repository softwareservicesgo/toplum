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
	Ingredient      []DictionaryDTO `json:"ingredients"`
	Content         DictionaryDTO   `json:"content"`
	Value           float64         `json:"value" binding:"required"`
	BusinessId      int             `json:"businesses_id" binding:"required"`
	ItemCategoryIds []int           `json:"item_category_ids" binding:"required"`
	Stock           *int            `json:"stock"`
}

type ItemGetOneDTO struct {
	Id              int             `json:"id"`
	Name            DictionaryDTO   `json:"name"`
	Ingredient      []DictionaryDTO `json:"ingredients"`
	Content         *DictionaryDTO  `json:"content"`
	ItemCategories  []DictionaryDTO `json:"item_categories"`
	ImagePath       string          `json:"image_path"`
	Value           float64         `json:"value"`
	DiscountPercent *int            `json:"discount_percent"`
	DiscountValue   *float64        `json:"discount_value"`
	Stock           *int            `json:"stock"`
}

type ItemGetAllDTO struct {
	Id              int           `json:"id"`
	Name            DictionaryDTO `json:"name"`
	ImagePath       string        `json:"image_path"`
	Value           float64       `json:"value"`
	DiscountPercent *int          `json:"discount_percent"`
	DiscountValue   *float64      `json:"discount_value"`
}

type GetAllWithCount struct {
	Count int             `json:"count"`
	Items []ItemGetAllDTO `json:"items"`
}

type DictionaryDTO struct {
	Tm string `json:"tm"`
	Ru string `json:"ru"`
	En string `json:"en"`
}

type ItemUpdateDTO struct {
	Name       DictionaryDTO   `json:"name"`
	Ingredient []DictionaryDTO `json:"ingredients"`
	Content    *DictionaryDTO  `json:"content"`
	ImagePath  string          `json:"image_path"`
	Value      float64         `json:"value"`
	Stock      *int            `json:"stock"`
}

type ItemForUpdateDTO struct {
	NameId       int
	IngredientId *int
	ContentId    *int
	ImagePath    string
	Value        float64
	Stock        *int
}

type Name struct {
	Id   int           `json:"id"`
	Name DictionaryDTO `json:"name"`
}
