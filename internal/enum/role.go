package enum

const (
	RoleAdmin    string = "ADMIN"
	RoleManager  string = "MANAGER"
	RoleEmployee string = "EMPLOYEE"
	RoleOperator string = "OPERATOR"
)

var Roles = map[string]struct{}{
	RoleAdmin:    {},
	RoleManager:  {},
	RoleEmployee: {},
	RoleOperator: {},
}

func IsValidRole(role string) bool {
	_, ok := Roles[role]
	return ok
}
