package deviceToken

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"restaurants/internal/appresult"
	"restaurants/internal/handlers"
	"restaurants/pkg/logging"
	"strconv"

	"github.com/gin-gonic/gin"
)

const (
	reviewURL     = ""
	reviewByIdURL = "/:id"
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
	router.POST(reviewURL, h.create)
	router.DELETE(reviewByIdURL, h.delete)
}

func (h *handler) create(c *gin.Context) {
	var (
		token DeviceToken
	)

	if err := c.ShouldBindJSON(&token); err != nil {
		fmt.Println("error binding JSON:", err)
		c.JSON(http.StatusBadRequest, appresult.ErrMissingParam)
		return
	}

	err := h.repository.Create(context.TODO(), token)
	if err != nil {
		if errors.Is(err, appresult.ErrAlreadyDataa) {
			c.JSON(http.StatusBadRequest, err)
			return
		}
		c.JSON(http.StatusInternalServerError, appresult.ErrInternalServer)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "success!!!",
	})
}

func (h *handler) delete(c *gin.Context) {
	clientIdStr := c.Param("id")
	clientId, err := strconv.Atoi(clientIdStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, appresult.ErrInternalServer)
		return
	}
	err = h.repository.Delete(context.TODO(), clientId)
	if err != nil {
		if errors.Is(err, appresult.ErrNotFoundBase) {
			c.JSON(http.StatusBadRequest, err)
			return
		}
		c.JSON(http.StatusInternalServerError, appresult.ErrInternalServer)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "success!!!",
	})
}
