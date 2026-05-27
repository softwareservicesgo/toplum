package order

import (
	"restaurants/internal/client/basket"
	"time"
)

type CreateOrderReq struct {
	BusinessesId   int    `json:"businesses_id"`
	ClientCouponId *int   `json:"client_coupon_id"`
	Place          string `json:"place"`
	OrderTime      string `json:"order_time" binding:"required,datetime=2006-01-02 15:04"`
}

type Basket struct {
	ItemId int
	Price  float64
	Count  int
}

type OrderOne struct {
	Id             int            `json:"id"`
	BusinessesId   int            `json:"businesses_id"`
	BusinessesName string         `json:"businesses_name"`
	GeneralBill    float64        `json:"general_bill"`
	ClientCouponId *int           `json:"client_coupon_id"`
	Coupon         *DictionaryDTO `json:"client_coupon"`
	BillWithCoupon *float64       `json:"bill_with_coupon"`
	Status         string         `json:"status"`
	CountItems     int            `json:"count_items"`
	Items          []basket.Item  `json:"items"`
}

type OrderAllForClient struct {
	Count  int               `json:"count"`
	Orders []OrdersForClient `json:"orders"`
}

type OrdersForClient struct {
	Id             int            `json:"id"`
	BusinessesName string         `json:"businesses_name"`
	GeneralBill    float64        `json:"general_bill"`
	Coupon         *DictionaryDTO `json:"client_coupon"`
	CountItems     int            `json:"count_items"`
	Status         string         `json:"status"`
}

type OrderAllForBusinesses struct {
	Count  int                   `json:"count"`
	Orders []OrdersForBusinesses `json:"orders"`
}

type OrdersForBusinesses struct {
	Id          int            `json:"id"`
	Client      Client         `json:"client"`
	GeneralBill float64        `json:"general_bill"`
	Coupon      *DictionaryDTO `json:"client_coupon"`
	CountItems  int            `json:"count_items"`
	Status      string         `json:"status"`
}

type Client struct {
	Id          int    `json:"id"`
	FullName    string `json:"full_name"`
	ImagePath   string `json:"image_path"`
	PhoneNumber string `json:"phone_number"`
}

type UpdateOrderReq struct {
	Place          string       `json:"place" binding:"required"`
	OrderTime      string       `json:"order_time" binding:"required,datetime=2006-01-02 15:04"`
	ClientCouponId *int         `json:"client_coupon_id"`
	Items          []UpdateItem `json:"items" binding:"required"`
}

type UpdateItem struct {
	ItemID   int `json:"item_id"`
	Quantity int `json:"quantity"`
}

type UpdateOrderStatusReq struct {
	Status string `json:"status" binding:"required"`
	Reason string `json:"reason"`
}

type CouponData struct {
	Created time.Time
	Life    int
	Tm      string
	Ru      string
	En      string
}

type DictionaryDTO struct {
	Tm string `json:"tm" binding:"required"`
	Ru string `json:"ru" binding:"required"`
	En string `json:"en" binding:"required"`
}
