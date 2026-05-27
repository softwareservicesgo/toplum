package reservation

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"restaurants/internal/appresult"
	"restaurants/pkg/utils"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var (
	upgrader       = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	restaurantConn = make(map[int]map[int]Restaurants)
	clientConn     = make(map[int]*ClientWS)

	availabilityDayConn  = make(map[int]map[int]AvailabilityDayClient)
	availabilityTimeConn = make(map[int]map[int]AvailabilityTimeClient)
	clientsMutex         sync.Mutex
)

func (h *handler) reservations(c *gin.Context) {
	restaurantId, err := strconv.Atoi(c.Param("restaurant_id"))
	if err != nil {
		appresult.HandleError(c, err)
		return
	}
	clientId, err := utils.ExtractUserIdFromToken(c, h.client)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	err = h.WSRepository.CheckRestaurant(context.TODO(), restaurantId, clientId)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	c.File("./manager.html")
}

func (h *handler) wsHandler(c *gin.Context) {
	restaurantId, _ := strconv.Atoi(c.Param("restaurant_id"))
	
	userId, err := utils.ExtractUserIdFromToken(c, h.client)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	baseURL := c.MustGet("baseURL").(string)

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		fmt.Println("upgrade error:", err)
		return
	}
	defer conn.Close()

	clientsMutex.Lock()
	if _, exists := restaurantConn[restaurantId]; !exists {
		restaurantConn[restaurantId] = make(map[int]Restaurants)
	}

	restaurantConn[restaurantId][userId] = Restaurants{
		Conn:    conn,
		Filter:  nil,
		BaseURL: baseURL,
	}
	clientsMutex.Unlock()

	defer func() {
		clientsMutex.Lock()
		delete(restaurantConn[restaurantId], userId)
		if len(restaurantConn[restaurantId]) == 0 {
			delete(restaurantConn, restaurantId)
		}
		clientsMutex.Unlock()
		fmt.Printf("user %d disconnected from restaurant %d\n", userId, restaurantId)
	}()

	for {
		var filter ReservationFilter
		_, msg, err := conn.ReadMessage()
		if err != nil {
			break
		}

		if err := json.Unmarshal(msg, &filter); err != nil {
			conn.WriteMessage(websocket.TextMessage, []byte(`{"error":"invalid request"}`))
			continue
		}

		clientsMutex.Lock()
		r := restaurantConn[restaurantId][userId]
		r.Filter = &filter
		restaurantConn[restaurantId][userId] = r
		clientsMutex.Unlock()

		SendReservation(h.Repository, restaurantId)
	}
}

func (h *handler) wsHandlerClient(c *gin.Context) {
	clientId, err := utils.ExtractUserIdFromToken(c, h.client)
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

	clientsMutex.Lock()
	clientConn[clientId] = &ClientWS{
		Conn: conn,
		Filter: &ReservationClientFilter{
			Limit:  "10",
			Offset: "0",
		},
		BaseURL: baseURL,
	}
	clientsMutex.Unlock()

	defer func() {
		clientsMutex.Lock()
		delete(clientConn, clientId)
		clientsMutex.Unlock()
	}()

	SendReservationsClient(h.Repository, clientId)

	for {
		var filter ReservationClientFilter
		_, msg, err := conn.ReadMessage()
		if err != nil {
			break
		}

		if err := json.Unmarshal(msg, &filter); err != nil {
			continue
		}

		clientsMutex.Lock()
		if cws, ok := clientConn[clientId]; ok {
			cws.Filter = &filter
		}
		clientsMutex.Unlock()

		SendReservationsClient(h.Repository, clientId)
	}
}

func (h *handler) availabilityDay(c *gin.Context) {
	var (
		availabilityDay AvailabilityDayReq
	)
	restaurantId, _ := strconv.Atoi(c.Param("restaurant_id"))

	clientId, err := utils.ExtractUserIdFromToken(c, h.client)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		fmt.Println("WS upgrade error:", err)
		return
	}

	clientsMutex.Lock()
	if _, exists := availabilityDayConn[restaurantId]; !exists {
		availabilityDayConn[restaurantId] = make(map[int]AvailabilityDayClient)
	}
	availabilityDayConn[restaurantId][clientId] = AvailabilityDayClient{
		Conn:        conn,
		PersonCount: 0,
	}
	clientsMutex.Unlock()

	defer func() {
		clientsMutex.Lock()
		delete(availabilityDayConn[restaurantId], clientId)
		if len(availabilityDayConn[restaurantId]) == 0 {
			delete(availabilityDayConn, restaurantId)
		}
		clientsMutex.Unlock()
		conn.Close()
	}()

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			break
		}

		if err := json.Unmarshal(msg, &availabilityDay); err != nil {
			conn.WriteMessage(websocket.TextMessage, []byte(`{"error":"invalid request"}`))
			continue
		}

		clientsMutex.Lock()
		availabilityDayConn[restaurantId][clientId] = AvailabilityDayClient{
			Conn:        conn,
			PersonCount: availabilityDay.PersonCount,
		}
		clientsMutex.Unlock()

		today := time.Now()
		endDate := today.AddDate(0, 1, 0)
		availabilityDay, _ := h.WSRepository.AvailabilityDay(context.TODO(), restaurantId, availabilityDay, today, endDate)

		resp, _ := json.Marshal(availabilityDay)
		conn.WriteMessage(websocket.TextMessage, resp)
	}
}

