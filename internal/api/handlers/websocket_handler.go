package handlers

import (
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type WebSocketHandler struct {
	logger  *zap.Logger
	clients sync.Map
}

func NewWebSocketHandler(logger *zap.Logger) *WebSocketHandler {
	return &WebSocketHandler{logger: logger}
}

type wsClient struct {
	conn      *websocket.Conn
	accountID string
	send      chan []byte
}

func (h *WebSocketHandler) HandleConnection(c *gin.Context) {
	accountID := c.Query("account_id")
	if accountID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "account_id query parameter required"})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.logger.Error("websocket upgrade failed", zap.Error(err))
		return
	}

	client := &wsClient{
		conn:      conn,
		accountID: accountID,
		send:      make(chan []byte, 256),
	}

	h.clients.Store(client, true)
	defer func() {
		h.clients.Delete(client)
		conn.Close()
	}()

	go client.writePump()
	client.readPump(h.logger)
}

func (c *wsClient) readPump(logger *zap.Logger) {
	defer c.conn.Close()
	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logger.Error("websocket read error", zap.Error(err))
			}
			break
		}
	}
}

func (c *wsClient) writePump() {
	defer c.conn.Close()
	for msg := range c.send {
		if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			return
		}
	}
}

func (h *WebSocketHandler) BroadcastToAccount(accountID string, msg []byte) {
	h.clients.Range(func(key, _ interface{}) bool {
		client := key.(*wsClient)
		if client.accountID == accountID {
			select {
			case client.send <- msg:
			default:
				close(client.send)
				h.clients.Delete(client)
			}
		}
		return true
	})
}
