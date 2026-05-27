package clients

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"restaurants/internal/appresult"
	"restaurants/internal/handlers"
	"restaurants/internal/middleware"
	"restaurants/pkg/logging"
	"restaurants/pkg/sms_sender"
	"restaurants/pkg/utils"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v4/pgxpool"
)

const (
	registerURL = "/registration"
	checkOTP    = "/checkOTP"
	loginURL    = "/login"
	profile     = "/profile"
	logout      = "/logout"
	clientURL   = ""
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
	router.POST(profile, middleware.JwtTokenCheck(h.client), h.createProfile)
	router.GET(profile, middleware.JwtTokenCheck(h.client), h.getProfile)
	router.PUT(clientURL, middleware.JwtTokenCheck(h.client), h.update)
	router.POST(logout, middleware.JwtTokenCheck(h.client), h.logout)
}

func (h *handler) register(c *gin.Context) {
	var (
		register RegisterDTO
	)
	if err := c.ShouldBindJSON(&register); err != nil {
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
		c.JSON(http.StatusInternalServerError,
			appresult.NewAppError(err, "failed to send sms", "500"))
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

	resp, err := h.repository.CheckOTP(context.TODO(), otp)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	token, err := utils.GenerateTokenPair(resp.UserId)
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

	resp, err := h.repository.Login(context.TODO(), login)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	token, err := utils.GenerateTokenPair(resp.UserId)
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
		imagePath string
	)
	userId, err := utils.ExtractUserIdFromToken(c, h.client)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	jsonData := c.PostForm("data")
	if err := json.Unmarshal([]byte(jsonData), &user); err != nil {
		appresult.HandleError(c, err)
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
		user      UserUpdateDTO
		imagePath string
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
