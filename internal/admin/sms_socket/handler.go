package sms_socket

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"restaurants/internal/appresult"
	"restaurants/internal/handlers"
	"restaurants/pkg/logging"
	"restaurants/pkg/sms_sender"
	"restaurants/pkg/sms_sender/socket_helper"
)

type handler struct {
	logger    *logging.Logger
	smsSender *sms_sender.Client
	upgrader  *websocket.Upgrader
}

func NewHandler(l *logging.Logger, smsSender *sms_sender.Client) handlers.Handler {
	return &handler{
		logger:    l,
		smsSender: smsSender,
		upgrader:  &websocket.Upgrader{},
	}
}

const initSocketUrl = "/init"

func (h *handler) Register(router *gin.RouterGroup) {
	router.GET(initSocketUrl, h.initSocket)
}

const secretSocketKey = "some_secret_key"

func (h *handler) initSocket(c *gin.Context) {
	secret := c.Request.Header.Get("X-Socket-Secret")
	if secret != secretSocketKey {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	socket, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.logger.Error("failed to upgrade socket", err)
		c.JSON(http.StatusInternalServerError, appresult.ErrInternalServer)
		return
	}

	client := socket_helper.NewClient(socket)

	go func(logger *logging.Logger) {
		if err := client.ReadPump(); err != nil {
			logger.Error("SOCKET READ PUMP", err)
		}
	}(h.logger)

	go func(logger *logging.Logger) {
		if err := client.WritePump(); err != nil {
			logger.Error("SOCKET WRITE PUMP", err)
		}
	}(h.logger)

	h.smsSender.RegisterClient(client)
}
