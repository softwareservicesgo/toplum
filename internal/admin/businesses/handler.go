package businesses

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"restaurants/internal/admin/item"
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
	businessesURL        = ""
	businessesById       = "/:id"
	businessesStatusById = "/status/:id"
	businessesIndex      = "/index"
)

type handler struct {
	logger          *logging.Logger
	repository      Repository
	utilsRepository utils.Repository
	foodRepository  item.Repository
	client          *pgxpool.Pool
}

func NewHandler(
	logger *logging.Logger,
	repository Repository,
	foodRepository item.Repository,
	utilsRepository utils.Repository,
	client *pgxpool.Pool,
) handlers.Handler {
	return &handler{
		logger:          logger,
		repository:      repository,
		utilsRepository: utilsRepository,
		foodRepository:  foodRepository,
		client:          client,
	}
}

func (h *handler) Register(router *gin.RouterGroup) {
	router.POST(businessesURL, middleware.JwtTokenCheck(h.client), h.create)
	router.GET(businessesById, middleware.JwtTokenCheck(h.client), h.getOne)
	router.GET(businessesURL, middleware.JwtTokenCheck(h.client), h.getAll)
	router.PATCH(businessesById, middleware.JwtTokenCheck(h.client), h.update)
	router.DELETE(businessesById, middleware.JwtTokenCheck(h.client), h.delete)
	router.PATCH(businessesStatusById, middleware.JwtTokenCheck(h.client), h.updateSatus)
	router.GET(businessesIndex, middleware.JwtTokenCheck(h.client), h.index)
}

func (h *handler) create(c *gin.Context) {
	var (
		business         BusinessesReqDTO
		additionalImages []string
	)

	userId, err := utils.ExtractUserIdFromToken(c, h.client)
	if err != nil {
		fmt.Println("error: ", err)
		appresult.HandleError(c, err)
		return
	}

	jsonData := c.PostForm("data")
	if err := json.Unmarshal([]byte(jsonData), &business); err != nil {
		fmt.Println("error: ", err)
		appresult.HandleError(c, err)
		return
	}

	mainImage, err := c.FormFile("mainImage")
	if err != nil {
		fmt.Println("error: ", err)
		appresult.HandleError(c, err)
		return
	}

	businessId, err := h.repository.Create(context.TODO(), userId, business)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	uploadDir := filepath.Join("uploads/businesses", fmt.Sprintf("businesses_%d", *businessId))
	if err := os.MkdirAll(uploadDir, os.ModePerm); err != nil {
		appresult.HandleError(c, err)
		return
	}

	mainImagePath, err := utils.SaveUploadedFile(c, mainImage, uploadDir)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	form, _ := c.MultipartForm()
	for _, file := range form.File["additionImages"] {
		filePath, err := utils.SaveUploadedFile(c, file, uploadDir)
		if err != nil {
			appresult.HandleError(c, err)
			return
		}
		additionalImages = append(additionalImages, filePath)
	}

	baseURL := c.MustGet("baseURL").(string)
	businessRes, err := h.repository.AddImages(context.TODO(), *businessId, mainImagePath, additionalImages, baseURL)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	c.JSON(http.StatusCreated, businessRes)
}

func (h *handler) getOne(c *gin.Context) {
	id := c.Param("id")
	businessId, err := strconv.Atoi(id)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	baseURL := c.MustGet("baseURL").(string)
	resp, err := h.repository.GetOne(context.TODO(), businessId, baseURL)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	items, err := h.foodRepository.GetItemsByBusiness(context.TODO(), businessId, baseURL)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}
	if items == nil || *items == nil {
		empty := []item.ItemGetAllDTO{}
		resp.Items = empty
	} else {
		resp.Items = *items
	}

	c.JSON(http.StatusOK, resp)
}

func (h *handler) getAll(c *gin.Context) {
	var filter BusinessesFilter
	_ = c.ShouldBindQuery(&filter)

	baseURL := c.MustGet("baseURL").(string)
	resp, count, err := h.repository.GetAll(context.TODO(), filter, baseURL)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, AllAndSum{
		Businesses: resp,
		Count:      *count,
	})
}

