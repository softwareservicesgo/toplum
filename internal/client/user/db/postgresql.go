package db

import (
	"context"
	"errors"
	"fmt"
	"restaurants/internal/appresult"
	user "restaurants/internal/client/user"
	"restaurants/internal/enum"
	"restaurants/pkg/client/postgresql"
	"restaurants/pkg/logging"
	"restaurants/pkg/utils"
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

func (r *repository) Register(ctx context.Context, dto user.RegisterDTO) (int, error) {
	var (
		id int
	)
	q := `
			SELECT id
			FROM users
			WHERE phone_number = $1
		`
	err := r.client.QueryRow(ctx, q, dto.PhoneNumber).Scan(&id)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return 0, err
	}

	randomNumber := utils.UniqueNumberGenerator(10000, 99999)
	fmt.Println("RANDOM: ", randomNumber)

	if id == 0 {
		q = `
		INSERT INTO users (phone_number, otp)
				VALUES ($1, $2)`

		_, err = r.client.Exec(ctx, q, dto.PhoneNumber, randomNumber)
		if err != nil {
			return 0, err
		}

		return randomNumber, nil
	}

	q = `update users set otp = $1 where id = $2`

	_, err = r.client.Exec(ctx, q, randomNumber, id)
	if err != nil {
		return 0, err
	}

	return randomNumber, nil
}

func (r *repository) CheckOTP(ctx context.Context, dto user.CheckOTP) (*user.ResultsOTP, *int, error) {
	var (
		users          user.ResultsOTP
		userId         int
		otp            int
		name, lastName *string
	)
	q := `
			SELECT id, otp, name, last_name
			FROM users
			WHERE phone_number = $1
		`
	err := r.client.QueryRow(ctx, q, dto.PhoneNumber).Scan(&userId, &otp, &name, &lastName)

	if err != nil {
		return nil, nil, appresult.ErrNotFoundTypeStr(dto.PhoneNumber)
	}

	if dto.Code != otp {
		return nil, nil, appresult.ErrOTP
	}

	if name == nil && lastName == nil {
		users.IsFirst = true
	} else {
		users.IsFirst = false
	}

	q = `
		UPDATE users
		SET otp = 0
		WHERE id = $1;
	`
	_, err = r.client.Exec(ctx, q, userId)
	if err != nil {
		return nil, nil, appresult.ErrInternalServer
	}

	return &users, &userId, nil
}

func (r *repository) Login(ctx context.Context, dto user.LoginDTO) (*user.ResultsOTP, *int, error) {
	var (
		users          user.ResultsOTP
		userId         int
		password       *string
		name, lastName *string
	)
	q := `
			SELECT id, password, name, last_name
			FROM users
			WHERE phone_number = $1
		`
	err := r.client.QueryRow(ctx, q, dto.PhoneNumber).Scan(&userId, &password, &name, &lastName)

	if err != nil {
		return nil, nil, appresult.ErrNotFoundTypeStr("phone number")
	}

	if dto.Password == "" {
		return nil, nil, appresult.ErrNotFoundTypeStr("password")
	}

	if password == nil || bcrypt.CompareHashAndPassword([]byte(*password), []byte(dto.Password)) != nil {
		return nil, nil, appresult.ErrWrong("password")
	}

	if name == nil && lastName == nil {
		users.IsFirst = true
	} else {
		users.IsFirst = false
	}

	return &users, &userId, nil
}

func (r *repository) CreateProfile(ctx context.Context, userId int, user user.UserReqDTO, imagePath *string, hashPassword string, baseURL string) (*user.Profile, error) {
	var (
		id, province_id int
	)
	q := `SELECT id FROM users WHERE id = $1`
	err := r.client.QueryRow(ctx, q, userId).Scan(&id)
	if err != nil {
		return nil, appresult.ErrNotFoundType(userId, "user")
	}

	q = `SELECT id FROM provinces WHERE id = $1`
	err = r.client.QueryRow(ctx, q, user.ProvinceId).Scan(&province_id)
	if err != nil {
		return nil, appresult.ErrNotFoundType(province_id, "province")
	}

	q = `
		UPDATE users
		SET name = $1, last_name = $2, image_path = $3, district = $4, province_id = $5, password = $6
		WHERE id = $7;
	`
	_, err = r.client.Exec(ctx, q, user.Name, user.LastName, imagePath, user.District, user.ProvinceId, hashPassword, id)
	if err != nil {
		return nil, appresult.ErrInternalServer
	}

	profile, err := r.GetProfile(ctx, userId, baseURL)
	if err != nil {
		return nil, err
	}

	return profile, nil
}

