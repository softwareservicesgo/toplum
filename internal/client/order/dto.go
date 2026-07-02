package order

import (
	"restaurants/internal/client/basket"
	"restaurants/internal/client/user"
)

type CreateOrderReq struct {
	BusinessesIds []int  `json:"businesses_ids"`
	Place         string `json:"place"`
	OrderTime     string `json:"order_time" binding:"required,datetime=2006-01-02 15:04"`
}

type Basket struct {
	ItemId          int
	Price           float64
	Count           int
	DiscountPercent *int
}

type OrderOne struct {
	Id              int           `json:"id"`
	BusinessesId    int           `json:"businesses_id"`
	BusinessesName  string        `json:"businesses_name"`
	BusinessesImage string        `json:"businesses_image"`
	GeneralBill     float64       `json:"general_bill"`
	Status          string        `json:"status"`
	CountItems      int           `json:"count_items"`
	Place           string        `json:"place"`
	OrderTime       string        `json:"order_time" binding:"required,datetime=2006-01-02 15:04"`
	Items           []basket.Item `json:"items"`
	ApprovedBy      *user.User    `json:"approved_by"`
}

type OrderAllForClient struct {
	Count  int        `json:"count"`
	Orders []OrderOne `json:"orders"`
}

type OrderAllForBusinesses struct {
	Count  int                   `json:"count"`
	Orders []OrdersForBusinesses `json:"orders"`
}

type OrdersForBusinesses struct {
	Id          int     `json:"id"`
	Client      Client  `json:"client"`
	GeneralBill float64 `json:"general_bill"`
	CountItems  int     `json:"count_items"`
	Status      string  `json:"status"`
}

type Client struct {
	Id          int    `json:"id"`
	FullName    string `json:"full_name"`
	ImagePath   string `json:"image_path"`
	PhoneNumber string `json:"phone_number"`
}

type UpdateOrderReq struct {
	Place     string       `json:"place" binding:"required"`
	OrderTime string       `json:"order_time" binding:"required,datetime=2006-01-02 15:04"`
	Items     []UpdateItem `json:"items" binding:"required"`
}

type UpdateItem struct {
	ItemID   int `json:"item_id"`
	Quantity int `json:"quantity"`
}

type UpdateOrderStatusReq struct {
	Status string `json:"status" binding:"required"`
	Reason string `json:"reason"`
}

type DictionaryDTO struct {
	Tm string `json:"tm" binding:"required"`
	Ru string `json:"ru" binding:"required"`
	En string `json:"en" binding:"required"`
}
