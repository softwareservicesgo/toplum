package item

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"restaurants/internal/appresult"
	"restaurants/internal/enum"
	"restaurants/internal/handlers"
	"restaurants/internal/middleware"
	"restaurants/pkg/logging"
	"restaurants/pkg/utils"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v4/pgxpool"
)

const (
	itemURL           = ""
	itemById          = "/:id"
	itemForUpdateById = "/forUpdate/:id"
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
	router.POST(itemURL, middleware.JwtTokenCheck(h.client), h.create)
	router.GET(itemById, middleware.JwtTokenCheck(h.client), h.getOne)
	router.GET(itemURL, middleware.JwtTokenCheck(h.client), h.getAll)
	router.PATCH(itemById, middleware.JwtTokenCheck(h.client), h.update)
	router.DELETE(itemById, middleware.JwtTokenCheck(h.client), h.delete)
	router.GET(itemForUpdateById, middleware.JwtTokenCheck(h.client), h.getForUpdate)
}

func (h *handler) create(c *gin.Context) {
	var item ItemReqDTO

	role, err := h.extractUserIdAndRole(c)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	if *role != enum.RoleAdmin && *role != enum.RoleManager {
		appresult.HandleError(c, appresult.ErrForbidden)
		return
	}

	jsonData := c.PostForm("data")
	if err := json.Unmarshal([]byte(jsonData), &item); err != nil {
		appresult.HandleError(c, err)
		return
	}

	uploadDir := filepath.Join("uploads/item")
	if err := os.MkdirAll(uploadDir, os.ModePerm); err != nil {
		appresult.HandleError(c, err)
		return
	}

	image, err := c.FormFile("image")
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	imagePath, err := utils.SaveUploadedFile(c, image, uploadDir)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	baseURL := c.MustGet("baseURL").(string)

	resp, err := h.repository.Create(context.TODO(), item, imagePath, baseURL)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	c.JSON(http.StatusCreated, resp)
}

func (h *handler) getOne(c *gin.Context) {
	id := c.Param("id")
	itemId, err := strconv.Atoi(id)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	baseURL := c.MustGet("baseURL").(string)

	resp, err := h.repository.GetOne(context.TODO(), itemId, baseURL)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *handler) getAll(c *gin.Context) {
	var filter ItemFilter
	_ = c.ShouldBindQuery(&filter)

	filter.ItemCategoryIds = parseIntArray(filter.ItemCategoryIdsStr)

	baseURL := c.MustGet("baseURL").(string)

	resp, err := h.repository.GetAll(context.TODO(), filter, baseURL)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *handler) update(c *gin.Context) {
	var (
		imagePath string
		item      ItemReqDTO
	)

	role, err := h.extractUserIdAndRole(c)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	if *role != enum.RoleAdmin && *role != enum.RoleManager {
		appresult.HandleError(c, appresult.ErrForbidden)
		return
	}

	id := c.Param("id")
	itemId, err := strconv.Atoi(id)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	jsonData := c.PostForm("data")
	if jsonData != "" {
		if err := json.Unmarshal([]byte(jsonData), &item); err != nil {
			appresult.HandleError(c, err)
			return
		}
	}

	uploadDir := filepath.Join("uploads/item")
	if err := os.MkdirAll(uploadDir, os.ModePerm); err != nil {
		appresult.HandleError(c, err)
		return
	}

	image, err := c.FormFile("image")
	if err == nil {
		imagePath, err = utils.SaveUploadedFile(c, image, uploadDir)
		if err != nil {
			appresult.HandleError(c, err)
			return
		}
	}

	baseURL := c.MustGet("baseURL").(string)

	resp, err := h.repository.Update(context.TODO(), itemId, item, imagePath, baseURL)
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

	if *role != enum.RoleAdmin && *role != enum.RoleManager {
		appresult.HandleError(c, appresult.ErrForbidden)
		return
	}

	id := c.Param("id")
	itemId, err := strconv.Atoi(id)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	err = h.repository.Delete(context.TODO(), itemId)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "success!!!",
	})
}

func (h *handler) getForUpdate(c *gin.Context) {
	role, err := h.extractUserIdAndRole(c)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	if *role != enum.RoleAdmin && *role != enum.RoleManager {
		appresult.HandleError(c, appresult.ErrForbidden)
		return
	}

	id := c.Param("id")
	itemId, err := strconv.Atoi(id)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	baseURL := c.MustGet("baseURL").(string)

	resp, err := h.repository.GetForUpdate(context.TODO(), itemId, baseURL)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *handler) extractUserIdAndRole(c *gin.Context) (*string, error) {
	userId, err := utils.ExtractUserIdFromToken(c, h.client)
	if err != nil {
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

func parseIntArray(query string) []int {
	query = strings.Trim(query, "[]")
	parts := strings.Split(query, ",")
	res := []int{}
	for _, p := range parts {
		if v, err := strconv.Atoi(strings.TrimSpace(p)); err == nil {
			res = append(res, v)
		}
	}
	return res
}
