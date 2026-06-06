package basket

type BasketReq struct {
	ItemId int `json:"item_id"`
}

type BasketsAll struct {
	Count               int       `json:"count"`
	BasketsByBusinesses []Baskets `json:"baskets_by_businesses"`
}

type Baskets struct {
	Businesses Businesses `json:"businesses"`
	Items      []Item     `json:"items"`
}

type DictionaryDTO struct {
	Tm string `json:"tm" binding:"required"`
	Ru string `json:"ru" binding:"required"`
	En string `json:"en" binding:"required"`
}

type Businesses struct {
	Id          int     `json:"id"`
	Name        string  `json:"name"`
	Image       string  `json:"image"`
	CountItems  int     `json:"count_items"`
	GeneralBill float64 `json:"general_bill"`
}

type Item struct {
	Id            int           `json:"id"`
	Image         string        `json:"image"`
	Name          DictionaryDTO `json:"name"`
	Value         float64       `json:"value"`
	DiscountValue *float64      `json:"discount_value"`
	Count         int           `json:"count"`
}
