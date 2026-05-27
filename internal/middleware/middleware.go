package middleware

import (
	"context"
	"fmt"
	"net/http"
	"restaurants/pkg/utils"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v4/pgxpool"
)

type UnsignedResponse struct {
	Message interface{} `json:"message"`
}

type appHandler func(w http.ResponseWriter, r *http.Request) error

func HeaderContentTypeJson() (string, string) {
	return "Content-Type", "application/json"
}
func AccessControlAllow() (string, string) {
	return "Access-Control-Allow-Origin", "*"
}

func respondWithError(c *gin.Context, code int, message interface{}) {
	c.AbortWithStatusJSON(code, gin.H{"error": message})
}

func BaseURLMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		scheme := "http"
		if c.Request.TLS != nil {
			scheme = "https"
		}
		baseURL := fmt.Sprintf("%s://%s", scheme, c.Request.Host)
		c.Set("baseURL", baseURL)

		c.Next()
	}
}

func JwtTokenCheck(db *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString := c.GetHeader("Authorization")
		if tokenString == "" {
			c.Next()
			return
		}

		if len(tokenString) < 7 || tokenString[:7] != "Bearer " {
			c.AbortWithStatusJSON(http.StatusUnauthorized, UnsignedResponse{Message: "invalid format"})
			return
		}

		accessToken := tokenString[7:]

		var exists bool
		err := db.QueryRow(context.TODO(),
			"SELECT EXISTS(SELECT 1 FROM blacklist WHERE token = $1)", accessToken,
		).Scan(&exists)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, UnsignedResponse{Message: "internal server error"})
			return
		}
		if exists {
			c.AbortWithStatusJSON(http.StatusUnauthorized, UnsignedResponse{Message: "token is blacklisted"})
			return
		}

		if _, err := utils.ParseToken(accessToken); err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, UnsignedResponse{Message: "invalid token"})
			return
		}

		c.Next()
	}
}
