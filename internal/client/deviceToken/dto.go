package deviceToken

type DeviceToken struct {
	ClientId *int   `json:"client_id"`
	Token    string `json:"token" binding:"required"`
}

type DeviceTokens struct {
	Id       int    `json:"id"`
	ClientId *int   `json:"client_id"`
	Token    string `json:"token" binding:"required"`
}