func (r *repository) GetProfile(ctx context.Context, userId int, baseURL string) (*user.Profile, error) {
	var (
		profile user.Profile
	)
	q := `
			SELECT 
    u.id, 
    u.name, 
    u.last_name, 
    u.phone_number, 
    u.image_path,
	u.district,
	p.id, p_name.tm, p_name.en, p_name.ru
		FROM users u
		JOIN provinces p        ON u.province_id = p.id
		JOIN dictionary p_name  ON p.name_dictionary_id = p_name.id
		WHERE u.id = $1
		`
	err := r.client.QueryRow(ctx, q, userId).Scan(
		&profile.Id,
		&profile.Name,
		&profile.LastName,
		&profile.PhoneNumber,
		&profile.ImagePath,
		&profile.District,
		&profile.Province.Id, &profile.Province.Name.Tm, &profile.Province.Name.En, &profile.Province.Name.Ru,
	)

	if err != nil {
		fmt.Println("error:", err)
		return nil, appresult.ErrNotFoundType(userId, "user")
	}

	if profile.ImagePath != nil && *profile.ImagePath != "" && baseURL != "" {
		cleanPath := strings.ReplaceAll(*profile.ImagePath, "\\", "/")
		newUrl := fmt.Sprintf("%s/%s", baseURL, cleanPath)
		profile.ImagePath = &newUrl
	}

	q = `
		SELECT ub.id, ub.businesses_id, b.name, ub.role, img.image_path, ub.status, ub.reason
		FROM user_businesses ub
		JOIN businesses b ON ub.businesses_id = b.id
		JOIN image_businesses img ON img.businesses_id = b.id AND img.is_main = true
		WHERE ub.user_id = $1
		ORDER BY ub.created_at DESC
	`

	rows, err := r.client.Query(ctx, q, userId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var organization user.Organization
		err := rows.Scan(
			&organization.Id,
			&organization.BusinessesId,
			&organization.BusinessesName,
			&organization.Role,
			&organization.BusinessesImage,
			&organization.Status,
			&organization.Reason,
		)
		if err != nil {
			return nil, err
		}
		if organization.BusinessesImage != "" && baseURL != "" {
			cleanPath := strings.ReplaceAll(organization.BusinessesImage, "\\", "/")
			organization.BusinessesImage = fmt.Sprintf("%s/%s", baseURL, cleanPath)
		}

		profile.Organizations = append(profile.Organizations, organization)
	}
	return &profile, nil
}

func (r *repository) UpdateProfile(ctx context.Context, userId int, users user.UserReqDTO, imagePath *string, hashPassword string, baseURL string) (*user.Profile, error) {
	var (
		image *string
	)

	q := `SELECT image_path FROM users WHERE id = $1`
	err := r.client.QueryRow(ctx, q, userId).Scan(&image)
	if err != nil {
		return nil, appresult.ErrNotFoundType(userId, "user")
	}

	if image != nil && *image != "" {
		images := []string{*image}
		utils.DropFiles(&images)
	}

	var provinceId int
	q = `SELECT id FROM provinces WHERE id = $1`
	err = r.client.QueryRow(ctx, q, users.ProvinceId).Scan(&provinceId)
	if err != nil {
		return nil, appresult.ErrNotFoundType(users.ProvinceId, "province")
	}

	q = `
		UPDATE users
		SET name = $1, last_name = $2, image_path = $3, district = $4, province_id = $5, password = $6
		WHERE id = $7
	`
	_, err = r.client.Exec(ctx, q, users.Name, users.LastName, imagePath, users.District, users.ProvinceId, hashPassword, userId)
	if err != nil {
		fmt.Println("error update user:", err)
		return nil, appresult.ErrInternalServer
	}

	profile, err := r.GetProfile(ctx, userId, baseURL)
	if err != nil {
		return nil, err
	}

	return profile, nil
}

