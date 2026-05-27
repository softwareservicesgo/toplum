package offer

type CreateOfferReq struct {
	Content string `json:"content"`
}

type OfferDTO struct {
	Id       int    `json:"id"`
	UserName string `json:"user_name"`
	Content  string `json:"content"`
}

type GetAll struct {
	Count  int        `json:"count"`
	Offers []OfferDTO `json:"offers"`
}
