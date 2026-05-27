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

func (r *repository) CheckOTP(ctx context.Context, dto user.CheckOTP) (*user.ResultsOTP, error) {
	var (
		users          user.ResultsOTP
		otp            int
		name, lastName *string
	)
	q := `
			SELECT id, otp, name, last_name
			FROM users
			WHERE phone_number = $1
		`
	err := r.client.QueryRow(ctx, q, dto.PhoneNumber).Scan(&users.UserId, &otp, &name, &lastName)

	if err != nil {
		return nil, appresult.ErrNotFoundTypeStr(dto.PhoneNumber)
	}

	if dto.Code != otp {
		return nil, appresult.ErrOTP
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
	_, err = r.client.Exec(ctx, q, users.UserId)
	if err != nil {
		return nil, appresult.ErrInternalServer
	}

	return &users, nil
}

func (r *repository) Login(ctx context.Context, dto user.LoginDTO) (*user.ResultsOTP, error) {
	var (
		users          user.ResultsOTP
		password       *string
		name, lastName *string
	)
	q := `
			SELECT id, password, name, last_name
			FROM users
			WHERE phone_number = $1
		`
	err := r.client.QueryRow(ctx, q, dto.PhoneNumber).Scan(&users.UserId, &password, &name, &lastName)

	if err != nil {
		return nil, appresult.ErrNotFoundTypeStr("phone number")
	}

	if password == nil || dto.Password == "" {
		return nil, appresult.ErrNotFoundTypeStr("password")
	}

	if bcrypt.CompareHashAndPassword([]byte(*password), []byte(dto.Password)) != nil {
		return nil, appresult.ErrWrong("password")
	}

	if name == nil && lastName == nil {
		users.IsFirst = true
	} else {
		users.IsFirst = false
	}

	return &users, nil
}

func (r *repository) CreateProfile(ctx context.Context, userId int, user user.UserReqDTO, imagePath, hashPassword string, baseURL string) (*user.Profile, error) {
	var (
		id int
	)
	q := `
			SELECT id
			FROM users
			WHERE id = $1
		`
	err := r.client.QueryRow(ctx, q, userId).Scan(&id)

	if err != nil {
		return nil, appresult.ErrNotFoundType(userId, "user")
	}

	q = `
		UPDATE users
		SET name = $1, last_name = $2, image_path = $3, location = $4, password = $5
		WHERE id = $6;
	`
	_, err = r.client.Exec(ctx, q, user.Name, user.LastName, imagePath, user.Location, hashPassword, id)
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
			SELECT id, name, last_name, phone_number, image_path, location
			FROM users
			WHERE id = $1
		`
	err := r.client.QueryRow(ctx, q, userId).Scan(
		&profile.Id,
		&profile.Name,
		&profile.LastName,
		&profile.PhoneNumber,
		&profile.ImagePath,
		&profile.Location,
	)

	if err != nil {
		return nil, appresult.ErrNotFoundType(userId, "user")
	}

	if profile.ImagePath != "" && baseURL != "" {
		cleanPath := strings.ReplaceAll(profile.ImagePath, "\\", "/")
		profile.ImagePath = fmt.Sprintf("%s/%s", baseURL, cleanPath)
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

func (r *repository) UpdateProfile(ctx context.Context, userId int, users user.UserUpdateDTO, imagePath, hashPassword string, baseURL string) (*user.Profile, error) {
	var (
		image string
	)
	q := `
			SELECT image_path
			FROM users
			WHERE id = $1
		`
	err := r.client.QueryRow(ctx, q, userId).Scan(&image)

	if err != nil {
		return nil, appresult.ErrNotFoundType(userId, "user")
	}

	if image != "" {
		imagges := []string{
			image,
		}
		utils.DropFiles(&imagges)
	}

	q = `
		UPDATE users
		SET name = $1, last_name = $2, image_path = $3, location = $4, password = $5
		WHERE id = $6;
	`
	_, err = r.client.Exec(ctx, q, users.Name, users.LastName, imagePath, users.Location, hashPassword, userId)
	if err != nil {
		fmt.Println("error: ", err)
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