func (r *repository) Logout(ctx context.Context, token string) error {
	q := `
		INSERT INTO blacklist (token)
				VALUES ($1)`

	_, err := r.client.Exec(ctx, q, token)
	if err != nil {
		return err
	}

	return nil
}

func (r *repository) SearchUsers(ctx context.Context, name, phoneNumber, limitStr, offsetStr, baseURL string) (*user.SearchUserAll, error) {
	var result user.SearchUserAll

	limit, offset, err := utils.ParsePagination(limitStr, offsetStr)
	if err != nil {
		fmt.Println("error: ", err)
		return nil, appresult.ErrInternalServer
	}

	where := "WHERE 1=1"
	args := []interface{}{}
	argIdx := 1

	name = strings.TrimSpace(name)
	if name != "" {
		parts := strings.Fields(name)

		for _, part := range parts {
			where += fmt.Sprintf(
				" AND (u.name ILIKE $%d OR COALESCE(u.last_name, '') ILIKE $%d)",
				argIdx, argIdx+1,
			)

			like := "%" + part + "%"
			args = append(args, like, like)
			argIdx += 2
		}
	}

	phoneNumber = strings.TrimSpace(phoneNumber)
	if phoneNumber != "" {
		where += fmt.Sprintf(" AND u.phone_number ILIKE $%d", argIdx)
		args = append(args, "%"+phoneNumber+"%")
		argIdx++
	}

	countQ := `SELECT COUNT(*) FROM users u ` + where
	if err := r.client.QueryRow(ctx, countQ, args...).Scan(&result.Count); err != nil {
		fmt.Println("error: ", err)
		return nil, appresult.ErrInternalServer
	}

	if result.Count == 0 {
		result.Users = []user.SearchUser{}
		return &result, nil
	}

	dataQ := fmt.Sprintf(`
		SELECT 
			u.id,
			TRIM(u.name || ' ' || COALESCE(u.last_name, '')) AS full_name,
			u.phone_number,
			u.image_path
		FROM users u
		%s
		ORDER BY u.id DESC
		LIMIT $%d OFFSET $%d
	`, where, argIdx, argIdx+1)

	dataArgs := append(args, limit, offset)

	rows, err := r.client.Query(ctx, dataQ, dataArgs...)
	if err != nil {
		fmt.Println("error: ", err)
		return nil, appresult.ErrInternalServer
	}
	defer rows.Close()

	for rows.Next() {
		var u user.SearchUser
		if err := rows.Scan(&u.Id, &u.FullName, &u.PhoneNumber, &u.ImagePath); err != nil {
			fmt.Println("error: ", err)
			return nil, appresult.ErrInternalServer
		}

		if u.ImagePath != nil && *u.ImagePath != "" && baseURL != "" {
			cleanPath := strings.ReplaceAll(*u.ImagePath, "\\", "/")
			newUrl := fmt.Sprintf("%s/%s", baseURL, cleanPath)
			u.ImagePath = &newUrl
		}

		result.Users = append(result.Users, u)
	}
	if err := rows.Err(); err != nil {
		fmt.Println("error: ", err)
		return nil, appresult.ErrInternalServer
	}

	return &result, nil
}

