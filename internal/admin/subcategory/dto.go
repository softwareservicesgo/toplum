package subcategory

type DictionaryDTO struct {
	Tm string `json:"tm"`
	En string `json:"en"`
	Ru string `json:"ru"`
}

type SubcategoryDTO struct {
	Name       DictionaryDTO `json:"name"`
	CategoryID int           `json:"category_id"`
}

type SubcategoryOneDTO struct {
	Id        int              `json:"id"`
	Name      DictionaryDTO    `json:"name"`
	Category  SubcategoriesDTO `json:"category"`
	ImagePath string           `json:"image_path"`
}

type SubcategoriesDTO struct {
	Id        int           `json:"id"`
	Name      DictionaryDTO `json:"name"`
	ImagePath string        `json:"image_path"`
}

type SubcategoryAllDTO struct {
	Subcategories []SubcategoriesDTO `json:"subcategories"`
	Count         int                `json:"count"`
}

type ByCategory struct {
	CategoryName DictionaryDTO `json:"category_name"`
	Subcategories []SubcategoriesDTO `json:"subcategories"`
}
