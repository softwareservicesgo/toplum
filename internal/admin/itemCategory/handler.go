package itemCategory

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
	itemCategoryURL   = ""
	itemCategoryIdURL = "/:id"
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
	router.POST(itemCategoryURL, middleware.JwtTokenCheck(h.client), h.create)
	router.GET(itemCategoryIdURL, middleware.JwtTokenCheck(h.client), h.getOne)
	router.GET(itemCategoryURL, middleware.JwtTokenCheck(h.client), h.getAll)
	router.PUT(itemCategoryIdURL, middleware.JwtTokenCheck(h.client), h.update)
	router.DELETE(itemCategoryIdURL, middleware.JwtTokenCheck(h.client), h.delete)
}

func (h *handler) create(c *gin.Context) {
	var dto ItemCategoryCreateDTO
	if err := c.ShouldBindJSON(&dto); err != nil {
		appresult.HandleError(c, err)
		return
	}

	role, err := h.extractUserIdAndRole(c, dto.BusinessID)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	if *role != enum.RoleAdmin && *role != enum.RoleManager {
		appresult.HandleError(c, appresult.ErrForbidden)
		return
	}

	resp, err := h.repository.Create(context.TODO(), dto)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	c.JSON(http.StatusCreated, resp)
}

func (h *handler) getOne(c *gin.Context) {
	id := c.Param("id")
	itemCategoryId, err := strconv.Atoi(id)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}
	resp, err := h.repository.GetOne(context.TODO(), itemCategoryId)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *handler) getAll(c *gin.Context) {
	businessId := c.Query("businesses_id")
	limit := c.Query("limit")
	offset := c.Query("offset")
	search := c.Query("search")

	resp, err := h.repository.GetAll(context.TODO(), businessId, search, limit, offset)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *handler) update(c *gin.Context) {
	var (
		itemCategory ItemCategoryNameDTO
	)

	id := c.Param("id")
	itemCategoryId, err := strconv.Atoi(id)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	businessID, err := h.repository.GetBusinessesById(context.TODO(), itemCategoryId)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	role, err := h.extractUserIdAndRole(c, *businessID)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	if *role != enum.RoleAdmin && *role != enum.RoleManager {
		appresult.HandleError(c, appresult.ErrForbidden)
		return
	}

	if err := c.ShouldBindJSON(&itemCategory); err != nil {
		appresult.HandleError(c, err)
		return
	}

	resp, err := h.repository.Update(context.TODO(), itemCategoryId, itemCategory)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *handler) delete(c *gin.Context) {
	id := c.Param("id")
	itemCategoryId, err := strconv.Atoi(id)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	businessID, err := h.repository.GetBusinessesById(context.TODO(), itemCategoryId)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	role, err := h.extractUserIdAndRole(c, *businessID)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	if *role != enum.RoleAdmin && *role != enum.RoleManager {
		appresult.HandleError(c, appresult.ErrForbidden)
		return
	}

	err = h.repository.Delete(context.TODO(), itemCategoryId)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "success!!!",
	})
}

func (h *handler) extractUserIdAndRole(c *gin.Context, businessesId int) (*string, error) {
	userId, err := utils.ExtractUserIdFromToken(c, h.client)
	if err != nil {
		return nil, err
	}
	if userId != -1 {
		role, err := h.utilsRepository.UserRoleById(context.TODO(), userId, &businessesId)
		if err != nil {
			return nil, err
		}
		return role, nil
	}

	return nil, nil
}
