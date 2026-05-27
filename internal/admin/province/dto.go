package province

type ProvinceDTO struct {
	Id   int           `json:"id"`
	Name DictionaryDTO `json:"name"`
}

type DictionaryDTO struct {
	Tm string `json:"tm" binding:"required"`
	Ru string `json:"ru" binding:"required"`
	En string `json:"en" binding:"required"`
}
