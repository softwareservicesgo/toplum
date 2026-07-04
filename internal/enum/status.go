package enum

const (
	APPROVED string = "APPROVED"
	PENDING  string = "PENDING"
	CANCELED string = "CANCELED"
	EXPIRED  string = "EXPIRED"

	COMPLETED_BY_CLIENT     string = "COMPLETED_BY_CLIENT"
	CANCELED_BY_CLIENT      string = "CANCELED_BY_CLIENT"
	COMPLETED_BY_BUSINESSES string = "COMPLETED_BY_BUSINESSES"
	CANCELED_BY_BUSINESSES  string = "CANCELED_BY_BUSINESSES"
)

var StatusBusinesses = map[string]struct{}{
	APPROVED: {},
	PENDING:  {},
	CANCELED: {},
	EXPIRED:  {},
}

func IsValidStatusBusinesses(status string) bool {
	_, ok := StatusBusinesses[status]
	return ok
}

var StatusOrder = map[string]struct{}{
	APPROVED:                {},
	PENDING:                 {},
	COMPLETED_BY_CLIENT:     {},
	CANCELED_BY_CLIENT:      {},
	COMPLETED_BY_BUSINESSES: {},
	CANCELED_BY_BUSINESSES:  {},
}

func IsValidStatusOrder(status string) bool {
	_, ok := StatusOrder[status]
	return ok
}

var StatusBusinessesRole = map[string]struct{}{
	APPROVED: {},
	PENDING:  {},
	CANCELED: {},
}

func IsValidStatusBusinessesRole(status string) bool {
	_, ok := StatusBusinessesRole[status]
	return ok
}