func (r *repository) UpdateStatusBusinessesRole(ctx context.Context, userId, userBusinessesId int, request user.UpdateBusinessesRoleStatusReq) error {

	var currentStatus string

	query := `
				SELECT status
			FROM user_businesses 
			WHERE id = $1 AND user_id = $2
		`

	err := r.client.QueryRow(ctx, query, userBusinessesId, userId).Scan(&currentStatus)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			content := fmt.Sprintf("user_businesses with user = %d, user_businesses", userId)
			return appresult.ErrNotFoundType(userBusinessesId, content)
		}
		fmt.Println("error: ", err)
		return appresult.ErrInternalServer
	}

	if currentStatus != enum.PENDING {
		return appresult.ErrStatus
	}

	if request.Reason != "" && request.Status == enum.CANCELED && len(request.Reason) > 150 {
		return appresult.ErrOverLimit(150, "reason")
	}

	q := `
		UPDATE user_businesses
			SET status=$1, reason=$2
			WHERE id=$3
		`

	_, err = r.client.Exec(ctx, q, request.Status, request.Reason, userBusinessesId)

	if err != nil {
		fmt.Println("error: ", err)
		return appresult.ErrInternalServer
	}

	return nil
}

func (r *repository) GetAllUsers(ctx context.Context, businessesId int, filter user.UserFilter, baseURL string) (*user.GetAllUser, error) {
	var result user.GetAllUser

	limit, offset, err := utils.ParsePagination(filter.Limit, filter.Offset)
	if err != nil {
		fmt.Println("error: ", err)
		return nil, appresult.ErrInternalServer
	}

	where := "WHERE ub.businesses_id = $1"
	args := []interface{}{businessesId}
	argIdx := 2

	search := strings.TrimSpace(filter.Search)
	if search != "" {
		like := "%" + search + "%"
		where += fmt.Sprintf(
			` AND (
			u.name ILIKE $%d
			OR COALESCE(u.last_name, '') ILIKE $%d
			OR u.phone_number ILIKE $%d
		)`,
			argIdx, argIdx+1, argIdx+2,
		)
		args = append(args, like, like, like)
		argIdx += 3
	}

	role := strings.TrimSpace(filter.Role)
	if role != "" {
		where += fmt.Sprintf(" AND ub.role = $%d", argIdx)
		args = append(args, role)
		argIdx++
	}

	status := strings.TrimSpace(filter.Status)
	if status != "" {
		where += fmt.Sprintf(" AND ub.status = $%d", argIdx)
		args = append(args, status)
		argIdx++
	}

	countQ := `
		SELECT COUNT(*) 
		FROM users u
		JOIN user_businesses ub ON ub.user_id = u.id
		` + where

	if err := r.client.QueryRow(ctx, countQ, args...).Scan(&result.Count); err != nil {
		fmt.Println("error: ", err)
		return nil, appresult.ErrInternalServer
	}

	if result.Count == 0 {
		result.Users = []user.Users{}
		return &result, nil
	}

	dataQ := fmt.Sprintf(`
		SELECT 
			u.id,
			TRIM(u.name || ' ' || COALESCE(u.last_name, '')) AS full_name,
			u.phone_number,
			u.image_path,
			ub.role,
			ub.status,
			ub.reason
		FROM users u
		JOIN user_businesses ub ON ub.user_id = u.id
		%s
		ORDER BY u.id DESC
		LIMIT $%d OFFSET $%d
	`, where, argIdx, argIdx+1)

	dataArgs := append(args, limit, offset)

	rows, err := r.client.Query(ctx, dataQ, dataArgs...)
	if err != nil {
		fmt.Println("error: ", err)
		return nil, appresult.ErrInternalServer
	}
	defer rows.Close()

	for rows.Next() {
		var u user.Users
		if err := rows.Scan(
			&u.Id,
			&u.FullName,
			&u.PhoneNumber,
			&u.ImagePath,
			&u.Role,
			&u.Status,
			&u.Reason,
		); err != nil {
			fmt.Println("error: ", err)
			return nil, appresult.ErrInternalServer
		}

		if u.ImagePath != nil && *u.ImagePath != "" && baseURL != "" {
			cleanPath := strings.ReplaceAll(*u.ImagePath, "\\", "/")
			newUrl := fmt.Sprintf("%s/%s", baseURL, cleanPath)
			u.ImagePath = &newUrl
		}

		result.Users = append(result.Users, u)
	}
	if err := rows.Err(); err != nil {
		fmt.Println("error: ", err)
		return nil, appresult.ErrInternalServer
	}

	return &result, nil
}
