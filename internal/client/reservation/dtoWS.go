package reservation

import (
	"time"

	"github.com/gorilla/websocket"
)

type AvailabilityDay struct {
	Day       string `json:"day" binding:"required,datetime=2006-01-02"`
	Available bool   `json:"available"`
}

type AvailabilityDayReq struct {
	PersonCount int `json:"person_count"`
}

type AvailabilityTime struct {
	Time      string `json:"time" binding:"required,datetime=15:04"`
	Available bool   `json:"available"`
}

type AvailabilityTimeReq struct {
	PersonCount int       `json:"person_count"`
	Day         time.Time `json:"day"`
}

type Restaurants struct {
	Conn    *websocket.Conn
	Filter  *ReservationFilter
	BaseURL string
}

type ClientWS struct {
    Conn    *websocket.Conn
    Filter  *ReservationClientFilter
    BaseURL string
}


type AvailabilityDayClient struct {
	Conn        *websocket.Conn
	PersonCount int
}

type AvailabilityTimeClient struct {
	Conn        *websocket.Conn
	PersonCount int
	Day         time.Time
}
