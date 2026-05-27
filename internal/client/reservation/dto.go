package reservation

type ReservationById struct {
	Id              int           `json:"id"`
	Client          Client        `json:"client"`
	ReservationDate string        `json:"reservation_date" binding:"required,datetime=2006-01-02 15:04"`
	ReservationTime int           `json:"reservation_time"`
	PersonCount     int           `json:"person_count"`
	WishContent     *string       `json:"wish_content"`
	ClientCoupon    *ClientCoupon `json:"client_coupon"`
	GeneralBill     *float64      `json:"general_bill"`
	InitialBill     *float64      `json:"initial_bill"`
	Foods           *[]Food       `json:"foods"`
	Table           Table         `json:"table"`
	Status          string        `json:"status"`
	Reason          *string       `json:"reason"`
}

type Food struct {
	Id        int           `json:"id"`
	FoodId    int           `json:"food_id"`
	FoodName  DictionaryDTO `json:"food_name"`
	FoodImage string        `json:"food_image"`
	Value     float32       `json:"value"`
	Count     int           `json:"count"`
}

type Table struct {
	TableId int `json:"table_id"`
	Seats   int `json:"seats"`
}

type Client struct {
	Id          int    `json:"id"`
	FullName    string `json:"full_name"`
	ImagePath   string `json:"image_path"`
	PhoneNumber string `json:"phone_number"`
}

type ReservationReqDTO struct {
	PersonCount     int       `json:"person_count" binding:"required"`
	ReservationDate string    `json:"reservation_date" binding:"required,datetime=2006-01-02 15:04"`
	WishContent     string    `json:"wish_content"`
	ClientCouponId  *int      `json:"client_coupon_id"`
	Foods           []FoodReq `json:"foods"`
	TableId         int       `json:"table_id" binding:"required"`
}

type FoodReq struct {
	FoodId int `json:"food_id"`
	Count  int `json:"count"`
}

type Reservation struct {
	Count        int               `json:"count"`
	Reservations []ReservationById `json:"reservations"`
}

type ReservationClients struct {
	Restaurant   Restaurant          `json:"restaurant"`
	Reservations []ReservationClient `json:"reservations"`
}

type Reservations struct {
	Count                  int                  `json:"count"`
	RestaurantReservations []ReservationClients `json:"restaurant_reservations"`
}

type ReservationPatchDTO struct {
	PersonCount     int        `json:"person_count" binding:"required"`
	ReservationDate string     `json:"reservation_date" binding:"required"`
	WishContent     *string    `json:"wish_content"`
	ClientCouponId  *int       `json:"client_coupon_id"`
	Foods           *[]FoodReq `json:"foods"`
	TableId         int        `json:"table_id" binding:"required"`
}

type ClientCoupon struct {
	Id         int           `json:"id"`
	CouponName DictionaryDTO `json:"coupon_name"`
}

type UpdateStatusByRestaurant struct {
	Status string `json:"status" binding:"required,oneof=approved confirmed completed cancelled_by_restaurant"`
	Reason string `json:"reason"`
}

type Reason struct {
	Reason string `json:"reason" binding:"required"`
}

type ReservationFilter struct {
	Type   string `json:"type"`
	Limit  string `form:"limit" json:"limit"`
	Offset string `form:"offset" json:"offset"`
	Search string `form:"search" json:"search"`
	Status string `form:"status" json:"status"`
	Day    string `form:"day" json:"day" binding:"datetime=2006-01-02"`
}

type ReservationClientFilter struct {
	Limit  string `form:"limit" json:"limit"`
	Offset string `form:"offset" json:"offset"`
}

type Restaurant struct {
	Id               int           `json:"id"`
	Images           string        `json:"images"`
	Name             DictionaryDTO `json:"name"`
	Rating           float32       `json:"rating"`
	Price            int           `json:"price"`
	Address          DictionaryDTO `json:"address"`
	CountReservation int           `json:"count_reservation"`
}

type ReservationClient struct {
	Id              int      `json:"id"`
	ReservationDate string   `json:"reservation_date" binding:"required,datetime=2006-01-02 15:04"`
	ReservationTime int      `json:"reservation_time"`
	PersonCount     int      `json:"person_count"`
	GeneralBill     *float64 `json:"general_bill"`
	Status          string   `json:"status"`
	Reason          *string  `json:"reason"`
}

type DictionaryDTO struct {
	Tm string `json:"tm" binding:"required"`
	Ru string `json:"ru" binding:"required"`
	En string `json:"en" binding:"required"`
}
