package clients

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
	Name       string         `json:"name"        binding:"required"`
	LastName   *string        `json:"last_name"`
	ProvinceId int            `json:"province_id" binding:"required,min=1"`
	District   *DictionaryDTO `json:"district"`
	Password   string         `json:"password" binding:"required,min=8,max=50"`
}

type Profile struct {
	Id            int            `json:"id"`
	Name          string         `json:"name"`
	LastName      *string        `json:"last_name"`
	PhoneNumber   string         `json:"phone_number"`
	ImagePath     *string        `json:"image_path"`
	Address       DictionaryDTO  `json:"address"`
	Organizations []Organization `json:"organizations"`
}

type Organization struct {
	Id             int    `json:"id"`
	BusinessesId   int    `json:"businesses_id"`
	BusinessesName string `json:"businesses_name"`
	Role           string `json:"role"`
}

type CountAndUsers struct {
	Count int       `json:"count"`
	Users []Profile `json:"Users"`
}

type User struct {
	Id       int    `json:"id"`
	FullName string `json:"fullName"`
}

type DictionaryDTO struct {
	Tm string `json:"tm"`
	En string `json:"en"`
	Ru string `json:"ru"`
}
