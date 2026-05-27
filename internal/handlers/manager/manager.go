package handlermanager

import (
	"restaurants/internal/admin/sms_socket"
	"restaurants/internal/middleware"
	"restaurants/pkg/sms_sender"

	"restaurants/internal/admin/auth"
	authdb "restaurants/internal/admin/auth/db"

	province "restaurants/internal/admin/province"
	provincedb "restaurants/internal/admin/province/db"

	subcategory "restaurants/internal/admin/subcategory"
	subcategorydb "restaurants/internal/admin/subcategory/db"

	businesses "restaurants/internal/admin/businesses"
	businessesdb "restaurants/internal/admin/businesses/db"

	user "restaurants/internal/admin/user"
	userdb "restaurants/internal/admin/user/db"

	itemCategory "restaurants/internal/admin/itemCategory"
	itemCategorydb "restaurants/internal/admin/itemCategory/db"

	item "restaurants/internal/admin/item"
	itemdb "restaurants/internal/admin/item/db"

	offer "restaurants/internal/client/offer"
	offerdb "restaurants/internal/client/offer/db"

	userClient "restaurants/internal/client/user"
	userClientdb "restaurants/internal/client/user/db"

	reservation "restaurants/internal/client/reservation"
	reservationdb "restaurants/internal/client/reservation/db"

	notification "restaurants/internal/client/notification"
	notificationdb "restaurants/internal/client/notification/db"

	deviceToken "restaurants/internal/client/deviceToken"
	deviceTokendb "restaurants/internal/client/deviceToken/db"

	basket "restaurants/internal/client/basket"
	basketdb "restaurants/internal/client/basket/db"

	orderClient "restaurants/internal/client/order"
	orderClientdb "restaurants/internal/client/order/db"
	orderClientWS "restaurants/internal/client/order/dbWS"

	category "restaurants/internal/admin/category"
	categorydb "restaurants/internal/admin/category/db"

	"restaurants/pkg/fcm"
	utilsdb "restaurants/pkg/utils/db"

	"restaurants/pkg/logging"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v4/pgxpool"
	_ "github.com/swaggo/swag"
)

const (
	authURL         = "/api/v1/auth"
	provinceURL     = "/api/v1/province"
	subcategoryURL  = "/api/v1/subcategory"
	businessesURL   = "/api/v1/businesses"
	userAdminURL    = "/api/v1/userAdmin"
	smsSocketURL    = "/api/v1/sms_socket"
	typeURL         = "/api/v1/type"
	itemCategoryURL = "/api/v1/itemCategory"
	itemURL         = "/api/v1/item"
	orderAdminURL   = "/api/v1/admin/order"
	categoryURL     = "/api/v1/category"

	userURL         = "/api/v1/user"
	offerURL        = "/api/v1/offer"
	reservationURL  = "/api/v1/reservation"
	notificationURL = "/api/v1/notification"
	deviceTokenURL  = "/api/v1/deviceToken"
	basketURL       = "/api/v1/basket"
	orderClientURL  = "/api/v1/order"
	organizationURL = "/api/v1/organization"
)

