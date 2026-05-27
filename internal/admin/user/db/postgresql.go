package db

import (
	"context"
	"errors"
	"fmt"
	"restaurants/internal/admin/user"
	"restaurants/internal/appresult"
	"restaurants/pkg/client/postgresql"
	"restaurants/pkg/logging"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v4"
	"golang.org/x/crypto/bcrypt"
)

type repository struct {
	client postgresql.Client
	logger *logging.Logger
}

func NewRepository(client postgresql.Client, logger *logging.Logger) user.Repository {
	return &repository{
		client: client,
		logger: logger,
	}
}

func (r *repository) Create(ctx context.Context, dto user.UserReqDTO, hashPassword string) (*user.UserDTO, error) {
	tx, err := r.client.Begin(ctx)
	if err != nil {
		fmt.Println("error begin tx:", err)
		return nil, appresult.ErrInternalServer
	}
	defer tx.Rollback(ctx)

	var userId int

	q := `SELECT password FROM users`
	rows, err := tx.Query(ctx, q)
	if err != nil {
		fmt.Println("error select password:", err)
		return nil, appresult.ErrInternalServer
	}
	defer rows.Close()

	passwordBytes := []byte(dto.Password)
	for rows.Next() {
		var oldHash string
		if err := rows.Scan(&oldHash); err != nil {
			fmt.Println("error scan password:", err)
			return nil, appresult.ErrInternalServer
		}
		if bcrypt.CompareHashAndPassword([]byte(oldHash), passwordBytes) == nil {
			return nil, appresult.ErrAlreadyData("user password")
		}
	}

	var businessId int
	q = `SELECT id FROM businesses WHERE id = $1`
	err = tx.QueryRow(ctx, q, dto.BusinessId).Scan(&businessId)
	if err != nil {
		fmt.Println("error:", err)
		return nil, appresult.ErrNotFoundType(dto.BusinessId, "business")
	}

	q = `INSERT INTO users (name, password, role, businesses_id) VALUES ($1, $2, $3, $4) RETURNING id`
	err = tx.QueryRow(ctx, q, dto.Name, hashPassword, dto.Role, dto.BusinessId).Scan(&userId)
	if err != nil {
		fmt.Println("error insert user:", err)
		return nil, appresult.ErrInternalServer
	}

	if err := tx.Commit(ctx); err != nil {
		fmt.Println("error commit tx:", err)
		return nil, appresult.ErrInternalServer
	}

	return r.GetOne(ctx, userId)
}

func (r *repository) GetOne(ctx context.Context, userID int) (*user.UserDTO, error) {
	var res user.UserDTO

	q := `SELECT id, name, role, businesses_id FROM users WHERE id = $1`
	err := r.client.QueryRow(ctx, q, userID).Scan(
		&res.Id, &res.Name, &res.Role, &res.BusinessId,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			fmt.Println("error: ", err)
			return nil, appresult.ErrNotFoundType(userID, "user")
		}
		fmt.Println("error: ", err)
		return nil, appresult.ErrInternalServer
	}

	return &res, nil
}

func (r *repository) GetAll(ctx context.Context, values map[string]string) (*[]user.UserDTO, *int, error) {
	var (
		users    []user.UserDTO
		conds    []string
		args     []interface{}
		argIndex = 1
		count    int
	)

	if search, ok := values["name"]; ok && search != "" {
		searchLike := "%" + search + "%"
		conds = append(conds, fmt.Sprintf("name ILIKE $%d", argIndex))
		args = append(args, searchLike)
		argIndex++
	}

	if role, ok := values["role"]; ok && role != "" {
		conds = append(conds, fmt.Sprintf("role = $%d", argIndex))
		args = append(args, role)
		argIndex++
	}

	if businessID, ok := values["businesses_id"]; ok && businessID != "" {
		conds = append(conds, fmt.Sprintf("businesses_id = $%d", argIndex))
		args = append(args, businessID)
		argIndex++
	}

	where := ""
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}

	sort := "created_at"
	sortType := "DESC"
	if st, ok := values["type_sort"]; ok && (strings.ToUpper(st) == "ASC" || strings.ToUpper(st) == "DESC") {
		sortType = strings.ToUpper(st)
	}

	limit := 10
	if l, ok := values["size"]; ok && l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = v
		}
	}

	page := 1
	if p, ok := values["page"]; ok && p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			page = v
		}
	}

	offset := (page - 1) * limit

	q := fmt.Sprintf(`
		SELECT id, name, role, businesses_id
		FROM users
		%s
		ORDER BY %s %s
		LIMIT %d OFFSET %d
	`, where, sort, sortType, limit, offset)

	rows, err := r.client.Query(ctx, q, args...)
	if err != nil {
		fmt.Println("error: ", err)
		return nil, nil, appresult.ErrInternalServer
	}
	defer rows.Close()

	for rows.Next() {
		var res user.UserDTO
		if err := rows.Scan(&res.Id, &res.Name, &res.Role, &res.BusinessId); err != nil {
			fmt.Println("error: ", err)
			return nil, nil, appresult.ErrInternalServer
		}
		users = append(users, res)
	}

	countQuery := fmt.Sprintf(`SELECT count(*) FROM users %s`, where)
	if err := r.client.QueryRow(ctx, countQuery, args...).Scan(&count); err != nil {
		fmt.Println("error:", err)
		return nil, nil, appresult.ErrInternalServer
	}

	return &users, &count, nil
}

