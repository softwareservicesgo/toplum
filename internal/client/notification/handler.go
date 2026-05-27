package notification

import (
	"context"
	"net/http"
	"restaurants/internal/appresult"
	"restaurants/internal/enum"
	"restaurants/internal/handlers"
	"restaurants/internal/middleware"
	"restaurants/pkg/fcm"
	"restaurants/pkg/logging"
	"restaurants/pkg/utils"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v4/pgxpool"
)

const (
	notificationURL           = ""
	notificationByClientURL   = "/client"
	notificationClientReadURL = "/readClient"
	notificationByIdURL       = "/:id"
)

type handler struct {
	Logger          *logging.Logger
	Repository      Repository
	fcmClient       *fcm.FCMClient
	utilsRepository utils.Repository
	client          *pgxpool.Pool
}

func NewHandler(logger *logging.Logger, repository Repository, fcmClient *fcm.FCMClient, utilsRepository utils.Repository, client *pgxpool.Pool) handlers.Handler {
	return &handler{
		Logger:          logger,
		Repository:      repository,
		fcmClient:       fcmClient,
		utilsRepository: utilsRepository,
		client:          client,
	}
}

func (h *handler) Register(router *gin.RouterGroup) {
	router.POST(notificationURL, middleware.JwtTokenCheck(h.client), h.create)
	router.GET(notificationByIdURL, middleware.JwtTokenCheck(h.client), h.getOne)
	router.GET(notificationURL, middleware.JwtTokenCheck(h.client), h.getAll)
	router.GET(notificationByClientURL, middleware.JwtTokenCheck(h.client), h.getAllByClient)
	router.PUT(notificationClientReadURL, middleware.JwtTokenCheck(h.client), h.readClient)
	router.DELETE(notificationByIdURL, middleware.JwtTokenCheck(h.client), h.delete)
}

func (h *handler) create(c *gin.Context) {
	var notification NotificationReqDTO

	role, err := h.extractUserIdAndRole(c)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}
	if *role != enum.RoleAdmin && *role != enum.RoleManager {
		appresult.HandleError(c, appresult.ErrForbidden)
		return
	}

	userId, err := utils.ExtractUserIdFromToken(c, h.client)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	if err := c.ShouldBindJSON(&notification); err != nil {
		appresult.HandleError(c, err)
		return
	}

	notificationRes, err := h.Repository.CreateNotification(context.TODO(), notification, userId)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	if err = h.fcmClient.SendToTopic(c, fcm.AllUsersTopicName, notificationRes); err != nil {
		h.Logger.Errorf("Error sending notification to topic: %v", err)
	}

	//TODO add this place to broker or at least change to bulk insert inside of func
	//err = h.Repository.CreateClientNotification(context.TODO(), tokens, notificationRes.Id)
	//if err != nil {
	//	c.JSON(http.StatusInternalServerError, appresult.ErrInternalServer)
	//	return
	//}

	c.JSON(http.StatusCreated, notificationRes)
}

func (h *handler) getOne(c *gin.Context) {
	notificationId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	notification, err := h.Repository.GetNotificationById(context.TODO(), notificationId)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, notification)
}

func (h *handler) getAll(c *gin.Context) {
	role, err := h.extractUserIdAndRole(c)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}
	if *role != enum.RoleAdmin && *role != enum.RoleManager && *role != enum.RoleEmployee {
		appresult.HandleError(c, appresult.ErrForbidden)
		return
	}
	var filter NotificationFilter
	_ = c.ShouldBindQuery(&filter)

	notifications, err := h.Repository.GetAll(context.TODO(), filter)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, notifications)
}

func (h *handler) getAllByClient(c *gin.Context) {
	var filter NotificationClientFilter

	clientId, err := utils.ExtractUserIdFromToken(c, h.client)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	deviceToken := c.Query("device_token")
	if deviceToken == "" {
		appresult.HandleError(c, err)
		return
	}
	_ = c.ShouldBindQuery(&filter)

	notifications, err := h.Repository.GetAllByClient(context.TODO(), clientId, deviceToken, filter)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, notifications)
}

func (h *handler) readClient(c *gin.Context) {
	clientId, err := utils.ExtractUserIdFromToken(c, h.client)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	deviceToken := c.Query("device_token")
	if deviceToken == "" {
		appresult.HandleError(c, err)
		return
	}

	err = h.Repository.ReadClient(context.TODO(), clientId, deviceToken)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, "Client read notifications")
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

	userId, err := utils.ExtractUserIdFromToken(c, h.client)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	notificationId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	err = h.Repository.Delete(context.TODO(), notificationId, userId)
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
