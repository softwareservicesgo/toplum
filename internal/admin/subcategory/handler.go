package subcategory

import (
	"context"
	"encoding/json"
	"fmt"
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
	subcategoryURL             = ""
	subcategoryIdURL           = "/:id"
	subcategoryByCategoriesURL = "/byCategory"
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
	router.POST(subcategoryURL, middleware.JwtTokenCheck(h.client), h.create)
	router.GET(subcategoryIdURL, middleware.JwtTokenCheck(h.client), h.getOne)
	router.GET(subcategoryURL, middleware.JwtTokenCheck(h.client), h.getAll)
	router.PATCH(subcategoryIdURL, middleware.JwtTokenCheck(h.client), h.update)
	router.DELETE(subcategoryIdURL, middleware.JwtTokenCheck(h.client), h.delete)
	router.GET(subcategoryByCategoriesURL, middleware.JwtTokenCheck(h.client), h.byCategory)
}

func (h *handler) create(c *gin.Context) {
	var subcategory SubcategoryDTO

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
	if err := json.Unmarshal([]byte(jsonData), &subcategory); err != nil {
		appresult.HandleError(c, err)
		return
	}

	uploadDir := filepath.Join("uploads/subcategory")

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

	resp, err := h.repository.Create(context.TODO(), subcategory, imagePath, baseURL)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	c.JSON(http.StatusCreated, resp)
}

func (h *handler) getOne(c *gin.Context) {
	id := c.Param("id")
	subcategoryID, err := strconv.Atoi(id)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	baseURL := c.MustGet("baseURL").(string)

	resp, err := h.repository.GetOne(context.TODO(), subcategoryID, baseURL)
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
	categoryID := c.Query("category_id")

	baseURL := c.MustGet("baseURL").(string)

	resp, err := h.repository.GetAll(context.TODO(), search, limit, offset, categoryID, baseURL)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *handler) update(c *gin.Context) {

	var (
		subcategory SubcategoryDTO
		imagePath   string
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
		if err := json.Unmarshal([]byte(jsonData), &subcategory); err != nil {
			appresult.HandleError(c, err)
			return
		}
	}

	uploadDir := filepath.Join("uploads/subcategory")

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

	resp, err := h.repository.Update(context.TODO(), categoryId, subcategory, imagePath, baseURL)
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

	subcategoryId, err := strconv.Atoi(id)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	err = h.repository.Delete(context.TODO(), subcategoryId)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "success!!!",
	})
}

func (h *handler) byCategory(c *gin.Context) {
	categoryIDStr := c.Query("category_ids")

	categoryIDs := parseIntArray(categoryIDStr)

	baseURL := c.MustGet("baseURL").(string)

	resp, err := h.repository.ByCategory(context.TODO(), categoryIDs, baseURL)
	if err != nil {
		fmt.Println("error, ", err)
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
