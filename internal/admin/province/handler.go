package province

import (
	"context"
	"fmt"
	"log"
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
	provinceURL  = ""
	provinceById = "/:id"
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
	router.POST(provinceURL, middleware.JwtTokenCheck(h.client), h.create)
	router.GET(provinceById, middleware.JwtTokenCheck(h.client), h.getOne)
	router.GET(provinceURL, middleware.JwtTokenCheck(h.client), h.getAll)
	router.PUT(provinceById, middleware.JwtTokenCheck(h.client), h.update)
	router.DELETE(provinceById, middleware.JwtTokenCheck(h.client), h.delete)
}

func (h *handler) create(c *gin.Context) {
	var (
		province DictionaryDTO
	)

	role, err := h.extractUserIdAndRole(c)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	if *role != enum.RoleAdmin {
		appresult.HandleError(c, appresult.ErrForbidden)
		return
	}

	if err := c.ShouldBindJSON(&province); err != nil {
		fmt.Println("error binding JSON:", err)
		appresult.HandleError(c, err)
		return
	}

	resp, err := h.repository.Create(context.TODO(), province)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	c.JSON(http.StatusCreated, resp)
}

func (h *handler) getOne(c *gin.Context) {
	id := c.Param("id")
	provinceId, err := strconv.Atoi(id)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	resp, err := h.repository.GetOne(context.TODO(), provinceId)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *handler) getAll(c *gin.Context) {
	search := c.Query("search")
	page := c.Query("page")
	size := c.Query("size")

	resp, err := h.repository.GetAll(context.TODO(), search, page, size)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *handler) update(c *gin.Context) {
	var (
		province DictionaryDTO
	)

	role, err := h.extractUserIdAndRole(c)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	if *role != enum.RoleAdmin {
		appresult.HandleError(c, appresult.ErrForbidden)
		return
	}

	id := c.Param("id")
	provinceId, err := strconv.Atoi(id)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	if err := c.ShouldBindJSON(&province); err != nil {
		log.Println("error binding JSON:", err)
		appresult.HandleError(c, err)
		return
	}

	resp, err := h.repository.Update(context.TODO(), provinceId, province)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *handler) delete(c *gin.Context) {
	role, err := h.extractUserIdAndRole(c)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	if *role != enum.RoleAdmin {
		appresult.HandleError(c, appresult.ErrForbidden)
		return
	}

	id := c.Param("id")
	provinceId, err := strconv.Atoi(id)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	err = h.repository.Delete(context.TODO(), provinceId)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "success!!!",
	})
}

func (h *handler) extractUserIdAndRole(c *gin.Context) (*string, error) {
	userId, err := utils.ExtractUserIdFromToken(c, h.client)
	if err != nil && userId != -1 {
		return nil, err
	}
	if userId != -1 {
		role, err := h.utilsRepository.UserRoleById(context.TODO(), userId, nil)
		if err != nil {
			return nil, err
		}
		return role, nil
	}

	return nil, nil
}
