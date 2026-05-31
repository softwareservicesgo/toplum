package db

import (
	"context"
	"errors"
	"fmt"
	"restaurants/internal/appresult"
	user "restaurants/internal/client/user"
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

	if password == nil || dto.Password == "" {
		return nil, nil, appresult.ErrNotFoundTypeStr("password")
	}

	if bcrypt.CompareHashAndPassword([]byte(*password), []byte(dto.Password)) != nil {
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
		districtId      *int
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

	if user.District != nil {
		queryDictionary := `INSERT INTO dictionary (tm, en, ru) VALUES ($1, $2, $3) RETURNING id`
		err = r.client.QueryRow(ctx, queryDictionary, user.District.Tm, user.District.En, user.District.Ru).Scan(&districtId)
		if err != nil {
			fmt.Println("error insert district dictionary:", err)
			return nil, appresult.ErrInternalServer
		}
	}

	q = `
		UPDATE users
		SET name = $1, last_name = $2, image_path = $3, district_dictionary_id = $4, province_id = $5, password = $6
		WHERE id = $7;
	`
	_, err = r.client.Exec(ctx, q, user.Name, user.LastName, imagePath, districtId, user.ProvinceId, hashPassword, id)
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
    CASE 
        WHEN d_district.tm IS NOT NULL 
        THEN (p_name.tm || ', ' || d_district.tm)
        ELSE p_name.tm
    END,
    CASE 
        WHEN d_district.en IS NOT NULL 
        THEN (p_name.en || ', ' || d_district.en)
        ELSE p_name.en
    END,
    CASE 
        WHEN d_district.ru IS NOT NULL 
        THEN (p_name.ru || ', ' || d_district.ru)
        ELSE p_name.ru
    END
		FROM users u
		JOIN provinces p        ON u.province_id = p.id
		JOIN dictionary p_name  ON p.name_dictionary_id = p_name.id
		LEFT JOIN dictionary d_district ON u.district_dictionary_id = d_district.id  -- LEFT JOIN!
		WHERE u.id = $1
		`
	err := r.client.QueryRow(ctx, q, userId).Scan(
		&profile.Id,
		&profile.Name,
		&profile.LastName,
		&profile.PhoneNumber,
		&profile.ImagePath,
		&profile.Address.Tm, &profile.Address.Ru, &profile.Address.En,
	)

	if err != nil {
		return nil, appresult.ErrNotFoundType(userId, "user")
	}

	if profile.ImagePath != nil && *profile.ImagePath != "" && baseURL != "" {
		cleanPath := strings.ReplaceAll(*profile.ImagePath, "\\", "/")
		newUrl := fmt.Sprintf("%s/%s", baseURL, cleanPath)
		profile.ImagePath = &newUrl
	}

	q = `
		SELECT ub.id, ub.businesses_id, b.name, ub.role
		FROM user_businesses ub
		JOIN businesses b ON ub.businesses_id = b.id
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
		)
		if err != nil {
			return nil, err
		}

		profile.Organizations = append(profile.Organizations, organization)
	}
	return &profile, nil
}

func (r *repository) UpdateProfile(ctx context.Context, userId int, users user.UserReqDTO, imagePath *string, hashPassword string, baseURL string) (*user.Profile, error) {
	var (
		image             *string
		districtId        *int
		oldDistrictDictId *int
	)

	q := `SELECT image_path, district_dictionary_id FROM users WHERE id = $1`
	err := r.client.QueryRow(ctx, q, userId).Scan(&image, &oldDistrictDictId)
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

	if users.District != nil {
		if oldDistrictDictId != nil {
			q = `UPDATE dictionary SET tm = $1, en = $2, ru = $3 WHERE id = $4`
			_, err = r.client.Exec(ctx, q, users.District.Tm, users.District.En, users.District.Ru, *oldDistrictDictId)
			if err != nil {
				fmt.Println("error update district dictionary:", err)
				return nil, appresult.ErrInternalServer
			}
			districtId = oldDistrictDictId
		} else {
			q = `INSERT INTO dictionary (tm, en, ru) VALUES ($1, $2, $3) RETURNING id`
			err = r.client.QueryRow(ctx, q, users.District.Tm, users.District.En, users.District.Ru).Scan(&districtId)
			if err != nil {
				fmt.Println("error insert district dictionary:", err)
				return nil, appresult.ErrInternalServer
			}
		}
	}

	q = `
		UPDATE users
		SET name = $1, last_name = $2, image_path = $3, district_dictionary_id = $4, province_id = $5, password = $6
		WHERE id = $7
	`
	_, err = r.client.Exec(ctx, q, users.Name, users.LastName, imagePath, districtId, users.ProvinceId, hashPassword, userId)
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
