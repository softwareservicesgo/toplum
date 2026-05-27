package user

var Fields = []string{
	"name",
	"role",
	"businesses_id",
	"sort",
	"type_sort",
	"size",
	"page",
}

type UserDTO struct {
	Id         int    `json:"id"`
	Name       string `json:"name"`
	Role       string `json:"role"`
	BusinessId *int   `json:"businesses_id"`
}

type UserReqDTO struct {
	Name       string `json:"name" binding:"required"`
	Password   string `json:"password" binding:"required"`
	Role       string `json:"role" binding:"required"`
	BusinessId int    `json:"businesses_id" binding:"required"`
}

type AllAndSum struct {
	Businesses *[]UserDTO `json:"users"`
	Count      int        `json:"count"`
}

type UserUpdateDTO struct {
	Name     string `json:"name"`
	Password string `json:"password"`
}

type UserForBusiness struct {
	Id   int    `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"`
}

type UsersForBusiness struct {
	Manager        UserForBusiness   `json:"manager"`
	EmployeesCount int               `json:"employees_count"`
	Employees      []UserForBusiness `json:"employees"`
}
