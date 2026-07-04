package user

import "restaurants/internal/admin/province"

type UserFilter struct {
	Role   string `form:"role"`
	Search string `form:"search"`
	Limit  string `form:"limit"`
	Offset string `form:"offset"`
	Status string `form:"status"`
}

type RegisterDTO struct {
	PhoneNumber string `json:"phone_number" binding:"required"`
}

type OTP struct {
	Code int `json:"code"`
}

type CheckOTP struct {
	Code        int    `json:"code"`
	PhoneNumber string `json:"phone_number"`
}

type LoginDTO struct {
	Password    string `json:"password"`
	PhoneNumber string `json:"phone_number"`
}

type ResultsOTP struct {
	IsFirst bool   `json:"is_first"`
	Token   string `json:"token"`
}

type UserReqDTO struct {
	Name       string  `json:"name"        binding:"required"`
	LastName   *string `json:"last_name"`
	ProvinceId int     `json:"province_id" binding:"required,min=1"`
	District   *string `json:"district"`
	Password   string  `json:"password" binding:"required,min=8,max=50"`
}

type Profile struct {
	Id            int                  `json:"id"`
	Name          string               `json:"name"`
	LastName      *string              `json:"last_name"`
	PhoneNumber   string               `json:"phone_number"`
	ImagePath     *string              `json:"image_path"`
	Province      province.ProvinceDTO `json:"province"`
	District      *string              `json:"district"`
	Organizations []Organization       `json:"organizations"`
}

type Organization struct {
	Id              int     `json:"id"`
	BusinessesId    int     `json:"businesses_id"`
	BusinessesName  string  `json:"businesses_name"`
	BusinessesImage string  `json:"businesses_image"`
	Role            string  `json:"role"`
	Status          string  `json:"status"`
	Reason          *string `json:"reason"`
}
type User struct {
	Id       int    `json:"id"`
	FullName string `json:"fullName"`
}

type SearchUser struct {
	Id          int     `json:"id"`
	FullName    string  `json:"fullName"`
	PhoneNumber string  `json:"phone_number"`
	ImagePath   *string `json:"image_path"`
}

type SearchUserAll struct {
	Count int          `json:"count"`
	Users []SearchUser `json:"users"`
}

type Users struct {
	Id          int     `json:"id"`
	FullName    string  `json:"fullName"`
	PhoneNumber string  `json:"phone_number"`
	ImagePath   *string `json:"image_path"`
	Role        string  `json:"role"`
	Status      string  `json:"status"`
	Reason      *string `json:"reason"`
}

type GetAllUser struct {
	Count int     `json:"count"`
	Users []Users `json:"users"`
}

type UpdateBusinessesRoleStatusReq struct {
	Status string `json:"status" binding:"required"`
	Reason string `json:"reason"`
}

type DictionaryDTO struct {
	Tm string `json:"tm"`
	En string `json:"en"`
	Ru string `json:"ru"`
}
