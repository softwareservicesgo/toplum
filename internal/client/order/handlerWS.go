package order

import (
	"context"
	"encoding/json"
	"net/http"
	"restaurants/internal/appresult"
	"restaurants/pkg/utils"
	"strconv"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var (
	upgrader       = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	businessesConn = make(map[int]map[int]OrderBusiness)
	clientConn     = make(map[int]*ClientWS)
	orderOneConn   = make(map[int]map[ConnKey]OrderOneWS)
	orderWSMutex   sync.Mutex
)

func (h *handler) checkBusinesses(c *gin.Context) {
	businessesId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		appresult.HandleError(c, err)
		return
	}
	userId, err := utils.ExtractUserIdFromToken(c, h.client)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}
	
	err = h.WSRepository.CheckBusinesses(context.TODO(), businessesId, userId)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	c.File("./html_for_test/order_businesses.html")
}

func (h *handler) wsHandlerBusinesses(c *gin.Context) {
	businessesId, err := strconv.Atoi(c.Param("businesses_id"))
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	userId, err := utils.ExtractUserIdFromToken(c, h.client)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	baseURL := c.MustGet("baseURL").(string)

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	orderWSMutex.Lock()
	if _, exists := businessesConn[businessesId]; !exists {
		businessesConn[businessesId] = make(map[int]OrderBusiness)
	}
	businessesConn[businessesId][userId] = OrderBusiness{
		Conn:    conn,
		Filter:  &OrderFilter{},
		BaseURL: baseURL,
	}
	orderWSMutex.Unlock()

	defer func() {
		orderWSMutex.Lock()
		delete(businessesConn[businessesId], userId)
		if len(businessesConn[businessesId]) == 0 {
			delete(businessesConn, businessesId)
		}
		orderWSMutex.Unlock()
	}()

	for {
		var filter OrderFilter
		_, msg, err := conn.ReadMessage()
		if err != nil {
			break
		}

		if err := json.Unmarshal(msg, &filter); err != nil {
			continue
		}

		orderWSMutex.Lock()
		if bc, ok := businessesConn[businessesId][userId]; ok {
			bc.Filter = &filter
			businessesConn[businessesId][userId] = bc
		}
		orderWSMutex.Unlock()

		SendOrdersBusinesses(h.repository, userId, businessesId)
	}
}

func (h *handler) checkClient(c *gin.Context) {
	clientId, err := utils.ExtractUserIdFromToken(c, h.client)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	err = h.WSRepository.CheckClient(context.TODO(), clientId)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	c.File("./html_for_test/order_client.html")
}

func (h *handler) wsHandlerClient(c *gin.Context) {
	clientId, err := utils.ExtractUserIdFromToken(c, h.client)
	if err != nil{
		appresult.HandleError(c, err)
		return
	}
	
	baseURL := c.MustGet("baseURL").(string)

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	orderWSMutex.Lock()
	clientConn[clientId] = &ClientWS{
		Conn: conn,
		Filter: &OrderClientFilter{},
		BaseURL: baseURL,
	}
	orderWSMutex.Unlock()

	defer func() {
		orderWSMutex.Lock()
		delete(clientConn, clientId)
		orderWSMutex.Unlock()
	}()

	SendOrdersClient(h.repository, clientId)

	for {
		var filter OrderClientFilter
		_, msg, err := conn.ReadMessage()
		if err != nil {
			break
		}

		if err := json.Unmarshal(msg, &filter); err != nil {
			continue
		}

		orderWSMutex.Lock()
		if cws, ok := clientConn[clientId]; ok {
			cws.Filter = &filter
		}
		orderWSMutex.Unlock()
		SendOrdersClient(h.repository, clientId)
	}
}

func (h *handler) checkOrder(c *gin.Context) {
	orderId, err := strconv.Atoi(c.Param("id"))

	if err != nil {
		appresult.HandleError(c, err)
			return
	}

	err = h.WSRepository.CheckOrder(context.TODO(), orderId)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	c.File("./html_for_test/order_one.html")
}

