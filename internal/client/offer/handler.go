package offer

import (
	"context"
	"net/http"
	"restaurants/internal/appresult"
	"restaurants/internal/enum"
	"restaurants/internal/handlers"
	"restaurants/internal/middleware"
	"restaurants/pkg/logging"
	"restaurants/pkg/utils"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v4/pgxpool"
)

const (
	offerURL      = ""
	offerCleanURL = "/clean"
)

type handler struct {
	logger         *logging.Logger
	repository     Repository
	utilRepository utils.Repository
	client         *pgxpool.Pool
}

func NewHandler(
	logger *logging.Logger,
	repository Repository,
	utilRepository utils.Repository,
	client *pgxpool.Pool,
) handlers.Handler {
	return &handler{
		logger:         logger,
		repository:     repository,
		utilRepository: utilRepository,
		client:         client,
	}
}

func (h *handler) Register(router *gin.RouterGroup) {
	router.POST(offerURL, middleware.JwtTokenCheck(h.client), h.create)
	router.GET(offerURL, middleware.JwtTokenCheck(h.client), h.getAll)
	router.DELETE(offerCleanURL, middleware.JwtTokenCheck(h.client), h.clean)
}

func (h *handler) create(c *gin.Context) {
	var req CreateOfferReq

	userId, err := utils.ExtractUserIdFromToken(c, h.client)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		appresult.HandleError(c, err)
		return
	}

	resp, err := h.repository.Create(context.TODO(), req, userId)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	c.JSON(http.StatusCreated, resp)
}

func (h *handler) getAll(c *gin.Context) {
	role, err := h.extractUserIdAndRole(c, nil)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}
	if *role != enum.RoleAdmin {
		appresult.HandleError(c, appresult.ErrForbidden)
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "1"))

	resp, err := h.repository.GetAll(context.TODO(), limit, offset)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *handler) clean(c *gin.Context) {
	role, err := h.extractUserIdAndRole(c, nil)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}
	if *role != enum.RoleAdmin {
		appresult.HandleError(c, appresult.ErrForbidden)
		return
	}

	err = h.repository.Clean(context.TODO())
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "offers cleaned",
	})
}

func (h *handler) extractUserIdAndRole(c *gin.Context, businessId *int) (*string, error) {
	userId, err := utils.ExtractUserIdFromToken(c, h.client)
	if err != nil {
		return nil, err
	}
	if userId != -1 {
		role, err := h.utilRepository.UserRoleById(context.TODO(), userId, businessId)
		if err != nil {
			return nil, err
		}
		return role, nil
	}

	return nil, nil
}
