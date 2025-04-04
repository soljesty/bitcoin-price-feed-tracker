package handlers

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/ZiyadBouazara/bitcoin-pulse/stockservice-go/internal/core/domain"
	"github.com/ZiyadBouazara/bitcoin-pulse/stockservice-go/internal/core/ports"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

const (
	maxMessageSize = 2048
	writeWait      = 10 // seconds
	pongWait       = 60 // seconds
	pingPeriod     = (pongWait * 9) / 10
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     checkOrigin,
}

type LivePricesHandler struct {
	priceService ports.PriceService
	logger       *zap.Logger
	mu           sync.Mutex
	clients      map[ports.WebSocketConn]bool
}

func NewLivePricesHandler(ps ports.PriceService, logger *zap.Logger) *LivePricesHandler {
	return &LivePricesHandler{
		priceService: ps,
		logger:       logger,
		clients:      make(map[ports.WebSocketConn]bool),
	}
}

func checkOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	// TODO: Implement proper origin validation against allowed domains
	return origin == "https://yourdomain.com" // Example
}

func (h *LivePricesHandler) HandleWebSocket(ctx *gin.Context) {
	ws, err := upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		h.logger.Error("WebSocket upgrade failed", zap.Error(err))
		return
	}

	conn := ws.(ports.WebSocketConn)
	h.mu.Lock()
	h.clients[conn] = true
	h.mu.Unlock()

	go h.manageConnection(ctx, conn)
}

func (h *LivePricesHandler) manageConnection(ctx *gin.Context, conn ports.WebSocketConn) {
	defer func() {
		h.cleanupConnection(conn)
	}()

	conn.SetReadLimit(maxMessageSize)
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(pongWait * time.Second))
		return nil
	})

	h.logger.Info("New client connected", zap.String("remote", conn.RemoteAddr().String()))
	h.priceService.AddClient(conn)

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				h.logger.Error("Unexpected WebSocket closure", zap.Error(err))
			}
			break
		}

		if err := h.handleClientMessage(conn, message); err != nil {
			h.logger.Error("Message handling failed", zap.Error(err))
			break
		}
	}
}

func (h *LivePricesHandler) handleClientMessage(conn ports.WebSocketConn, message []byte) error {
	var subMsg domain.SubscriptionMessage
	if err := json.Unmarshal(message, &subMsg); err != nil {
		h.sendError(conn, "Invalid message format")
		return fmt.Errorf("invalid message format: %w", err)
	}

	if !domain.IsSupportedStock(string(subMsg.Stock)) {
		h.sendError(conn, "Unsupported stock symbol")
		return fmt.Errorf("unsupported stock: %s", subMsg.Stock)
	}

	switch subMsg.Action {
	case domain.Subscribe:
		return h.priceService.Subscribe(conn, subMsg.Stock)
	case domain.Unsubscribe:
		return h.priceService.Unsubscribe(conn, subMsg.Stock)
	default:
		h.sendError(conn, "Invalid action")
		return fmt.Errorf("unknown action: %s", subMsg.Action)
	}
}

func (h *LivePricesHandler) cleanupConnection(conn ports.WebSocketConn) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.clients[conn]; ok {
		h.priceService.RemoveClient(conn)
		delete(h.clients, conn)
		conn.Close()
		h.logger.Info("Client disconnected", zap.String("remote", conn.RemoteAddr().String()))
	}
}

func (h *LivePricesHandler) sendError(conn ports.WebSocketConn, errorMessage string) {
	errMsg := domain.ErrorMessage{
		Type:    "error",
		Message: errorMessage,
	}

	message, err := json.Marshal(errMsg)
	if err != nil {
		h.logger.Error("Failed to marshal error message", zap.Error(err))
		return
	}

	conn.SetWriteDeadline(time.Now().Add(writeWait * time.Second))
	if err := conn.WriteMessage(websocket.TextMessage, message); err != nil {
		h.logger.Error("Failed to send error message", 
			zap.String("remote", conn.RemoteAddr().String()),
			zap.Error(err))
	}
}