func (h *handler) availabilityTime(c *gin.Context) {
	var (
		availabilityTime AvailabilityTimeReq
	)
	restaurantId, _ := strconv.Atoi(c.Param("restaurant_id"))
	
	clientId, err := utils.ExtractUserIdFromToken(c, h.client)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		fmt.Println("WS upgrade error:", err)
		return
	}

	clientsMutex.Lock()
	if _, exists := availabilityTimeConn[restaurantId]; !exists {
		availabilityTimeConn[restaurantId] = make(map[int]AvailabilityTimeClient)
	}
	availabilityTimeConn[restaurantId][clientId] = AvailabilityTimeClient{
		Conn:        conn,
		PersonCount: 0,
		Day:         time.Time{},
	}
	clientsMutex.Unlock()

	defer func() {
		clientsMutex.Lock()
		delete(availabilityTimeConn[restaurantId], clientId)
		if len(availabilityTimeConn[restaurantId]) == 0 {
			delete(availabilityTimeConn, restaurantId)
		}
		clientsMutex.Unlock()
		conn.Close()
	}()

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			break
		}

		if err := json.Unmarshal(msg, &availabilityTime); err != nil {
			conn.WriteMessage(websocket.TextMessage, []byte(`{"error":"invalid request"}`))
			continue
		}

		clientsMutex.Lock()
		availabilityTimeConn[restaurantId][clientId] = AvailabilityTimeClient{
			Conn:        conn,
			PersonCount: availabilityTime.PersonCount,
			Day:         availabilityTime.Day,
		}
		clientsMutex.Unlock()

		availabilityTime, _, err := h.WSRepository.AvailabilityTimes(context.TODO(), restaurantId, availabilityTime)
		if err != nil {
			appresult.HandleError(c, err)
			return
		}
		resp, _ := json.Marshal(availabilityTime)
		conn.WriteMessage(websocket.TextMessage, resp)
	}
}

func SendAvailabilityDay(r RepositoryWS, restaurantId int) {
	today := time.Now()
	endDate := today.AddDate(0, 1, 0)

	if clients, exists := availabilityDayConn[restaurantId]; exists {
		for _, availability := range clients {

			availabilityDay, err := r.AvailabilityDay(
				context.TODO(), restaurantId, AvailabilityDayReq{PersonCount: availability.PersonCount}, today, endDate)
			if err != nil {
				return
			}

			msg, _ := json.Marshal(availabilityDay)
			availability.Conn.WriteMessage(websocket.TextMessage, msg)
		}
	}
}

func SendAvailabilityTime(r RepositoryWS, restaurantId int) {
	if clients, exists := availabilityTimeConn[restaurantId]; exists {
		for _, availability := range clients {
			availabilityTimeReq := AvailabilityTimeReq{
				PersonCount: availability.PersonCount,
				Day:         availability.Day,
			}

			availabilityTime, _, err := r.AvailabilityTimes(
				context.TODO(), restaurantId, availabilityTimeReq)
			if err != nil {
				return
			}

			msg, _ := json.Marshal(availabilityTime)
			availability.Conn.WriteMessage(websocket.TextMessage, msg)
		}
	}
}

func SendReservation(repo Repository, restaurantId int) {
	if clients, exists := restaurantConn[restaurantId]; exists {
		for clientId, r := range clients {
			if r.Conn == nil {
				continue
			}

			allReservationsMsg, err := repo.GetByRestaurantId(context.TODO(), restaurantId, *r.Filter, r.BaseURL)
			if err != nil {
				return
			}
			msg, _ := json.Marshal(allReservationsMsg)

			err = r.Conn.WriteMessage(websocket.TextMessage, msg)
			if err != nil {
				fmt.Printf("error sending to client %d (restaurant %d): %v\n", clientId, restaurantId, err)
			}
		}
	}
}

func SendReservationsClient(repo Repository, clientId int) {
	clientsMutex.Lock()
	cws, exists := clientConn[clientId]
	clientsMutex.Unlock()

	if !exists || cws.Conn == nil {
		return
	}
	res, err := repo.GetByClientId(
		context.TODO(),
		clientId,
		*cws.Filter,
		cws.BaseURL,
	)
	if err != nil {
		return
	}

	msg, _ := json.Marshal(res)
	_ = cws.Conn.WriteMessage(websocket.TextMessage, msg)
}

func NotifyRestaurantUpdate(
	ctx context.Context,
	restaurantID int,
	clientId int,
	r Repository,
	rWS RepositoryWS,
) error {
	SendReservation(r, restaurantID)
	SendAvailabilityDay(rWS, restaurantID)
	SendAvailabilityTime(rWS, restaurantID)
	if clientId != 0 {
		SendReservationsClient(r, clientId)
	}

	return nil
}