func Manager(client *pgxpool.Pool, logger *logging.Logger, smsSender *sms_sender.Client, fcmClient *fcm.FCMClient) *gin.Engine {
	router := gin.Default()

	router.Use(func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin == "http://localhost:3000" || origin == "https://prechemical-bibi-pentomic.ngrok-free.dev" {
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
		}

		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers",
			"Content-Type, Content-Length, Accept-Encoding, Authorization, accept, origin, Cache-Control, X-Requested-With, ngrok-skip-browser-warning")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE, PATCH")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	router.Use(middleware.BaseURLMiddleware())

	utilRepository := utilsdb.NewRepository(client, logger)

	authRouterManager := router.Group(authURL)
	authRepository := authdb.NewRepository(client, logger)
	authRouterHandler := auth.NewHandler(logger, authRepository)
	authRouterHandler.Register(authRouterManager)

	provinceRouterManager := router.Group(provinceURL)
	provinceRepository := provincedb.NewRepository(client, logger)
	provinceRouterHandler := province.NewHandler(logger, provinceRepository, utilRepository, client)
	provinceRouterHandler.Register(provinceRouterManager)

	subcategoryRouterManager := router.Group(subcategoryURL)
	subcategoryRepository := subcategorydb.NewRepository(client, logger)
	subcategoryRouterHandler := subcategory.NewHandler(logger, subcategoryRepository, utilRepository, client)
	subcategoryRouterHandler.Register(subcategoryRouterManager)

	userRouterManager := router.Group(userAdminURL)
	userRepository := userdb.NewRepository(client, logger)
	userRouterHandler := user.NewHandler(logger, userRepository, utilRepository, client)
	userRouterHandler.Register(userRouterManager)

	smsSocketRouter := router.Group(smsSocketURL)
	smsSocket := sms_socket.NewHandler(logger, smsSender)
	smsSocket.Register(smsSocketRouter)

	itemCategoryRouterManager := router.Group(itemCategoryURL)
	itemCategoryRepository := itemCategorydb.NewRepository(client, logger)
	itemCategoryRouterHandler := itemCategory.NewHandler(logger, itemCategoryRepository, utilRepository, client)
	itemCategoryRouterHandler.Register(itemCategoryRouterManager)

	itemRouterManager := router.Group(itemURL)
	itemRepository := itemdb.NewRepository(client, logger)
	itemRouterHandler := item.NewHandler(logger, itemRepository, utilRepository, client)
	itemRouterHandler.Register(itemRouterManager)

	businessesRouterManager := router.Group(businessesURL)
	businessesRepository := businessesdb.NewRepository(client, logger)
	businessesRouterHandler := businesses.NewHandler(logger, businessesRepository, itemRepository, utilRepository, client)
	businessesRouterHandler.Register(businessesRouterManager)

	userClientRouterManager := router.Group(userURL)
	userClientRepository := userClientdb.NewRepository(client, logger)
	userClientRouterHandler := userClient.NewHandler(client, logger, userClientRepository, utilRepository, smsSender)
	userClientRouterHandler.Register(userClientRouterManager)

	offerRouterManager := router.Group(offerURL)
	offerRepository := offerdb.NewRepository(client, logger)
	offerRouterHandler := offer.NewHandler(logger, offerRepository, utilRepository, client)
	offerRouterHandler.Register(offerRouterManager)

	reservationManager := router.Group(reservationURL)
	reservationRepository := reservationdb.NewRepository(client, logger)
	//reservationRepositoryWS := reservationdbWS.NewRepository(client, logger)
	reservationRouterHandler := reservation.NewHandler(logger, reservationRepository, utilRepository, fcmClient, client)
	reservationRouterHandler.Register(reservationManager)

	notificationManager := router.Group(notificationURL)
	notificationRepository := notificationdb.NewRepository(client, logger)
	notificationRouterHandler := notification.NewHandler(logger, notificationRepository, fcmClient, utilRepository, client)
	notificationRouterHandler.Register(notificationManager)

	deviceTokenManager := router.Group(deviceTokenURL)
	deviceTokenRepository := deviceTokendb.NewRepository(client, logger)
	deviceTokenRouterHandler := deviceToken.NewHandler(logger, deviceTokenRepository)
	deviceTokenRouterHandler.Register(deviceTokenManager)

	basketManager := router.Group(basketURL)
	basketRepository := basketdb.NewRepository(client, logger)
	basketHandler := basket.NewHandler(logger, basketRepository, utilRepository, client)
	basketHandler.Register(basketManager)

	orderManagerClient := router.Group(orderClientURL)
	orderRepositoryClient := orderClientdb.NewRepository(client, logger, basketRepository)
	orderRepositoryWS := orderClientWS.NewRepository(client, logger)
	orderHandlerClient := orderClient.NewHandler(logger, orderRepositoryClient, orderRepositoryWS, utilRepository, client)
	orderHandlerClient.Register(orderManagerClient)

	categoryManagerClient := router.Group(categoryURL)
	categoryRepositoryClient := categorydb.NewRepository(client, logger)
	categoryHandlerClient := category.NewHandler(logger, categoryRepositoryClient, utilRepository, client)
	categoryHandlerClient.Register(categoryManagerClient)

	return router
}
