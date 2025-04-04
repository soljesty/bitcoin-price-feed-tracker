package ports

import (
	"context"
	"net"

	"github.com/ZiyadBouazara/bitcoin-pulse/stockservice-go/internal/core/domain"
)

type PriceService interface {
	StartConsuming(ctx context.Context)
	AddClient(ws WebSocketConn)
	RemoveClient(ws WebSocketConn)
	Subscribe(ws WebSocketConn, stock domain.Stock) error
	Unsubscribe(ws WebSocketConn, stock domain.Stock) error
}

type Logger interface {
	Errorf(format string, args ...interface{})
	Info(args ...interface{})
	Infof(format string, args ...interface{})
	Debugf(format string, args ...interface{})
}

type Consumer interface {
	Start(ctx context.Context) error
	SetListener(handlePriceEvent func(event *domain.PriceEvent) error,
	)
}

type PriceEventListener interface {
	OnPriceEvent(event *domain.PriceEvent) error
}

type Notifier interface {
	Broadcast(event *domain.PriceEvent) error
	AddClient(ws WebSocketConn)
	RemoveClient(ws WebSocketConn)
	Subscribe(ws WebSocketConn, stock domain.Stock) error
	Unsubscribe(ws WebSocketConn, stock domain.Stock) error
}

type WebSocketConn interface {
	ReadMessage() (messageType int, p []byte, err error)
	WriteMessage(messageType int, data []byte) error
	Close() error
	RemoteAddr() net.Addr
}
