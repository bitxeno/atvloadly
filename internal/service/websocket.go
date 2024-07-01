package service

import (
	"encoding/json"
	"fmt"

	"github.com/bitxeno/atvloadly/internal/log"
	"github.com/bitxeno/atvloadly/internal/manager"
	"github.com/bitxeno/atvloadly/internal/model"
	"github.com/gofiber/contrib/websocket"
)

func HandleInstallMessage(c *websocket.Conn) {
	websocketMgr := manager.NewWebsocketManager(c)
	defer websocketMgr.Cancel()
	installMgr := manager.NewInteractiveInstallManager()
	installMgr.OnOutput(func(line string) {
		websocketMgr.WriteMessage(line)
	})
	defer installMgr.Close()

	for {
		msg, err := websocketMgr.ReadMessage()
		if err != nil {
			// websocket client close
			if websocket.IsUnexpectedCloseError(err) || websocket.IsCloseError(err) {
				return
			}
			log.Err(err).Msg("Read websocket message error: ")
			return
		}

		switch msg.Type {
		case model.MessageTypeInstall:
			var v model.InstalledApp
			err := json.Unmarshal([]byte(msg.Data), &v)
			if err != nil {
				msg := fmt.Sprintf("ERROR: %s", err.Error())
				_ = c.WriteMessage(websocket.TextMessage, []byte(msg))
				continue
			}

			if v.Account == "" || v.Password == "" || v.UDID == "" {
				_ = c.WriteMessage(websocket.TextMessage, []byte("account or password or UDID is empty"))
				continue
			}

			go runInstallMessage(websocketMgr, installMgr, v)
		case model.MessageType2FA:
			code := msg.Data
			installMgr.Write([]byte(code + "\n"))
		default:
			_ = c.WriteMessage(websocket.TextMessage, []byte("ERROR: invalid message type"))
			continue
		}
	}
}

func runInstallMessage(mgr *manager.WebsocketManager, installMgr *manager.InstallManager, v model.InstalledApp) {
	err := installMgr.Start(mgr.Context(), v.UDID, v.Account, v.Password, v.IpaPath)
	if err != nil {
		msg := fmt.Sprintf("ERROR: %s", err.Error())
		mgr.WriteMessage(msg)
		return
	}
	log.Infof("install exit: %s", v.IpaPath)
}

func HandlePairMessage(c *websocket.Conn) {
	websocketMgr := manager.NewWebsocketManager(c)
	defer websocketMgr.Cancel()
	pairMgr := manager.NewPairManager()
	pairMgr.OnOutput(func(line string) {
		websocketMgr.WriteMessage(line)
	})
	defer pairMgr.Close()

	for {
		msg, err := websocketMgr.ReadMessage()
		if err != nil {
			// websocket client close
			if websocket.IsUnexpectedCloseError(err) || websocket.IsCloseError(err) {
				return
			}
			log.Err(err).Msg("Read websocket message error: ")
			return
		}

		switch msg.Type {
		case model.MessageTypePair:
			udid := msg.Data
			go runPairMessage(websocketMgr, pairMgr, udid)
		case model.MessageTypePairConfirm:
			code := msg.Data
			pairMgr.Write([]byte(code + "\n"))
		default:
			_ = c.WriteMessage(websocket.TextMessage, []byte("ERROR: invalid message type"))
			continue
		}
	}
}

func runPairMessage(mgr *manager.WebsocketManager, pairMgr *manager.PairManager, udid string) {
	err := pairMgr.Start(mgr.Context(), udid)
	if err != nil {
		msg := fmt.Sprintf("ERROR: %s", err.Error())
		mgr.WriteMessage(msg)
		return
	}
}
