package service

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/bitxeno/atvloadly/internal/ipa"
	"github.com/bitxeno/atvloadly/internal/log"
	"github.com/bitxeno/atvloadly/internal/manager"
	"github.com/bitxeno/atvloadly/internal/model"
	"github.com/bitxeno/atvloadly/internal/signing"
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
				writeInstallRejected(websocketMgr, err.Error())
				continue
			}

			if err := validateInstallRequest(&v); err != nil {
				if v.IsExternalSigning() && signing.ClassOf(err) != "" {
					websocketMgr.WriteMessage(failureReportLine(err))
				}
				writeInstallRejected(websocketMgr, err.Error())
				continue
			}

			if err := ValidateCustomName(v.CustomName); err != nil {
				writeInstallRejected(websocketMgr, err.Error())
				continue
			}

			dev, found := manager.GetDeviceByUDID(v.UDID)
			if !found || dev == nil {
				writeInstallRejected(websocketMgr, fmt.Sprintf("device not found for UDID: %s", v.UDID))
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

// writeInstallRejected reports an install request refused before it started,
// ending it with the failure marker the install page waits for.
func writeInstallRejected(mgr *manager.WebsocketManager, reason string) {
	mgr.WriteMessage(fmt.Sprintf("ERROR: %s", reason))
	mgr.WriteMessage("\n")
	mgr.WriteMessage("Installation Failed!")
}

// validateInstallRequest checks an install request of the install page. Apple
// ID installs need an account and take the IPA path as given. External
// certificate installs need a signing identity and no account; they may carry
// a source like Apple ID installs. Their local IPA path is confined to the
// upload and installed apps directories, a remote URL passes as given. Only
// external certificate installs accept a custom bundle identifier; the plan
// reports an invalid one.
func validateInstallRequest(v *model.InstalledApp) error {
	if v.SigningMode != "" && !v.SigningMode.IsValid() {
		return fmt.Errorf("invalid signing mode: %q", v.SigningMode)
	}
	v.SigningMode = v.EffectiveSigningMode()
	v.CustomIdentifier = strings.TrimSpace(v.CustomIdentifier)
	switch v.SigningMode {
	case model.SigningModeAppleID:
		if v.Account == "" || v.UDID == "" {
			return fmt.Errorf("account or UDID is empty")
		}
		if v.CustomIdentifier != "" {
			return fmt.Errorf("a custom bundle identifier requires an external signing certificate")
		}
		v.SigningIdentityID = 0
		v.AllowMissingEntitlements = false
	case model.SigningModeExternalCertificate:
		if v.UDID == "" {
			return fmt.Errorf("UDID is empty")
		}
		if v.Account != "" {
			return fmt.Errorf("an account cannot be used with an external signing certificate")
		}
		if v.SigningIdentityID == 0 {
			return fmt.Errorf("no signing identity selected")
		}
		if !ipa.IsRemoteURL(v.IpaPath) {
			resolved, err := ResolveClientIPAPath(v.IpaPath)
			if err != nil {
				return err
			}
			v.IpaPath = resolved
		}
	}
	v.SignedBundleIdentifier = ""
	return nil
}

func runInstallMessage(mgr *manager.WebsocketManager, installMgr *manager.InstallManager, v model.InstalledApp, dev *model.Device) {
	if v.Source.Tracked() {
		mgr.WriteMessage("Resolving build from source...\n")
		if err := ResolveSourceInstall(&v); err != nil {
			mgr.WriteMessage(fmt.Sprintf("ERROR: %s", err.Error()))
			mgr.WriteMessage("\n")
			mgr.WriteMessage("Installation Failed!")
			return
		}
	} else {
		// Only the server writes source data.
		v.Source = model.AppSource{}
	}

	ipaPath := v.IpaPath
	if ipa.IsRemoteURL(ipaPath) {
		mgr.WriteMessage("Downloading IPA from URL...\n")
		lastPct := int64(-1)
		result, err := ipa.DownloadAndParse(ipaPath, func(downloaded, total int64) {
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
		ipaPath = result.LocalPath
		defer func() { _ = os.Remove(result.LocalPath) }()
		if result.IconPath != "" {
			defer func() { _ = os.Remove(result.IconPath) }()
		}
		mgr.WriteMessage("Download complete!\n")

		if err := ipa.CheckPlatform(result.Platforms, dev.DeviceClass); err != nil {
			mgr.WriteMessage(fmt.Sprintf("ERROR: %s", err.Error()))
			mgr.WriteMessage("\n")
			mgr.WriteMessage("Installation Failed!")
			return
		}

		v.IpaPath = result.LocalPath
		v.IpaName = result.Name
		v.BundleIdentifier = result.BundleIdentifier
		v.Version = result.Version
		v.Icon = result.IconPath
	}

	if v.IsExternalSigning() {
		runExternalInstallMessage(mgr, installMgr, v, dev)
		return
	}

	err := installMgr.Start(mgr.Context(), manager.InstallOptions{
		UDID:             v.UDID,
		Account:          v.Account,
		IP:               dev.IP,
		Port:             dev.Port,
		IpaPath:          ipaPath,
		CustomName:       v.CustomName,
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

// runExternalInstallMessage signs and installs v with its external signing
// identity. A failure is recorded on the previous record of the same app, if
// any; a first installation that fails creates no record, and a request
// cancelled by the install page records nothing. Only the upload files of v
// are cleaned up.
func runExternalInstallMessage(mgr *manager.WebsocketManager, installMgr *manager.InstallManager, v model.InstalledApp, dev *model.Device) {
	defer CleanExternalUpload(v.IpaPath, v.Icon)
	// RunExternalInstall releases its own lease before the record is saved.
	// DeleteSigningIdentity refuses a delete without force only while a lease
	// is held or an installed app uses the identity, so this lease covers the
	// window up to SaveApp.
	release := signing.AcquireLease(v.SigningIdentityID)
	defer release()

	result, err := RunExternalInstall(mgr.Context(), installMgr, v, dev, false)
	if err != nil {
		// Leaving the install page closes its socket, which cancels the
		// request: that is not a failure of the installed app.
		if mgr.Context().Err() == nil {
			recordExternalInstallFailure(v, err)
		}
		mgr.WriteMessage(fmt.Sprintf("ERROR: %s", err.Error()))
		mgr.WriteMessage("\n")
		mgr.WriteMessage("Installation Failed!")
		return
	}

	now := time.Now()
	expirationDate := result.ExpiresAt.Local()
	v.RefreshedDate = &now
	v.ExpirationDate = &expirationDate
	v.RefreshedResult = true
	v.RefreshedError = model.RefreshedErrorNone
	v.SignedBundleIdentifier = result.SignedBundleIdentifier

	app, err := SaveApp(v)
	if err != nil {
		mgr.WriteMessage(fmt.Sprintf("ERROR: save app to db failed. %s", err.Error()))
		mgr.WriteMessage("\n")
		mgr.WriteMessage("Installation Failed!")
		return
	}
	installMgr.SaveLog(app.ID)
	mgr.WriteMessage("Installation Succeeded!")
}

// recordExternalInstallFailure marks the previous record of the external
// certificate installation v as failed with the class of err.
func recordExternalInstallFailure(v model.InstalledApp, err error) {
	cur, found, findErr := findInstalledApp(v)
	if findErr != nil {
		log.Err(findErr).Msgf("Cannot find the installed app record of %s", v.IpaName)
		return
	}
	if !found {
		return
	}
	cur.RefreshedResult = false
	cur.RefreshedError = RefreshedErrorOf(err)
	if updateErr := UpdateAppRefreshResult(cur); updateErr != nil {
		log.Err(updateErr).Msgf("Cannot record the installation failure of %s", v.IpaName)
	}
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

	// Detect browser disconnects so discovery stops when the scanner page closes.
	go func() {
		for {
			if _, err := websocketMgr.ReadMessage(); err != nil {
				websocketMgr.Cancel()
				return
			}
		}
	}()

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

// HandleScreenshotMessage removed — the screenshot flow now uses the
// POST /api/devices/:id/screenshot REST endpoint instead of WebSocket.
