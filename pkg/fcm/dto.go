package fcm

type NotificationDTO struct {
	Id         int           `json:"id"`
	Restaurant Restaurant    `json:"restaurant"`
	Title      DictionaryDTO `json:"title"`
	Content    DictionaryDTO `json:"content"`
	LifeDay    int           `json:"life_day"`
	CreatedAt  string        `json:"created_at" binding:"datetime=2006-01-02 15:04"`
}

type DictionaryDTO struct {
	Tm string `json:"tm" binding:"required"`
	Ru string `json:"ru" binding:"required"`
	En string `json:"en" binding:"required"`
}

type Restaurant struct {
	Id   int    `json:"id"`
	Name string `json:"name"`
}
