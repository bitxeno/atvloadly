package service

import (
	"encoding/json"
	"fmt"
	"time"

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

			if v.Account == "" || v.UDID == "" {
				_ = c.WriteMessage(websocket.TextMessage, []byte("account or UDID is empty"))
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
	err := installMgr.Start(mgr.Context(), manager.InstallOptions{
		UDID:             v.UDID,
		Account:          v.Account,
		Password:         v.Password,
		IpaPath:          v.IpaPath,
		RemoveExtensions: v.RemoveExtensions,
		RefreshMode:      false,
	})
	if err != nil {
		installMgr.CleanTempFiles(v.IpaPath)
		msg := fmt.Sprintf("ERROR: %s", err.Error())
		mgr.WriteMessage(msg)
		mgr.WriteMessage("Installation Failed!")
		return
	}

	if installMgr.IsSuccess() {
		now := time.Now()
		expirationDate := now.AddDate(0, 0, 7)
		if installMgr.ProvisioningProfile != nil {
			expirationDate = installMgr.ProvisioningProfile.ExpirationDate.Local()
		}
		v.RefreshedDate = &now
		v.ExpirationDate = &expirationDate
		v.RefreshedResult = true

		app, err := SaveApp(v)
		if err != nil {
			installMgr.CleanTempFiles(v.IpaPath)
			msg := fmt.Sprintf("ERROR: save app to db failed. %s", err.Error())
			mgr.WriteMessage(msg)
			mgr.WriteMessage("Installation Failed!")
			return
		} else {
			installMgr.SaveLog(app.ID)
			mgr.WriteMessage("Installation Succeeded!")
		}
	}

	installMgr.CleanTempFiles(v.IpaPath)
}

func HandleLoginMessage(c *websocket.Conn) {
	websocketMgr := manager.NewWebsocketManager(c)
	defer websocketMgr.Cancel()
	loginMgr := manager.NewLoginManager()
	loginMgr.OnOutput(func(line string) {
		websocketMgr.WriteMessage(line)
	})
	defer loginMgr.Close()

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
		case model.MessageTypeLogin:
			var v struct {
				Account  string `json:"account"`
				Password string `json:"password"`
			}
			err := json.Unmarshal([]byte(msg.Data), &v)
			if err != nil {
				msg := fmt.Sprintf("ERROR: %s", err.Error())
				_ = c.WriteMessage(websocket.TextMessage, []byte(msg))
				continue
			}

			if v.Account == "" || v.Password == "" {
				_ = c.WriteMessage(websocket.TextMessage, []byte("account or password is empty"))
				continue
			}

			go runLoginMessage(websocketMgr, loginMgr, v.Account, v.Password)
		case model.MessageType2FA:
			code := msg.Data
			loginMgr.Write([]byte(code + "\n"))
		default:
			_ = c.WriteMessage(websocket.TextMessage, []byte("ERROR: invalid message type"))
			continue
		}
	}
}

func runLoginMessage(mgr *manager.WebsocketManager, loginMgr *manager.LoginManager, account, password string) {
	err := loginMgr.Start(mgr.Context(), account, password)
	if err != nil {
		msg := fmt.Sprintf("ERROR: %s", err.Error())
		mgr.WriteMessage(msg)
		mgr.WriteMessage("Login Failed!")
		return
	}
	mgr.WriteMessage("Login Succeeded!")
}

func HandleScanMessage(c *websocket.Conn) {
	websocketMgr := manager.NewWebsocketManager(c)
	defer websocketMgr.Cancel()

	log.Info("Starting service scan via WebSocket...")
	ctx := websocketMgr.Context()

	err := manager.ScanServices(ctx, func(serviceType string, name string, host string, address string, port uint16, txt [][]byte) {
		// Convert txt to string array for JSON
		txtStrs := make([]string, len(txt))
		for i, b := range txt {
			txtStrs[i] = string(b)
		}

		data := map[string]any{
			"type":    serviceType,
			"name":    name,
			"host":    host,
			"address": address,
			"port":    port,
			"txt":     txtStrs,
		}
		bytes, _ := json.Marshal(data)
		websocketMgr.WriteMessage(string(bytes))
	})

	if err != nil {
		log.Err(err).Msg("ScanServices failed")
		websocketMgr.WriteMessage(fmt.Sprintf("ERROR: %s", err.Error()))
	}
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
