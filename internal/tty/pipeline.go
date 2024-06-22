package tty

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/bitxeno/atvloadly/internal/tty/lib"
	"github.com/gofiber/contrib/websocket"
	"github.com/runletapp/go-console"
)

// PipeLine Connect websocket and childprocess
type PipeLine struct {
	pty console.Console
	skt *websocket.Conn
}

// NewPipeLine Malloc PipeLine
func NewPipeLine(conn *websocket.Conn, command string) (*PipeLine, error) {
	proc, err := console.New(120, 60)
	if err != nil {
		return nil, err
	}
	err = proc.Start([]string{command})
	if err != nil {
		return nil, err
	}
	return &PipeLine{proc, conn}, nil
}

// ReadSktAndWritePty read skt and write pty
func (w *PipeLine) ReadSktAndWritePty(logChan chan string) {
	for {
		mt, payload, err := w.skt.ReadMessage()
		if err != nil && err != io.EOF {
			logChan <- fmt.Sprintf("Error ReadSktAndWritePty websocket ReadMessage failed: %s", err)
			return
		}
		if mt != websocket.TextMessage {
			logChan <- fmt.Sprintf("Error ReadSktAndWritePty Invalid message type %d", mt)
			return
		}
		var msg lib.Message
		err = json.Unmarshal(payload, &msg)
		if err != nil {
			logChan <- fmt.Sprintf("Error ReadSktAndWritePty Invalid message %s", err)
			return
		}
		switch msg.Type {
		case lib.TypeResize:
			var size []int
			err := json.Unmarshal(msg.Data, &size)
			if err != nil {
				logChan <- fmt.Sprintf("Error ReadSktAndWritePty Invalid resize message: %s", err)
				return
			}
			err = w.pty.SetSize(size[0], size[1])
			if err != nil {
				logChan <- fmt.Sprintf("Error ReadSktAndWritePty pty resize failed: %s", err)
				return
			}
		case lib.TypeData:
			var dat string
			err := json.Unmarshal(msg.Data, &dat)
			if err != nil {
				logChan <- fmt.Sprintf("Error ReadSktAndWritePty Invalid data message %s", err)
				return
			}
			_, err = w.pty.Write([]byte(dat))
			if err != nil {
				logChan <- fmt.Sprintf("Error ReadSktAndWritePty pty write failed: %s", err)
				return
			}
		default:
			logChan <- fmt.Sprintf("Error ReadSktAndWritePty Invalid message type %d", mt)
			return
		}
	}
}

// ReadPtyAndWriteSkt read pty and write skt
func (w *PipeLine) ReadPtyAndWriteSkt(logChan chan string) {
	buf := make([]byte, 4096)
	for {
		n, err := w.pty.Read(buf)
		if err != nil {
			logChan <- fmt.Sprintf("Error ReadPtyAndWriteSkt pty read failed: %s", err)
			return
		}
		err = w.skt.WriteMessage(websocket.TextMessage, buf[:n])
		if err != nil {
			logChan <- fmt.Sprintf("Error ReadPtyAndWriteSkt skt write failed: %s", err)
			return
		}
	}
}

func (w *PipeLine) Close() {
	w.pty.Close()
}