func (r *repository) Update(ctx context.Context, userId int, dto user.UserUpdateDTO, hashPassword string) (*user.UserDTO, error) {
	var (
		currentName     string
		currentPassword string
		businessId      int
	)

	tx, err := r.client.Begin(ctx)
	if err != nil {
		return nil, appresult.ErrInternalServer
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)
			panic(p)
		}
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	q := `SELECT name, password, businesses_id FROM users WHERE id = $1`
	err = r.client.QueryRow(ctx, q, userId).Scan(&currentName, &currentPassword, &businessId)
	if err != nil {
		fmt.Println("error: ", err)
		return nil, appresult.ErrNotFoundType(userId, "user")
	}

	if dto.Name != "" {
		currentName = dto.Name
	}

	if dto.Password != "" {
		q = `SELECT id, password FROM users`
		rows, err := tx.Query(ctx, q)
		if err != nil {
			fmt.Println("error: ", err)
			return nil, appresult.ErrInternalServer
		}
		defer rows.Close()

		for rows.Next() {
			var otherUserID int
			var oldHash string
			if err := rows.Scan(&otherUserID, &oldHash); err != nil {
				return nil, appresult.ErrInternalServer
			}

			if otherUserID == userId {
				continue
			}

			if bcrypt.CompareHashAndPassword([]byte(oldHash), []byte(dto.Password)) == nil {
				return nil, appresult.ErrPasswordAgainByName
			}
		}
	}

	q = `UPDATE users SET name = $1, password = $2 WHERE id = $3`
	newPasswordToSave := currentPassword
	if dto.Password != "" {
		newPasswordToSave = hashPassword
	}

	if _, err = tx.Exec(ctx, q, currentName, newPasswordToSave, userId); err != nil {
		fmt.Println("error: ", err)
		return nil, appresult.ErrInternalServer
	}

	if err = tx.Commit(ctx); err != nil {
		fmt.Println("error: ", err)
		return nil, appresult.ErrInternalServer
	}

	return r.GetOne(ctx, userId)
}

func (r *repository) Delete(ctx context.Context, userId int, role string) error {
	var res user.UserDTO

	tx, err := r.client.Begin(ctx)
	if err != nil {
		fmt.Println("error: ", err)
		return appresult.ErrInternalServer
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)
			panic(p)
		}
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	q := `SELECT id, name, role, businesses_id FROM users WHERE id = $1`
	err = r.client.QueryRow(ctx, q, userId).Scan(
		&res.Id, &res.Name, &res.Role, &res.BusinessId,
	)
	if err != nil {
		fmt.Println("error:: ", err)
		return appresult.ErrNotFoundType(userId, "user")
	}

	q = `DELETE FROM users WHERE id = $1`
	if _, err = tx.Exec(ctx, q, userId); err != nil {
		fmt.Println("error:", err)
		return appresult.ErrInternalServer
	}

	if err = tx.Commit(ctx); err != nil {
		fmt.Println("error: ", err)
		return appresult.ErrInternalServer
	}

	return nil
}

func (r *repository) GetAllByBusiness(ctx context.Context, businessId int, limit int, offset int) (*user.UsersForBusiness, error) {
	var users user.UsersForBusiness
	var id int

	q := `SELECT id FROM businesses WHERE id = $1`
	err := r.client.QueryRow(ctx, q, businessId).Scan(&id)
	if err != nil {
		return nil, appresult.ErrNotFoundType(businessId, "business")
	}

	qManager := `SELECT id, name, role FROM users WHERE businesses_id = $1 AND role = 'MANAGER' LIMIT 1`
	err = r.client.QueryRow(ctx, qManager, businessId).Scan(&users.Manager.Id, &users.Manager.Name, &users.Manager.Role)
	if err != nil && err != pgx.ErrNoRows {
		fmt.Println("error manager:", err)
		return nil, appresult.ErrInternalServer
	}

	qCount := `SELECT COUNT(*) FROM users WHERE businesses_id = $1 AND role = 'EMPLOYEE'`
	err = r.client.QueryRow(ctx, qCount, businessId).Scan(&users.EmployeesCount)
	if err != nil {
		fmt.Println("error count employees:", err)
		return nil, appresult.ErrInternalServer
	}

	offset = (offset - 1) * limit
	qEmployees := `SELECT id, name, role FROM users WHERE businesses_id = $1 AND role = 'EMPLOYEE' ORDER BY created_at LIMIT $2 OFFSET $3`
	rows, err := r.client.Query(ctx, qEmployees, businessId, limit, offset)
	if err != nil {
		fmt.Println("error employees:", err)
		return nil, appresult.ErrInternalServer
	}
	defer rows.Close()

	for rows.Next() {
		var emp user.UserForBusiness
		if err := rows.Scan(&emp.Id, &emp.Name, &emp.Role); err != nil {
			fmt.Println("error scan employee:", err)
			return nil, appresult.ErrInternalServer
		}
		users.Employees = append(users.Employees, emp)
	}

	return &users, nil
}
