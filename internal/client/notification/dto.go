package notification

import "restaurants/pkg/fcm"

type NotificationReqDTO struct {
	RestaurantId int           `json:"restaurant_id"`
	Title        DictionaryDTO `json:"title"`
	Content      DictionaryDTO `json:"content"`
	LifeDay      int           `json:"life_day"`
}

type Notifications struct {
	Count         int                   `json:"count"`
	Notifications []fcm.NotificationDTO `json:"notifications"`
}

type NotificationsClient struct {
	Count         int                   `json:"count"`
	UnreadCount   int                   `json:"unread_count"`
	Notifications []fcm.NotificationDTO `json:"notifications"`
}

type NotificationFilter struct {
	RestaurantId string `form:"restaurant_id"`
	Limit        string `form:"limit" json:"limit"`
	Offset       string `form:"offset" json:"offset"`
	Life         string `form:"life" json:"life"`
	Day          string `form:"day" json:"day" binding:"datetime=2006-01-02"`
}

type NotificationClientFilter struct {
	RestaurantId string `form:"restaurant_id"`
	Offset       string `form:"offset" json:"offset"`
	Limit        string `form:"limit" json:"limit"`
}

type DictionaryDTO struct {
	Tm string `json:"tm" binding:"required"`
	Ru string `json:"ru" binding:"required"`
	En string `json:"en" binding:"required"`
}