func (h *handler) update(c *gin.Context) {
	var (
		business         BusinessesReqDTO
		mainImagePath    string
		additionalImages []string
	)

	id := c.Param("id")
	businessId, err := strconv.Atoi(id)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	role, err := h.extractUserIdAndRole(c, &businessId)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}
	if *role != enum.RoleAdmin && *role != enum.RoleManager {
		appresult.HandleError(c, appresult.ErrForbidden)
		return
	}

	uploadDir := filepath.Join("uploads/businesses", fmt.Sprintf("businesses_%d", businessId))
	if err := os.MkdirAll(uploadDir, os.ModePerm); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot create upload dir"})
		return
	}

	mainImage, err := c.FormFile("mainImage")
	if err == nil && mainImage.Size > 0 {
		images, err := h.repository.GetAndDeleteImage(context.TODO(), businessId, true)
		if err != nil {
			appresult.HandleError(c, err)
			return
		}
		utils.DropFiles(images)

		mainImagePath, err = utils.SaveUploadedFile(c, mainImage, uploadDir)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot save main image"})
			return
		}
	} else if err != http.ErrMissingFile {
		appresult.HandleError(c, err)
		return
	}

	form, err := c.MultipartForm()
	if err == nil {
		files, ok := form.File["additionImages"]
		if ok && len(files) > 0 {
			images, err := h.repository.GetAndDeleteImage(context.TODO(), businessId, false)
			if err != nil {
				appresult.HandleError(c, err)
				return
			}
			utils.DropFiles(images)

			for _, file := range files {
				filePath, err := utils.SaveUploadedFile(c, file, uploadDir)
				if err != nil {
					appresult.HandleError(c, err)
					return
				}
				additionalImages = append(additionalImages, filePath)
			}
		}
	}

	if mainImagePath != "" || len(additionalImages) > 0 {
		if _, err := h.repository.AddImages(context.TODO(), businessId, mainImagePath, additionalImages, ""); err != nil {
			appresult.HandleError(c, err)
			return
		}
	}

	jsonData := c.PostForm("data")
	if jsonData != "" {
		if err := json.Unmarshal([]byte(jsonData), &business); err != nil {
			appresult.HandleError(c, err)
			return
		}
		err := h.repository.Update(context.TODO(), businessId, business)
		if err != nil {
			appresult.HandleError(c, err)
			return
		}
	}

	baseURL := c.MustGet("baseURL").(string)
	resp, err := h.repository.GetOne(context.TODO(), businessId, baseURL)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *handler) updateSatus(c *gin.Context) {
	var (
		status UpdateStatus
	)

	id := c.Param("id")
	businessId, err := strconv.Atoi(id)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	role, err := h.extractUserIdAndRole(c, &businessId)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}
	if *role != enum.RoleAdmin && *role != enum.RoleManager {
		appresult.HandleError(c, appresult.ErrForbidden)
		return
	}

	if err := c.ShouldBindJSON(&status); err != nil {
		fmt.Println("error binding JSON:", err)
		appresult.HandleError(c, err)
		return
	}

	err = h.repository.UpdateStatus(context.TODO(), businessId, status)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "success!!!",
	})
}

func (h *handler) delete(c *gin.Context) {
	id := c.Param("id")
	businessId, err := strconv.Atoi(id)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	role, err := h.extractUserIdAndRole(c, &businessId)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}
	if *role != enum.RoleAdmin {
		appresult.HandleError(c, appresult.ErrForbidden)
		return
	}

	err = h.repository.Delete(context.TODO(), businessId)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	folderPath := fmt.Sprintf("uploads/businesses_%d", businessId)
	_ = utils.RemoveFile(folderPath)

	c.JSON(http.StatusOK, gin.H{
		"message": "success!!!",
	})
}

func (h *handler) index(c *gin.Context) {
	var filter IndexFilter
	_ = c.ShouldBindQuery(&filter)

	if filter.SubcategoryIdsRaw != "" {
		filter.SubcategoryIds = parseIntArray(filter.SubcategoryIdsRaw)
	}

	if filter.CategoryIdRaw != "" {
		filter.CategoryIds = parseIntArray(filter.CategoryIdRaw)
	}

	baseURL := c.MustGet("baseURL").(string)
	resp, err := h.repository.Index(context.TODO(), filter, baseURL)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *handler) extractUserIdAndRole(c *gin.Context, businessId *int) (*string, error) {
	userId, err := utils.ExtractUserIdFromToken(c, h.client)
	if err != nil && userId != -1 {
		return nil, err
	}
	if userId != -1 {
		role, err := h.utilsRepository.UserRoleById(context.TODO(), userId, businessId)
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

func parseStringArray(query string) []string {
	query = strings.Trim(query, "[]")
	parts := strings.Split(query, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}
