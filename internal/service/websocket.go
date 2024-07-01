package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/bitxeno/atvloadly/internal/log"
	"github.com/bitxeno/atvloadly/internal/manager"
	"github.com/bitxeno/atvloadly/internal/model"
	"github.com/gofiber/contrib/websocket"
)

func HandleInstallMessage(c *websocket.Conn) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	installMgr := manager.NewInteractiveInstallManager()
	installMgr.OnOutput(func(line string) {
		_ = c.WriteMessage(websocket.TextMessage, []byte(line))
	})
	defer installMgr.Close()

	for {
		var msg model.Message
		if err := c.ReadJSON(&msg); err != nil {
			// websocket client close
			if websocket.IsUnexpectedCloseError(err) || websocket.IsCloseError(err) {
				return
			}
			log.Err(err).Msg("Read websocket message error: ")
			return
		}
		var data string
		if err := json.Unmarshal(msg.Data, &data); err != nil {
			msg := fmt.Sprintf("ERROR: %s", err.Error())
			_ = c.WriteMessage(websocket.TextMessage, []byte(msg))
			return
		}

		switch msg.Type {
		case model.MessageTypeInstall:
			var v model.InstalledApp
			err := json.Unmarshal([]byte(data), &v)
			if err != nil {
				msg := fmt.Sprintf("ERROR: %s", err.Error())
				_ = c.WriteMessage(websocket.TextMessage, []byte(msg))
				continue
			}

			if v.Account == "" || v.Password == "" || v.UDID == "" {
				_ = c.WriteMessage(websocket.TextMessage, []byte("account or password or UDID is empty"))
				continue
			}

			go runInstallMessage(ctx, c, installMgr, v)
		case model.MessageType2FA:
			code := data
			installMgr.Write([]byte(code + "\n"))
		default:
			_ = c.WriteMessage(websocket.TextMessage, []byte("ERROR: invalid message type"))
			continue
		}
	}
}

func runInstallMessage(ctx context.Context, c *websocket.Conn, installMgr *manager.InstallManager, v model.InstalledApp) {
	err := installMgr.Start(ctx, v.UDID, v.Account, v.Password, v.IpaPath)
	if err != nil {
		msg := fmt.Sprintf("ERROR: %s", err.Error())
		_ = c.WriteMessage(websocket.TextMessage, []byte(msg))
		return
	}
	log.Infof("install exit: %s", v.IpaPath)
}

func HandlePairMessage(c *websocket.Conn) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	pairMgr := manager.NewPairManager()
	pairMgr.OnOutput(func(line string) {
		_ = c.WriteMessage(websocket.TextMessage, []byte(line))
	})
	defer pairMgr.Close()

	for {
		var msg model.Message
		if err := c.ReadJSON(&msg); err != nil {
			// websocket client close
			if websocket.IsUnexpectedCloseError(err) || websocket.IsCloseError(err) {
				return
			}
			log.Err(err).Msg("Read websocket message error: ")
			return
		}
		var data string
		if err := json.Unmarshal(msg.Data, &data); err != nil {
			msg := fmt.Sprintf("ERROR: %s", err.Error())
			_ = c.WriteMessage(websocket.TextMessage, []byte(msg))
			return
		}

		switch msg.Type {
		case model.MessageTypePair:
			udid := data
			go runPairMessage(ctx, c, pairMgr, udid)
		case model.MessageTypePairConfirm:
			code := data
			pairMgr.Write([]byte(code + "\n"))
		default:
			_ = c.WriteMessage(websocket.TextMessage, []byte("ERROR: invalid message type"))
			continue
		}
	}
}

func runPairMessage(ctx context.Context, c *websocket.Conn, pairMgr *manager.PairManager, udid string) {
	err := pairMgr.Start(ctx, udid)
	if err != nil {
		msg := fmt.Sprintf("ERROR: %s", err.Error())
		_ = c.WriteMessage(websocket.TextMessage, []byte(msg))
		return
	}
}
