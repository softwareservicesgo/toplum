package auth

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"restaurants/internal/appresult"
	"restaurants/internal/handlers"
	"restaurants/pkg/logging"
	"restaurants/pkg/utils"
)

const (
	loginURL = "/login"
)

type handler struct {
	logger     *logging.Logger
	repository Repository
}

func NewHandler(logger *logging.Logger, repository Repository) handlers.Handler {
	return &handler{
		logger:     logger,
		repository: repository,
	}
}

func (h *handler) Register(router *gin.RouterGroup) {
	router.POST(loginURL, h.login)
}

func (h *handler) login(c *gin.Context) {
	var (
		login LoginDTO
	)

	if err := c.ShouldBindJSON(&login); err != nil {
		fmt.Println("error binding JSON:", err)
		appresult.HandleError(c, err)
		return
	}

	resp, err := h.repository.Login(context.TODO(), login)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	token, err := utils.GenerateTokenPair(resp.Id)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	successResult := ReqLoginDTO{
		Token:        token,
		Id:           resp.Id,
		Role:         resp.Role,
		Name:         resp.Name,
		BusinessesId: resp.BusinessesId,
	}
	c.JSON(http.StatusOK, successResult)
}
