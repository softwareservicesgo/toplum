package reservation

import (
	"context"
	"fmt"
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
	reservationDataByClientURL = "/reservationClient"
	reservationURL             = "/:id"
	reservationCheckURL        = "/check/:id"
	reservationClientURL       = "/byClient/:reservation_id"
	reservationRestaurantURL   = "/byRestaurant/:reservation_id"

	reservationAdminURL            = "/ws/admin/:restaurant_id"
	reservationClientWSURL         = "/ws/client/:restaurant_id"
	reservationAvailabilityDayURL  = "/ws/availability/day/:restaurant_id/"
	reservationAvailabilityTimeURL = "/ws/availability/time/:restaurant_id/"

	reservationURLWS = "/ws/:restaurant_id/"
)

type handler struct {
	Logger          *logging.Logger
	Repository      Repository
	WSRepository    RepositoryWS
	utilsRepository utils.Repository
	FCMClient       *fcm.FCMClient
	client          *pgxpool.Pool
}

func NewHandler(logger *logging.Logger, repository Repository, utilRepository utils.Repository, fcmClient *fcm.FCMClient, client *pgxpool.Pool) handlers.Handler {
	return &handler{
		Logger:          logger,
		Repository:      repository,
		utilsRepository: utilRepository,
		FCMClient:       fcmClient,
		client:          client,
	}
}

func (h *handler) Register(router *gin.RouterGroup) {
	router.POST(reservationURL, middleware.JwtTokenCheck(h.client), h.create)
	router.GET(reservationCheckURL, middleware.JwtTokenCheck(h.client), h.reservationForCheck)
	router.PUT(reservationURL, middleware.JwtTokenCheck(h.client), h.update)
	router.GET(reservationURL, middleware.JwtTokenCheck(h.client), h.getData)
	router.GET(reservationDataByClientURL, middleware.JwtTokenCheck(h.client), h.getDataByClient)
	router.PUT(reservationClientURL, middleware.JwtTokenCheck(h.client), h.deleteByClientReservation)
	router.PUT(reservationRestaurantURL, middleware.JwtTokenCheck(h.client), h.updateByRestaurantReservation)

	router.GET(reservationURLWS, middleware.JwtTokenCheck(h.client), h.reservations)
	router.GET(reservationAdminURL, middleware.JwtTokenCheck(h.client), h.wsHandler)
	router.GET(reservationClientWSURL, middleware.JwtTokenCheck(h.client), h.wsHandlerClient)
	router.GET(reservationAvailabilityDayURL, middleware.JwtTokenCheck(h.client), h.availabilityDay)
	router.GET(reservationAvailabilityTimeURL, middleware.JwtTokenCheck(h.client), h.availabilityTime)
}

func (h *handler) create(c *gin.Context) {
	var res ReservationReqDTO

	clientId, err := utils.ExtractUserIdFromToken(c, h.client)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	restaurantId, errR := strconv.Atoi(c.Param("id"))
	if errR != nil {
		appresult.HandleError(c, err)
		return
	}

	if err := c.ShouldBindJSON(&res); err != nil {
		appresult.HandleError(c, err)
		return
	}
	baseURL := c.MustGet("baseURL").(string)
	reservations, err := h.Repository.CreateRecervation(context.TODO(), restaurantId, clientId, res, baseURL)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	NotifyRestaurantUpdate(context.TODO(), restaurantId, 0, h.Repository, h.WSRepository)
	c.JSON(http.StatusCreated, reservations)
}

func (h *handler) reservationForCheck(c *gin.Context) {
	var res ReservationReqDTO

	clientId, err := utils.ExtractUserIdFromToken(c, h.client)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	restaurantId, errR := strconv.Atoi(c.Param("id"))
	if errR != nil {
		appresult.HandleError(c, err)
		return
	}

	if err := c.ShouldBindJSON(&res); err != nil {
		appresult.HandleError(c, err)
		return
	}
	baseURL := c.MustGet("baseURL").(string)
	reservations, err := h.Repository.ReservationForCheck(context.TODO(), restaurantId, clientId, res, baseURL)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, reservations)
}

