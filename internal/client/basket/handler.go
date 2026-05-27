package basket

import (
	"context"
	"net/http"
	"restaurants/internal/appresult"
	"restaurants/internal/handlers"
	"restaurants/internal/middleware"
	"restaurants/pkg/logging"
	"restaurants/pkg/utils"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v4/pgxpool"
)

const (
	basketURL  = ""
	basketById = "/:id"
)

type handler struct {
	logger          *logging.Logger
	repository      Repository
	utilsRepository utils.Repository
	client          *pgxpool.Pool
}

func NewHandler(logger *logging.Logger, repository Repository, utilsRepository utils.Repository, client *pgxpool.Pool) handlers.Handler {
	return &handler{
		logger:          logger,
		repository:      repository,
		utilsRepository: utilsRepository,
		client:          client,
	}
}

func (h *handler) Register(router *gin.RouterGroup) {
	router.POST(basketURL, middleware.JwtTokenCheck(h.client), h.create)
	router.GET(basketById, middleware.JwtTokenCheck(h.client), h.getOne)
	router.GET(basketURL, middleware.JwtTokenCheck(h.client), h.getAll)
	router.DELETE(basketById, middleware.JwtTokenCheck(h.client), h.delete)
}

func (h *handler) create(c *gin.Context) {
	var (
		foods BasketReq
	)

	clientId, err := utils.ExtractUserIdFromToken(c, h.client)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	if err := c.ShouldBindJSON(&foods); err != nil {
		appresult.HandleError(c, err)
		return
	}

	err = h.repository.Create(context.TODO(), clientId, foods)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	c.JSON(http.StatusCreated, "succsess!!!")
}

func (h *handler) getOne(c *gin.Context) {
	clientId, err := utils.ExtractUserIdFromToken(c, h.client)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}
	id := c.Param("id")
	restId, err := strconv.Atoi(id)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	clientCouponId := c.Query("clientCoupondId")

	baseURL := c.MustGet("baseURL").(string)
	resp, err := h.repository.GetOne(context.TODO(), clientId, restId, clientCouponId, baseURL)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *handler) getAll(c *gin.Context) {
	clientId, err := utils.ExtractUserIdFromToken(c, h.client)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	page := c.Query("page")
	size := c.Query("size")

	baseURL := c.MustGet("baseURL").(string)

	resp, err := h.repository.GetAll(context.TODO(), clientId, page, size, baseURL)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *handler) delete(c *gin.Context) {
	clientId, err := utils.ExtractUserIdFromToken(c, h.client)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	id := c.Param("id")
	foodId, err := strconv.Atoi(id)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	err = h.repository.Delete(context.TODO(), clientId, foodId)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, "succsess!!!")
}
