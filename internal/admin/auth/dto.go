package auth

type LoginDTO struct {
	Name     string `json:"name"`
	Password string `json:"password"`
}

type ResLoginDTO struct {
	Id           int    `json:"id"`
	Password     string `json:"password"`
	Role         string `json:"role"`
	Name         string `json:"name"`
	BusinessesId int    `json:"businesses_id"`
}

type ReqLoginDTO struct {
	Token        string `json:"token"`
	Id           int    `json:"id"`
	Name         string `json:"name"`
	Role         string `json:"role"`
	BusinessesId int    `json:"businesses_id"`
}
