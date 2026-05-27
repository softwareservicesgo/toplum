package user

import (
	"context"
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
	userURL        = ""
	userById       = "/:id"
	userByBusiness = "/byBusinesses/:businessesId"
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
	router.POST(userURL, middleware.JwtTokenCheck(h.client), h.create)
	router.GET(userById, middleware.JwtTokenCheck(h.client), h.getOne)
	router.GET(userURL, middleware.JwtTokenCheck(h.client), h.getAll)
	router.PATCH(userById, middleware.JwtTokenCheck(h.client), h.update)
	router.DELETE(userById, middleware.JwtTokenCheck(h.client), h.delete)
	router.GET(userByBusiness, middleware.JwtTokenCheck(h.client), h.userByBusinessId)
}

func (h *handler) create(c *gin.Context) {
	var User UserReqDTO

	role, _, err := h.extractUserIdAndRole(c)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	if err := c.ShouldBindJSON(&User); err != nil {
		log.Println("error binding JSON:", err)
		appresult.HandleError(c, err)
		return
	}

	if (User.Role == enum.RoleManager && *role != enum.RoleAdmin) ||
		(User.Role == enum.RoleEmployee && *role == enum.RoleEmployee) {
		appresult.HandleError(c, appresult.ErrForbidden)
		return
	}

	if User.Role != enum.RoleManager && User.Role != enum.RoleEmployee {
		appresult.HandleError(c, appresult.ErrForbidden)
		return
	}

	hashPassword, err := utils.HashPassword(User.Password)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	user, err := h.repository.Create(context.TODO(), User, hashPassword)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	c.JSON(http.StatusCreated, user)
}

func (h *handler) getOne(c *gin.Context) {
	role, _, err := h.extractUserIdAndRole(c)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}
	if *role != enum.RoleAdmin && *role != enum.RoleManager && *role != enum.RoleEmployee {
		appresult.HandleError(c, appresult.ErrForbidden)
		return
	}

	id := c.Param("id")
	userId, err := strconv.Atoi(id)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	resp, err := h.repository.GetOne(context.TODO(), userId)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *handler) getAll(c *gin.Context) {
	role, _, err := h.extractUserIdAndRole(c)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	if *role != enum.RoleAdmin && *role != enum.RoleManager {
		appresult.HandleError(c, appresult.ErrForbidden)
		return
	}

	params := Fields
	values := make(map[string]string)

	for _, param := range params {
		values[param] = c.Query(param)
	}

	resp, count, err := h.repository.GetAll(context.TODO(), values)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	allAndSum := AllAndSum{
		Businesses: resp,
		Count:      *count,
	}

	c.JSON(http.StatusOK, allAndSum)
}

func (h *handler) update(c *gin.Context) {
	var (
		User         UserUpdateDTO
		hashPassword string
	)

	role, tokenUserId, err := h.extractUserIdAndRole(c)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	id := c.Param("id")
	userId, err := strconv.Atoi(id)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	if userId != *tokenUserId && *role != enum.RoleAdmin && *role != enum.RoleManager {
		appresult.HandleError(c, appresult.ErrForbidden)
		return
	}

	if err := c.ShouldBindJSON(&User); err != nil {
		appresult.HandleError(c, err)
		return
	}
	if User.Password != "" {
		hashPassword, err = utils.HashPassword(User.Password)
		if err != nil {
			appresult.HandleError(c, err)
			return
		}
	}

	resp, err := h.repository.Update(context.TODO(), userId, User, hashPassword)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *handler) delete(c *gin.Context) {
	role, _, err := h.extractUserIdAndRole(c)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	if *role != enum.RoleAdmin && *role != enum.RoleManager {
		appresult.HandleError(c, appresult.ErrForbidden)
		return
	}

	id := c.Param("id")
	userId, err := strconv.Atoi(id)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	err = h.repository.Delete(context.TODO(), userId, *role)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "success!!!",
	})
}

func (h *handler) userByBusinessId(c *gin.Context) {
	var (
		limit, offset int
	)
	role, _, err := h.extractUserIdAndRole(c)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	if *role != enum.RoleAdmin && *role != enum.RoleManager && *role != enum.RoleEmployee {
		appresult.HandleError(c, appresult.ErrForbidden)
		return
	}

	id := c.Param("businessesId")
	businessId, err := strconv.Atoi(id)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	limitStr := c.Query("limit")
	if limitStr == "" {
		limit = 10
	} else {
		limit, err = strconv.Atoi(limitStr)
		if err != nil {
			appresult.HandleError(c, err)
			return
		}
	}

	offsetStr := c.Query("offset")
	if offsetStr == "" {
		offset = 1
	} else {
		offset, err = strconv.Atoi(offsetStr)
		if err != nil {
			appresult.HandleError(c, err)
			return
		}
	}

	resp, err := h.repository.GetAllByBusiness(context.TODO(), businessId, limit, offset)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *handler) extractUserIdAndRole(c *gin.Context) (*string, *int, error) {
	userID, err := utils.ExtractUserIdFromToken(c, h.client)
	if err != nil {
		return nil, nil, err
	}

	role, err := h.utilsRepository.UserRoleById(context.TODO(), userID, nil)
	if err != nil {
		return nil, nil, err
	}

	return role, &userID, nil
}
