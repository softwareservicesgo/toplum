package user

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"restaurants/internal/appresult"
	"restaurants/internal/enum"
	"restaurants/internal/handlers"
	"restaurants/internal/middleware"
	"restaurants/pkg/logging"
	"restaurants/pkg/sms_sender"
	"restaurants/pkg/utils"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v4/pgxpool"
)

const (
	registerURL = "/registration"
	checkOTP    = "/checkOTP"
	loginURL    = "/login"
	profileURL  = "/profile"
	logoutURL   = "/logout"
	searchURL   = "/search"
	URL         = ""
	byIdURL     = "/:id"
)

type handler struct {
	logger         *logging.Logger
	repository     Repository
	utilRepository utils.Repository
	smsSender      *sms_sender.Client
	client         *pgxpool.Pool
}

func NewHandler(client *pgxpool.Pool, logger *logging.Logger, repository Repository,
	utilRepository utils.Repository, smsSender *sms_sender.Client) handlers.Handler {
	return &handler{
		logger:         logger,
		repository:     repository,
		utilRepository: utilRepository,
		smsSender:      smsSender,
		client:         client,
	}
}

func (h *handler) Register(router *gin.RouterGroup) {
	router.POST(registerURL, h.register)
	router.POST(checkOTP, h.checkOTP)
	router.POST(loginURL, h.login)
	router.POST(profileURL, middleware.JwtTokenCheck(h.client), h.createProfile)
	router.GET(profileURL, middleware.JwtTokenCheck(h.client), h.getProfile)
	router.PUT(URL, middleware.JwtTokenCheck(h.client), h.update)
	router.POST(logoutURL, middleware.JwtTokenCheck(h.client), h.logout)
	router.GET(searchURL, middleware.JwtTokenCheck(h.client), h.searchUsers)
	router.PUT(byIdURL, middleware.JwtTokenCheck(h.client), h.updateStatusBusinessesRole)
	router.GET(byIdURL, middleware.JwtTokenCheck(h.client), h.getUsers)
}

func (h *handler) register(c *gin.Context) {
	var (
		register RegisterDTO
	)
	if err := c.ShouldBindJSON(&register); err != nil {
		appresult.HandleError(c, err)
		return
	}

	err := utils.ValidatePhoneNumber(register.PhoneNumber)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	randomNumber, err := h.repository.Register(c, register)
	if err != nil {
		log.Println("[ERROR]", "failed to register user:", err)
		appresult.HandleError(c, err)
		return
	}

	if err := h.smsSender.SendOtp(register.PhoneNumber, randomNumber); err != nil {
		h.logger.Errorln("failed to send otp:", err, "; phone:", register.PhoneNumber)
		// c.JSON(http.StatusInternalServerError,
		// 	appresult.NewAppError(err, "failed to send sms", "500"))
	}

	c.JSON(http.StatusOK, map[string]interface{}{
		"otp": randomNumber,
	})
}

func (h *handler) checkOTP(c *gin.Context) {
	var (
		otp CheckOTP
	)

	if err := c.ShouldBindJSON(&otp); err != nil {
		appresult.HandleError(c, err)
		return
	}

	err := utils.ValidatePhoneNumber(otp.PhoneNumber)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	resp, userId, err := h.repository.CheckOTP(context.TODO(), otp)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	token, err := utils.GenerateTokenPair(*userId)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	resp.Token = token
	c.JSON(http.StatusOK, resp)
}

func (h *handler) login(c *gin.Context) {
	var (
		login LoginDTO
	)

	if err := c.ShouldBindJSON(&login); err != nil {
		fmt.Println("error binding JSON:", err)
		appresult.HandleError(c, err)
		return
	}

	resp, userId, err := h.repository.Login(context.TODO(), login)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	token, err := utils.GenerateTokenPair(*userId)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	resp.Token = token
	c.JSON(http.StatusOK, resp)
}

