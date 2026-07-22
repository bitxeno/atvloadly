package service

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bitxeno/atvloadly/internal/app"
	atvhttp "github.com/bitxeno/atvloadly/internal/http"
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

			dev, found := manager.GetDeviceByUDID(v.UDID)
			if !found || dev == nil {
				_ = c.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("ERROR: device not found for UDID: %s", v.UDID)))
				continue
			}

			go runInstallMessage(websocketMgr, installMgr, v, dev)
		case model.MessageType2FA:
			code := msg.Data
			installMgr.Write([]byte(code + "\n"))
		default:
			_ = c.WriteMessage(websocket.TextMessage, []byte("ERROR: invalid message type"))
			continue
		}
	}
}

func runInstallMessage(mgr *manager.WebsocketManager, installMgr *manager.InstallManager, v model.InstalledApp, dev *model.Device) {
	ipaPath := v.IpaPath
	if strings.HasPrefix(ipaPath, "http:") || strings.HasPrefix(ipaPath, "https:") {
		mgr.WriteMessage("Downloading IPA from URL...\n")
		lastPct := int64(-1)
		tmpPath, err := downloadIPAFromURL(ipaPath, func(downloaded, total int64) {
			if total <= 0 {
				return
			}
			pct := downloaded * 100 / total
			if pct >= lastPct+5 {
				lastPct = pct - (pct % 5)
				mgr.WriteMessage(fmt.Sprintf("Download progress: %d%%\n", lastPct))
			}
		})
		if err != nil {
			msg := fmt.Sprintf("ERROR: failed to download IPA: %s", err.Error())
			mgr.WriteMessage(msg)
			mgr.WriteMessage("\n")
			mgr.WriteMessage("Installation Failed!")
			return
		}
		ipaPath = tmpPath
		defer os.Remove(tmpPath)
		mgr.WriteMessage("Download complete!\n")
	}

	err := installMgr.Start(mgr.Context(), manager.InstallOptions{
		UDID:             v.UDID,
		Account:          v.Account,
		IP:               dev.IP,
		Port:             dev.Port,
		IpaPath:          ipaPath,
		RemoveExtensions: v.RemoveExtensions,
		RefreshMode:      false,
	})
	if err != nil {
		installMgr.CleanTempFiles(v.IpaPath)
		msg := fmt.Sprintf("ERROR: %s", err.Error())
		mgr.WriteMessage(msg)
		mgr.WriteMessage("\n")
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
			mgr.WriteMessage("\n")
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
			id := msg.Data
			device, found := manager.GetDeviceByID(id)
			if !found || device == nil {
				websocketMgr.WriteMessage(fmt.Sprintf("ERROR: device not found: %s", id))
				continue
			}
			go runPairMessage(websocketMgr, pairMgr, *device)
		case model.MessageTypePairConfirm:
			code := msg.Data
			pairMgr.Write([]byte(code + "\n"))
		default:
			_ = c.WriteMessage(websocket.TextMessage, []byte("ERROR: invalid message type"))
			continue
		}
	}
}

func runPairMessage(mgr *manager.WebsocketManager, pairMgr *manager.PairManager, device model.Device) {
	err := pairMgr.Start(mgr.Context(), device)
	if err != nil {
		msg := fmt.Sprintf("ERROR: %s", err.Error())
		mgr.WriteMessage(msg)
		return
	}
}

func downloadIPAFromURL(rawURL string, progressFn func(downloaded, total int64)) (string, error) {
	saveDir := filepath.Join(app.Config.Server.DataDir, "tmp")
	if err := os.MkdirAll(saveDir, os.ModePerm); err != nil {
		return "", fmt.Errorf("failed to create temporary directory: %w", err)
	}

	tmpFile, err := os.CreateTemp(saveDir, "install_url_*.ipa")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary file: %w", err)
	}
	tmpPath := tmpFile.Name()

	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set(atvhttp.HEADER_USER_AGENT, atvhttp.HTTP_USER_AGENT)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("failed to download ipa: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("download failed with status code %d", resp.StatusCode)
	}

	writer := &progressWriter{
		dest:       tmpFile,
		total:      resp.ContentLength,
		progressFn: progressFn,
	}
	if _, err := io.Copy(writer, resp.Body); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("failed to write ipa file: %w", err)
	}
	_ = tmpFile.Close()

	return tmpPath, nil
}

type progressWriter struct {
	dest       io.WriteCloser
	total      int64
	downloaded int64
	progressFn func(downloaded, total int64)
}

func (pw *progressWriter) Write(p []byte) (int, error) {
	n, err := pw.dest.Write(p)
	pw.downloaded += int64(n)
	if pw.progressFn != nil {
		pw.progressFn(pw.downloaded, pw.total)
	}
	return n, err
}

// HandleScreenshotMessage removed — the screenshot flow now uses the
// POST /api/devices/:id/screenshot REST endpoint instead of WebSocket.
