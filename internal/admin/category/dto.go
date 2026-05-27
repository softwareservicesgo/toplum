package category

type CategoryDTO struct {
	Id        int           `json:"id"`
	Name      DictionaryDTO `json:"name"`
	ImagePath string        `json:"image_path"`
	HasDetail bool          `json:"has_detail"`
}

type DictionaryDTO struct {
	Tm string `json:"tm" binding:"required"`
	Ru string `json:"ru" binding:"required"`
	En string `json:"en" binding:"required"`
}

type CategoryReqDTO struct {
	Name      DictionaryDTO `json:"name"`
	HasDetail *bool         `json:"has_detail"`
}
type CategoryAllDTO struct {
	Categories []CategoryDTO `json:"categories"`
	Count      int           `json:"count"`
}
