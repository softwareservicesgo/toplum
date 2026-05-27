package itemCategory

type ItemCategoryCreateDTO struct {
	Name       DictionaryDTO `json:"name"`
	BusinessID int           `json:"businesses_id"`
}

type ItemCategoryDTO struct {
	ID         int           `json:"id"`
	Name       DictionaryDTO `json:"name"`
	BusinessID int           `json:"businesses_id"`
}

type DictionaryDTO struct {
	Tm string `json:"tm" binding:"required"`
	Ru string `json:"ru" binding:"required"`
	En string `json:"en" binding:"required"`
}

type ItemCategoryAllDTO struct {
	Categories []ItemCategoryDTO `json:"categories"`
	Count      int               `json:"count"`
}

type ItemCategoryNameDTO struct {
	Name DictionaryDTO `json:"name"`
}
