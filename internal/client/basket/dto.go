package basket

type BasketReq struct {
	ItemId int `json:"item_id"`
	Count  int `json:"count"`
}

type BasketsAll struct {
	Count               int       `json:"count"`
	BasketsByBusinesses []Baskets `json:"baskets_by_businesses"`
}

type Baskets struct {
	Businesses Businessess `json:"businesses"`
	Items      []Item      `json:"items"`
}

type Basket struct {
	Businesses Businesses `json:"businesses"`
	Items      []Item     `json:"items"`
}

type DictionaryDTO struct {
	Tm string `json:"tm" binding:"required"`
	Ru string `json:"ru" binding:"required"`
	En string `json:"en" binding:"required"`
}

type Businessess struct {
	Id          int     `json:"id"`
	Name        string  `json:"name"`
	CountItems  int     `json:"count_items"`
	GeneralBill float64 `json:"general_bill"`
}

type Businesses struct {
	Id             int            `json:"id"`
	Name           string         `json:"name"`
	CountItems     int            `json:"count_items"`
	GeneralBill    float64        `json:"general_bill"`
	ClientCouponId *int           `json:"client_coupon_id"`
	Coupon         *DictionaryDTO `json:"client_coupon"`
	BillWithCoupon *float64       `json:"bill_with_coupon"`
}

type Item struct {
	Id     int           `json:"id"`
	Images string        `json:"images"`
	Name   DictionaryDTO `json:"name"`
	Value  float64       `json:"value"`
	Count  int           `json:"count"`
}
