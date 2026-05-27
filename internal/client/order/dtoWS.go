package order

import (
	"github.com/gorilla/websocket"
)

type OrderFilter struct {
	Limit  string `json:"limit"`
	Offset string `json:"offset"`
	Status string `json:"status"`
}

type OrderClientFilter struct {
	Limit  string `json:"limit"`
	Offset string `json:"offset"`
	Status string `json:"status"`
	Search string `json:"search"`
}

type OrderBusiness struct {
	Conn    *websocket.Conn
	Filter  *OrderFilter
	BaseURL string
}

type ClientWS struct {
	Conn    *websocket.Conn
	Filter  *OrderClientFilter
	BaseURL string
}

type OrderOneWS struct {
	Conn    *websocket.Conn
	BaseURL string
}

type ConnKey struct {
    ID   int
    Role string 
}