func (h *handler) wsHandlerOrderOne(c *gin.Context) {
    orderId, _ := strconv.Atoi(c.Param("id"))
    baseURL := c.MustGet("baseURL").(string)

    role := c.Query("role")
    if role == "" {
        role = "client"
    }

    var key ConnKey

    if role == "businesses" {
        userId, err := utils.ExtractUserIdFromToken(c, h.client)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}
        key = ConnKey{ID: userId, Role: "businesses"}
    } else {
        clientId, err := utils.ExtractUserIdFromToken(c, h.client)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}
        key = ConnKey{ID: clientId, Role: "client"}
    }

    conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
    if err != nil {
        return
    }
    defer conn.Close()

    orderWSMutex.Lock()
    if _, exists := orderOneConn[orderId]; !exists {
        orderOneConn[orderId] = make(map[ConnKey]OrderOneWS)
    }

    orderOneConn[orderId][key] = OrderOneWS{
        Conn:    conn,
        BaseURL: baseURL,
    }
    orderWSMutex.Unlock()

    defer func() {
        orderWSMutex.Lock()
        if ow, ok := orderOneConn[orderId][key]; ok && ow.Conn == conn {
            delete(orderOneConn[orderId], key)
            if len(orderOneConn[orderId]) == 0 {
                delete(orderOneConn, orderId)
            }
        }
        orderWSMutex.Unlock()
    }()

    SendOrderOne(h.repository, orderId)

    for {
        _, _, err := conn.ReadMessage()
        if err != nil {
            break
        }
	SendOrderOne(h.repository, orderId)
    }
}

func SendOrdersBusinesses(repo Repository, userId, businessesId int) {
	orderWSMutex.Lock()
	bc, exists := businessesConn[businessesId][userId]
	orderWSMutex.Unlock()

	if !exists || bc.Conn == nil {
		return
	}

	orders, err := repo.GetAllForBusinesses(context.TODO(), businessesId, userId, bc.Filter.Limit, bc.Filter.Offset, bc.Filter.Status, bc.BaseURL)
	if err != nil {
		return
	}

	msg, _ := json.Marshal(orders)
	_ = bc.Conn.WriteMessage(websocket.TextMessage, msg)
}

func SendOrdersClient(repo Repository, clientId int) {
	orderWSMutex.Lock()
	cws, exists := clientConn[clientId]
	orderWSMutex.Unlock()

	if !exists || cws.Conn == nil {
		return
	}
	
	orders, err := repo.GetAllForClient(context.TODO(), clientId, cws.Filter.Limit, cws.Filter.Offset, cws.Filter.Status, cws.Filter.Search, cws.BaseURL)
	if err != nil {
		return
	}

	msg, _ := json.Marshal(orders)
	_ = cws.Conn.WriteMessage(websocket.TextMessage, msg)
}

func SendOrderOne(repo Repository, orderId int) {
    orderWSMutex.Lock()
    users, exists := orderOneConn[orderId]
    if !exists || len(users) == 0 {
        orderWSMutex.Unlock()
        return
    }
    var baseURL string
    for _, ow := range users {
        baseURL = ow.BaseURL
        break
    }
    orderWSMutex.Unlock()

    order, err := repo.GetOne(context.TODO(), orderId, baseURL)
    if err != nil {
        return
    }

    msg, _ := json.Marshal(order)

    orderWSMutex.Lock()
    for _, ow := range users {
        if ow.Conn != nil {
            _ = ow.Conn.WriteMessage(websocket.TextMessage, msg)
        }
    }
    orderWSMutex.Unlock()
}

func NotifyOrderUpdate(businessesId, clientId, orderId int, repo Repository) {
	if clients, exists := businessesConn[businessesId]; exists {
		for userId := range clients {
			SendOrdersBusinesses(repo, userId, businessesId)
		}
	}

	if clientId != 0 {
		SendOrdersClient(repo, clientId)
	}

	  if orderId != 0 {
        SendOrderOne(repo, orderId)
    }
}
