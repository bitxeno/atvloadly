package manager

import (
	"context"

	"github.com/bitxeno/atvloadly/internal/model"
	"github.com/gofiber/contrib/websocket"
)

type WebsocketManager struct {
	ctx    context.Context
	cancel context.CancelFunc
	conn   *websocket.Conn
	chMsg  chan string
}

func NewWebsocketManager(conn *websocket.Conn) *WebsocketManager {
	ctx, cancel := context.WithCancel(context.Background())
	mgr := &WebsocketManager{
		ctx:    ctx,
		cancel: cancel,
		conn:   conn,
		chMsg:  make(chan string, 100),
	}

	go mgr.runWriteMessage()

	return mgr
}

func (mgr *WebsocketManager) runWriteMessage() {
	for {
		select {
		case <-mgr.ctx.Done():
			return
		case msg := <-mgr.chMsg:
			_ = mgr.conn.WriteMessage(websocket.TextMessage, []byte(msg))
		}
	}
}

func (mgr *WebsocketManager) ReadMessage() (*model.Message, error) {
	var msg model.Message
	if err := mgr.conn.ReadJSON(&msg); err != nil {
		return nil, err
	}

	return &msg, nil
}

func (mgr *WebsocketManager) WriteMessage(msg string) {
	mgr.chMsg <- msg
}

func (mgr *WebsocketManager) Cancel() {
	mgr.cancel()
}

func (mgr *WebsocketManager) Context() context.Context {
	return mgr.ctx
}
