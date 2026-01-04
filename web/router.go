package web

import (
	"fmt"
	"image/png"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/bitxeno/atvloadly/internal/app"
	"github.com/bitxeno/atvloadly/internal/i18n"
	"github.com/bitxeno/atvloadly/internal/ipa"
	"github.com/bitxeno/atvloadly/internal/manager"
	"github.com/bitxeno/atvloadly/internal/model"
	"github.com/bitxeno/atvloadly/internal/notify"
	"github.com/bitxeno/atvloadly/internal/service"
	"github.com/bitxeno/atvloadly/internal/task"
	"github.com/bitxeno/atvloadly/internal/tty"
	"github.com/bitxeno/atvloadly/internal/utils"
	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
)

func route(fi *fiber.App) {
	fi.Use("/", filesystem.New(filesystem.Config{
		Root: http.FS(StaticAssets()),
	}))
	fi.Use("/ws", func(c *fiber.Ctx) error {
		// IsWebSocketUpgrade returns true if the client
		// requested upgrade to the WebSocket protocol.
		if websocket.IsWebSocketUpgrade(c) {
			c.Locals("allowed", true)
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})
	fi.Get("/ws/tty", websocket.New(func(c *websocket.Conn) {
		term, err := tty.New(c, "bash")
		if err != nil {
			msg := fmt.Sprintf("ERROR: %s", err.Error())
			_ = c.WriteMessage(websocket.TextMessage, []byte(msg))
			return
		}
		defer term.Close()

		term.SetCWD(app.Config.Server.DataDir)
		term.Start()
	}))
	fi.Get("/ws/pair", websocket.New(service.HandlePairMessage))
	fi.Get("/ws/install", websocket.New(service.HandleInstallMessage))
	fi.Get("/ws/login", websocket.New(service.HandleLoginMessage))
	fi.Get("/apps/:id/icon", func(c *fiber.Ctx) error {
		id := utils.MustParseInt(c.Params("id"))

		t, err := service.GetApp(uint(id))
		if err != nil {
			return c.Status(http.StatusNotFound).SendString(err.Error())
		}

		if t.Icon != "" {
			return c.Status(http.StatusOK).SendFile(t.Icon, false)
		} else {
			return c.Status(http.StatusNotFound).SendString("")
		}
	})
	fi.Get("/apps/:id/log", func(c *fiber.Ctx) error {
		id := utils.MustParseInt(c.Params("id"))

		path := filepath.Join(app.Config.Server.DataDir, "log", fmt.Sprintf("task_%d.log", id))
		c.Set("Cache-Control", "no-cache, no-store, must-revalidate;")
		c.Set("pragma", "no-cache")
		return c.Status(http.StatusOK).SendFile(path, false)

	})

	// API route
	api := fi.Group("/api")
	api.Get("/hello", func(c *fiber.Ctx) error {
		return c.SendString("hello world.")
	})
	api.Post("/lang/sync", func(c *fiber.Ctx) error {
		lang := c.Query("lang")
		accept := c.Get("Accept-Language")
		if lang != "" {
			service.SetLanguage(lang)
		} else {
			service.SetLanguage(accept)
		}
		return c.Status(http.StatusOK).JSON(apiSuccess(i18n.Localize("language")))
	})
	api.Get("/settings", func(c *fiber.Ctx) error {
		return c.Status(http.StatusOK).JSON(apiSuccess(app.Settings))
	})

	api.Get("/accounts", func(c *fiber.Ctx) error {
		accounts, err := manager.GetAppleAccounts()
		if err != nil {
			return c.Status(http.StatusOK).JSON(apiError(err.Error()))
		}

		return c.Status(http.StatusOK).JSON(apiSuccess(accounts.Accounts))
	})

	api.Post("/accounts/delete", func(c *fiber.Ctx) error {
		var req struct {
			Email string `json:"email"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(http.StatusOK).JSON(apiError("Invalid argument"))
		}

		if err := manager.DeleteAppleAccount(req.Email); err != nil {
			return c.Status(http.StatusOK).JSON(apiError(err.Error()))
		}
		return c.Status(http.StatusOK).JSON(apiSuccess(true))
	})

	api.Get("/accounts/devices", func(c *fiber.Ctx) error {
		email := c.Query("email")
		if email == "" {
			return c.Status(http.StatusOK).JSON(apiError("email is required"))
		}
		devices, err := manager.GetAccountDevices(email)
		if err != nil {
			return c.Status(http.StatusOK).JSON(apiError(err.Error()))
		}
		return c.Status(http.StatusOK).JSON(apiSuccess(devices))
	})

	api.Post("/accounts/devices/delete", func(c *fiber.Ctx) error {
		var req struct {
			Email    string `json:"email"`
			DeviceID string `json:"deviceId"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(http.StatusOK).JSON(apiError("Invalid argument"))
		}
		if err := manager.DeleteAccountDevice(req.Email, req.DeviceID); err != nil {
			return c.Status(http.StatusOK).JSON(apiError(err.Error()))
		}
		return c.Status(http.StatusOK).JSON(apiSuccess(true))
	})

	api.Get("/certificates", func(c *fiber.Ctx) error {
		email := c.Query("email")
		if email == "" {
			return c.Status(http.StatusOK).JSON(apiError("email is required"))
		}
		certs, err := manager.GetCertificates(email)
		if err != nil {
			return c.Status(http.StatusOK).JSON(apiError(err.Error()))
		}
		return c.Status(http.StatusOK).JSON(apiSuccess(certs))
	})

	api.Post("/certificates/revoke", func(c *fiber.Ctx) error {
		var req struct {
			Email        string `json:"email"`
			SerialNumber string `json:"serialNumber"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(http.StatusOK).JSON(apiError("Invalid argument"))
		}
		if err := manager.RevokeCertificate(req.Email, req.SerialNumber); err != nil {
			return c.Status(http.StatusOK).JSON(apiError(err.Error()))
		}
		return c.Status(http.StatusOK).JSON(apiSuccess(true))
	})

	api.Post("/settings/:key", func(c *fiber.Ctx) error {
		var settings app.SettingsConfiguration
		if err := c.BodyParser(&settings); err != nil {
			return c.Status(http.StatusOK).JSON(apiError("Invalid argument. error: " + err.Error()))
		}

		key := c.Params("key")
		switch key {
		case "notification":
			app.Settings.Notification = settings.Notification
		case "task":
			app.Settings.Task = settings.Task
			if err := task.ReloadTask(); err != nil {
				return c.Status(http.StatusOK).JSON(apiError("时间格式错误: " + err.Error()))
			}
		}

		app.SaveSettings()
		return c.Status(http.StatusOK).JSON(apiSuccess(true))
	})

	api.Get("/devices", func(c *fiber.Ctx) error {
		manager.ReloadDevices()

		devices, err := manager.GetDevices()
		if err != nil {
			return c.Status(http.StatusOK).JSON(apiError(err.Error()))
		} else {
			return c.Status(http.StatusOK).JSON(apiSuccess(devices))
		}
	})

	api.Get("/devices/:id", func(c *fiber.Ctx) error {
		id := c.Params("id")
		if device, ok := manager.GetDeviceByID(id); ok {
			manager.AppendDeviceProductInfo(device)
			return c.Status(http.StatusOK).JSON(apiSuccess(device))
		}

		return c.Status(http.StatusOK).JSON(apiError("not found"))
	})

	api.Post("/devices/:id/mountimage", func(c *fiber.Ctx) error {
		id := c.Params("id")

		if err := service.MountDeveloperDiskImage(c.Context(), id); err != nil {
			return c.Status(http.StatusOK).JSON(apiSuccess(err.Error()))
		} else {
			return c.Status(http.StatusOK).JSON(apiSuccess("success"))
		}
	})

	api.Post("/devices/:id/check/devmode", func(c *fiber.Ctx) error {
		id := c.Params("id")

		enabled, err := service.CheckDeveloperMode(c.Context(), id)
		if err != nil {
			return c.Status(http.StatusOK).JSON(apiSuccess(err.Error()))
		}
		return c.Status(http.StatusOK).JSON(apiSuccess(enabled))
	})

	api.Post("/devices/:id/check/afc", func(c *fiber.Ctx) error {
		id := c.Params("id")

		if err := service.CheckAfcService(c.Context(), id); err != nil {
			return c.Status(http.StatusOK).JSON(apiError(err.Error()))
		} else {
			return c.Status(http.StatusOK).JSON(apiSuccess("success"))
		}
	})

	api.Get("/scan", func(c *fiber.Ctx) error {
		manager.ScanDevices()

		devices, err := manager.GetDevices()
		if err != nil {
			return c.Status(http.StatusOK).JSON(apiError(err.Error()))
		} else {
			return c.Status(http.StatusOK).JSON(apiSuccess(devices))
		}
	})

	api.Get("/reload", func(c *fiber.Ctx) error {
		manager.ReloadDevices()

		return c.Status(http.StatusOK).JSON(apiSuccess(true))
	})

	api.Post("/pair", func(c *fiber.Ctx) error {
		devices, err := manager.GetDevices()
		if err != nil {
			return c.Status(http.StatusOK).JSON(apiError(err.Error()))
		} else {
			return c.Status(http.StatusOK).JSON(apiSuccess(devices))
		}
	})

	api.Post("/upload", func(c *fiber.Ctx) error {
		form, _ := c.MultipartForm()
		files := form.File["files"]

		result := []model.IpaFile{}
		saveDir := filepath.Join(app.Config.Server.DataDir, "tmp")
		if err := os.MkdirAll(saveDir, os.ModePerm); err != nil {
			return c.Status(http.StatusOK).JSON(apiError("failed to create directory :" + saveDir))
		}
		for _, file := range files {
			timestamp := time.Now().UnixMicro()
			name := service.GetValidName(utils.FileNameWithoutExt(file.Filename))
			dstName := fmt.Sprintf("%s_%d%s", name, timestamp, filepath.Ext(file.Filename))
			dst := filepath.Join(saveDir, dstName)

			// Upload the file to specific dst.
			if err := c.SaveFile(file, dst); err != nil {
				return c.Status(http.StatusOK).JSON(apiError(err.Error()))
			}

			ipaFile := model.IpaFile{
				Name: file.Filename,
				Path: dst,
			}

			info, err := ipa.ParseFile(dst)
			if err != nil {
				return c.Status(http.StatusOK).JSON(apiError(err.Error()))
			}

			ipaFile.Name = info.Name()
			ipaFile.BundleIdentifier = info.Identifier()
			ipaFile.Version = info.Version()

			// 保存icon
			if info.Icon() != nil {
				iconName := fmt.Sprintf("%s_%d%s", name, timestamp, ".png")
				iconDst := filepath.Join(saveDir, iconName)
				out, err := os.Create(iconDst)
				if err == nil {
					defer out.Close()

					if err := png.Encode(out, info.Icon()); err == nil {
						ipaFile.Icon = iconDst
					}
				}
			}

			result = append(result, ipaFile)
		}

		return c.Status(http.StatusOK).JSON(apiSuccess(result))
	})

	api.Get("/apps", func(c *fiber.Ctx) error {
		apps, err := service.GetAppList()
		if err != nil {
			return c.Status(http.StatusOK).JSON(apiError(err.Error()))
		} else {
			return c.Status(http.StatusOK).JSON(apiSuccess(apps))
		}
	})

	api.Get("/apps/installing", func(c *fiber.Ctx) error {
		return c.Status(http.StatusOK).JSON(apiSuccess(task.GetCurrentInstallingApps()))
	})

	api.Post("/apps", func(c *fiber.Ctx) error {
		var installApp model.InstalledApp
		if err := c.BodyParser(&installApp); err != nil {
			return c.Status(http.StatusOK).JSON(apiError(err.Error()))
		}

		ipa, err := service.SaveApp(installApp)
		if err != nil {
			return c.Status(http.StatusOK).JSON(apiError(err.Error()))
		} else {
			return c.Status(http.StatusOK).JSON(apiSuccess(ipa))
		}
	})

	api.Post("/clean", func(c *fiber.Ctx) error {
		var ipa model.IpaFile
		if err := c.BodyParser(&ipa); err != nil {
			return c.Status(http.StatusOK).JSON(apiError(err.Error()))
		}

		// clean upload temp file
		if ipa.Path != "" {
			_ = os.RemoveAll(ipa.Path)
		}
		if ipa.Icon != "" {
			_ = os.RemoveAll(ipa.Icon)
		}

		return c.Status(http.StatusOK).JSON(apiSuccess(true))
	})

	api.Post("/apps/:id/delete", func(c *fiber.Ctx) error {
		id := utils.MustParseInt(c.Params("id"))

		ok, err := service.DeleteApp(uint(id))
		if err != nil {
			return c.Status(http.StatusOK).JSON(apiError(err.Error()))
		} else {
			return c.Status(http.StatusOK).JSON(apiSuccess(ok))
		}
	})

	api.Post("/apps/:id/refresh", func(c *fiber.Ctx) error {
		id := utils.MustParseInt(c.Params("id"))

		t, err := service.GetApp(uint(id))
		if err != nil {
			return c.Status(http.StatusOK).JSON(apiError(err.Error()))
		}

		task.RunInstallApp(*t)
		return c.Status(http.StatusOK).JSON(apiSuccess(true))
	})

	api.Get("/service/status", func(c *fiber.Ctx) error {
		status := service.GetServiceStatus()
		return c.Status(http.StatusOK).JSON(apiSuccess(status))
	})

	api.Get("/notify/send", func(c *fiber.Ctx) error {
		title := c.Query("title")
		desc := c.Query("desc")

		if err := notify.Send(title, desc); err != nil {
			return c.Status(http.StatusOK).JSON(apiError(err.Error()))
		} else {
			return c.Status(http.StatusOK).JSON(apiSuccess(true))
		}
	})

	api.Post("/notify/send/test", func(c *fiber.Ctx) error {
		var settings app.SettingsConfiguration
		if err := c.BodyParser(&settings); err != nil {
			return c.Status(http.StatusOK).JSON(apiError("Invalid argument. error: " + err.Error()))
		}
		settings.Notification.Enabled = true

		if err := notify.SendWithConfig("atvloadly", "test message", settings); err != nil {
			return c.Status(http.StatusOK).JSON(apiError(err.Error()))
		} else {
			return c.Status(http.StatusOK).JSON(apiSuccess(true))
		}
	})

}