func (h *handler) update(c *gin.Context) {
	var res ReservationPatchDTO

	clientId, err := utils.ExtractUserIdFromToken(c, h.client)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	reservationId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	if err := c.ShouldBindJSON(&res); err != nil {
		appresult.HandleError(c, err)
		return
	}

	baseURL := c.MustGet("baseURL").(string)
	reservations, restaurantId, err := h.Repository.Update(context.TODO(), reservationId, res, clientId, baseURL)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	NotifyRestaurantUpdate(context.TODO(), *restaurantId, 0, h.Repository, h.WSRepository)
	c.JSON(http.StatusCreated, reservations)
}

func (h *handler) getData(c *gin.Context) {
	role, err := h.extractUserIdAndRole(c)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}
	if *role != enum.RoleAdmin && *role != enum.RoleManager {
		appresult.HandleError(c, appresult.ErrForbidden)
		return
	}

	restaurantId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		appresult.HandleError(c, err)
		return
	}
	var filter ReservationFilter
	_ = c.ShouldBindQuery(&filter)

	baseURL := c.MustGet("baseURL").(string)
	allReservations, err := h.Repository.GetByRestaurantId(context.TODO(), restaurantId, filter, baseURL)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, allReservations)
}

func (h *handler) getDataByClient(c *gin.Context) {
	var filter ReservationClientFilter
	_ = c.ShouldBindQuery(&filter)

	clientId, err := utils.ExtractUserIdFromToken(c, h.client)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	baseURL := c.MustGet("baseURL").(string)
	allReservations, err := h.Repository.GetByClientId(context.TODO(), clientId, filter, baseURL)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, allReservations)
}

func (h *handler) deleteByClientReservation(c *gin.Context) {
	var reason Reason

	clientId, err := utils.ExtractUserIdFromToken(c, h.client)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	reservationId, err2 := strconv.Atoi(c.Param("reservation_id"))
	if err2 != nil {
		appresult.HandleError(c, err)
		return
	}

	if err := c.ShouldBindJSON(&reason); err != nil {
		appresult.HandleError(c, err)
		return
	}

	restaurantID, clientID, _, err := h.Repository.DeleteClientReservation(context.TODO(), clientId, reservationId, reason)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	NotifyRestaurantUpdate(context.TODO(), *restaurantID, *clientID, h.Repository, h.WSRepository)
	c.JSON(http.StatusCreated, gin.H{
		"message": "success!!!",
	})
}

func (h *handler) updateByRestaurantReservation(c *gin.Context) {
	var (
		status UpdateStatusByRestaurant
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

	userId, err := utils.ExtractUserIdFromToken(c, h.client)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	reservationId, err2 := strconv.Atoi(c.Param("reservation_id"))
	if err2 != nil {
		appresult.HandleError(c, err)
		return
	}

	if err := c.ShouldBindJSON(&status); err != nil {
		appresult.HandleError(c, err)
		return
	}

	restaurantId, clientId, FCMToken, err := h.Repository.UpdateByRestaurantStatusReservation(context.TODO(), userId, reservationId, status)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	NotifyRestaurantUpdate(context.TODO(), *restaurantId, *clientId, h.Repository, h.WSRepository)

	if status.Status == "cancelled_by_restaurant" && FCMToken != nil {
		ntf, err := h.Repository.FindForNotification(context.TODO(), *restaurantId, *clientId)

		if err != nil {
			fmt.Println("error: ", err)
		} else {
			err = h.FCMClient.SendToToken(context.TODO(), *FCMToken, ntf)
			if err != nil {
				fmt.Println("[ERROR-RES-CHECK] Push failed:", err)
			} else {
				fmt.Println("[JOB-RES-CHECK] Push sent to:", clientId)
			}
		}
	}
	c.JSON(http.StatusCreated, gin.H{
		"message": "success!!!",
	})
}

func (h *handler) extractUserIdAndRole(c *gin.Context) (*string, error) {
	userId, err := utils.ExtractUserIdFromToken(c, h.client)
	if err != nil {
		return nil, err
	}
	role, err := h.utilsRepository.UserRoleById(context.TODO(), userId, nil)
	if err != nil {
		return nil, err
	}
	return role, nil
}