func (h *handler) createProfile(c *gin.Context) {
	var (
		user      UserReqDTO
		imagePath *string
	)
	userId, err := utils.ExtractUserIdFromToken(c, h.client)
	if err != nil {
		fmt.Println("error: ", err)
		appresult.HandleError(c, err)
		return
	}

	jsonData := c.PostForm("data")
	if err := json.Unmarshal([]byte(jsonData), &user); err != nil {
		fmt.Println("error: ", err)
		appresult.HandleError(c, err)
		return
	}
	if user.Name == "" || user.ProvinceId == 0 || user.Password == "" {
		c.JSON(http.StatusInternalServerError, appresult.ErrRequiredData)
		return
	}
	if len(user.Password) < 8 || 50 < len(user.Password) {
		c.JSON(http.StatusInternalServerError, appresult.ErrPasswordLength)
		return
	}

	uploadDir := filepath.Join("uploads/user")
	if err := os.MkdirAll(uploadDir, os.ModePerm); err != nil {
		appresult.HandleError(c, err)
		return
	}

	image, err := c.FormFile("image")
	if err == nil {
		imagePath, err = utils.SaveUploadedFile(c, image, uploadDir)
		if err != nil {
			fmt.Println("error: ", err)
			appresult.HandleError(c, err)
			return
		}
	}

	hashPassword, err := utils.HashPassword(user.Password)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	baseURL := c.MustGet("baseURL").(string)

	resp, err := h.repository.CreateProfile(context.TODO(), userId, user, imagePath, hashPassword, baseURL)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	c.JSON(http.StatusCreated, resp)
}

func (h *handler) getProfile(c *gin.Context) {
	userId, err := utils.ExtractUserIdFromToken(c, h.client)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	baseURL := c.MustGet("baseURL").(string)

	resp, err := h.repository.GetProfile(context.TODO(), userId, baseURL)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	c.JSON(http.StatusCreated, resp)
}

func (h *handler) update(c *gin.Context) {
	var (
		user      UserReqDTO
		imagePath *string
	)
	userId, err := utils.ExtractUserIdFromToken(c, h.client)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	jsonData := c.PostForm("data")
	if jsonData != "" {
		if err := json.Unmarshal([]byte(jsonData), &user); err != nil {
			appresult.HandleError(c, err)
			return
		}
	}

	if user.Name == "" || user.ProvinceId == 0 || user.Password == "" {
		c.JSON(http.StatusInternalServerError, appresult.ErrRequiredData)
		return
	}
	if len(user.Password) < 8 || 50 < len(user.Password) {
		c.JSON(http.StatusInternalServerError, appresult.ErrPasswordLength)
		return
	}

	uploadDir := filepath.Join("uploads/user")
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

	hashPassword, err := utils.HashPassword(user.Password)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	baseURL := c.MustGet("baseURL").(string)

	resp, err := h.repository.UpdateProfile(context.TODO(), userId, user, imagePath, hashPassword, baseURL)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *handler) logout(c *gin.Context) {
	_, err := utils.ExtractUserIdFromToken(c, h.client)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	token := c.GetHeader("Authorization")
	token = token[7:]

	err = h.repository.Logout(context.TODO(), token)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "success!!!",
	})
}

func (h *handler) searchUsers(c *gin.Context) {

	name := c.Query("name")
	phoneNumber := c.Query("phone_number")
	limit := c.Query("limit")
	offset := c.Query("offset")

	baseURL := c.MustGet("baseURL").(string)

	resp, err := h.repository.SearchUsers(context.TODO(), name, phoneNumber, limit, offset, baseURL)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *handler) updateStatusBusinessesRole(c *gin.Context) {
	var (
		request UpdateBusinessesRoleStatusReq
	)

	userBusinessesId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	userId, err := utils.ExtractUserIdFromToken(c, h.client)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		appresult.HandleError(c, err)
		return
	}

	if !enum.IsValidStatusBusinessesRole(request.Status) {
		appresult.HandleError(c, appresult.ErrStatus)
		return
	}

	err = h.repository.UpdateStatusBusinessesRole(context.TODO(), userId, userBusinessesId, request)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "success!!!",
	})
}

func (h *handler) getUsers(c *gin.Context) {
	var filter UserFilter

	businessesId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	_ = c.ShouldBindQuery(&filter)

	baseURL := c.MustGet("baseURL").(string)

	resp, err := h.repository.GetAllUsers(context.TODO(), businessesId, filter, baseURL)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}
