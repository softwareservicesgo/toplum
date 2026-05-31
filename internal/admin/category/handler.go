package category

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

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v4/pgxpool"
)

const (
	categoryURL  = ""
	categoryById = "/:id"
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
	router.POST(categoryURL, middleware.JwtTokenCheck(h.client), h.create)
	router.GET(categoryById, middleware.JwtTokenCheck(h.client), h.getOne)
	router.GET(categoryURL, middleware.JwtTokenCheck(h.client), h.getAll)
	router.PATCH(categoryById, middleware.JwtTokenCheck(h.client), h.update)
	router.DELETE(categoryById, middleware.JwtTokenCheck(h.client), h.delete)
}

func (h *handler) create(c *gin.Context) {
	var category CategoryReqDTO

	role, err := h.extractUserIdAndRole(c)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	if *role != enum.RoleAdmin {
		appresult.HandleError(c, appresult.ErrForbidden)
		return
	}

	jsonData := c.PostForm("data")
	if err := json.Unmarshal([]byte(jsonData), &category); err != nil {
		appresult.HandleError(c, err)
		return
	}

	uploadDir := filepath.Join("uploads/category")

	if err := os.MkdirAll(uploadDir, os.ModePerm); err != nil {
		appresult.HandleError(c, err)
		return
	}

	image, err := c.FormFile("image")
	if err != nil {
		appresult.HandleError(c, appresult.ErrNotImage)
		return
	}

	imagePath, err := utils.SaveUploadedFile(c, image, uploadDir)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	baseURL := c.MustGet("baseURL").(string)

	resp, err := h.repository.Create(context.TODO(), category, *imagePath, baseURL)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	c.JSON(http.StatusCreated, resp)
}

func (h *handler) getOne(c *gin.Context) {
	id := c.Param("id")

	categoryId, err := strconv.Atoi(id)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	baseURL := c.MustGet("baseURL").(string)

	resp, err := h.repository.GetOne(context.TODO(), categoryId, baseURL)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *handler) getAll(c *gin.Context) {
	search := c.Query("search")
	limit := c.Query("limit")
	offset := c.Query("offset")

	baseURL := c.MustGet("baseURL").(string)

	resp, err := h.repository.GetAll(context.TODO(), search, limit, offset, baseURL)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *handler) update(c *gin.Context) {
	var (
		category  CategoryReqDTO
		imagePath string
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

	categoryId, err := strconv.Atoi(id)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	jsonData := c.PostForm("data")

	if jsonData != "" {
		if err := json.Unmarshal([]byte(jsonData), &category); err != nil {
			appresult.HandleError(c, err)
			return
		}
	}

	uploadDir := filepath.Join("uploads/category")

	if err := os.MkdirAll(uploadDir, os.ModePerm); err != nil {
		appresult.HandleError(c, err)
		return
	}

	image, err := c.FormFile("image")

	if err == nil {

		imagePathNew, err := utils.SaveUploadedFile(c, image, uploadDir)
		if err != nil {
			appresult.HandleError(c, err)
			return
		}
		imagePath = *imagePathNew

	}

	baseURL := c.MustGet("baseURL").(string)

	resp, err := h.repository.Update(context.TODO(), categoryId, category, imagePath, baseURL)
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

	categoryId, err := strconv.Atoi(id)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	err = h.repository.Delete(context.TODO(), categoryId)
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
