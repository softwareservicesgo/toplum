package utils

import (
	"errors"
	"fmt"
	"mime/multipart"
	"os"
	"path/filepath"
	"restaurants/internal/appresult"
	"restaurants/internal/config"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt"
	"github.com/jackc/pgx/v4/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

func DoWithTries(fn func() error, attemtps int, delay time.Duration) (err error) {
	for attemtps > 0 {
		if err = fn(); err != nil {
			time.Sleep(delay)
			attemtps--
			continue
		}
		return nil
	}
	return
}

func RemoveFile(path string) error {
	err := os.RemoveAll(path)
	if err != nil {
		return err
	}
	return nil
}

func DropFiles(images *[]string) {
	if images == nil {
		return
	}
	for _, imgPath := range *images {
		if err := os.Remove(imgPath); err != nil {
			fmt.Println("Failed to remove file:", imgPath, err)
		}
	}
}

func GenerateTokenPair(id int) (string, error) {
	cfg := config.GetConfig("")
	token := jwt.New(jwt.SigningMethodHS256)

	claims := token.Claims.(jwt.MapClaims)
	claims["id"] = id
	claims["time_now"] = time.Now()

	t, err := token.SignedString([]byte(cfg.JwtKey))
	if err != nil {
		return "", err
	}

	return t, nil
}

func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func ExtractUserIdFromToken(c *gin.Context, db *pgxpool.Pool) (int, error) {

	tokenString := c.GetHeader("Authorization")
	if tokenString == "" {
		return -1, appresult.ErrMissingToken
	}

	if len(tokenString) < 7 || tokenString[:7] != "Bearer " {
		return 0, appresult.ErrBadToken
	}

	tokenString = tokenString[7:]
	token, err := ParseToken(tokenString)
	if err != nil {
		fmt.Println("error: ", err)
		return 0, errors.New("invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		fmt.Println("error: ", err)
		return 0, errors.New("unable to parse claims")
	}

	userID, ok := claims["id"].(float64)
	if !ok {
		fmt.Println("error: ", err)
		return 0, errors.New("missing 'id' claim in token")
	}

	return int(userID), nil
}

func ParseToken(jwtToken string) (*jwt.Token, error) {
	cfg := config.GetConfig("")

	token, err := jwt.Parse(jwtToken, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(cfg.JwtKey), nil
	})

	if err != nil {
		return nil, errors.New("bad token")
	}

	return token, nil
}

func SaveUploadedFile(c *gin.Context, file *multipart.FileHeader, uploadDir string) (*string, error) {
	cleanName := strings.ReplaceAll(filepath.Base(file.Filename), " ", "")
	fileName := fmt.Sprintf("%d_%s", time.Now().UnixNano(), cleanName)
	filePath := filepath.Join(uploadDir, fileName)

	if err := c.SaveUploadedFile(file, filePath); err != nil {
		return nil, err
	}
	return &filePath, nil
}

func UniqueNumberGenerator(min, max int) int {
	mod := max - min + 1
	step := 17777
	current := int(time.Now().UnixNano() % int64(mod))

	num := current + min
	current = (current + step) % mod
	return num
}

func ParsePagination(limitStr, offsetStr string) (*int, *int, error) {
	offsetInt, err := strconv.Atoi(offsetStr)
	if err != nil || offsetInt < 1 {
		offsetInt = 1
	}
	limitInt, err := strconv.Atoi(limitStr)
	if err != nil || limitInt < 1 {
		limitInt = 10
	}
	if err != nil {
		return nil, nil, appresult.ErrInternalServer
	}
	offsetInt = (offsetInt - 1) * limitInt

	return &limitInt, &offsetInt, nil
}
