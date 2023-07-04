package router

import (
	"fmt"
	"image/png"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/bitxeno/atvloadly/config"
	"github.com/bitxeno/atvloadly/internal/cfg"
	"github.com/bitxeno/atvloadly/internal/utils"
	"github.com/bitxeno/atvloadly/ipa"
	"github.com/bitxeno/atvloadly/manager"
	"github.com/bitxeno/atvloadly/model"
	"github.com/bitxeno/atvloadly/notify"
	"github.com/bitxeno/atvloadly/service"
	"github.com/bitxeno/atvloadly/task"
	"github.com/bitxeno/atvloadly/tty"
	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
)

func Create(app *fiber.App, f fs.FS) {
	app.Use("/", filesystem.New(filesystem.Config{
		Root: http.FS(f),
	}))
	app.Use("/ws", func(c *fiber.Ctx) error {
		// IsWebSocketUpgrade returns true if the client
		// requested upgrade to the WebSocket protocol.
		if websocket.IsWebSocketUpgrade(c) {
			c.Locals("allowed", true)
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})
	app.Get("/ws/tty", websocket.New(func(c *websocket.Conn) {
		term, err := tty.New(c, "bash")
		if err != nil {
			_ = c.WriteMessage(websocket.TextMessage, []byte(err.Error()))
			return
		}
		defer term.Close()

		term.SetCWD(cfg.Server.WorkDir)
		term.SetENV([]string{"ALTSERVER_ANISETTE_SERVER=\"http://127.0.0.1:6969\""})
		term.Start()
	}))
	app.Get("/apps/:id/icon", func(c *fiber.Ctx) error {
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
	app.Get("/apps/:id/log", func(c *fiber.Ctx) error {
		id := utils.MustParseInt(c.Params("id"))

		path := filepath.Join(cfg.Server.WorkDir, "log", fmt.Sprintf("task_%d.log", id))
		return c.Status(http.StatusOK).SendFile(path, false)

	})

	// api路由
	api := app.Group("/api")
	api.Get("/hello", func(c *fiber.Ctx) error {
		return c.SendString("hello world.")
	})
	api.Get("/settings", func(c *fiber.Ctx) error {
		return c.Status(http.StatusOK).JSON(apiSuccess(config.Settings))
	})
	api.Post("/settings/:key", func(c *fiber.Ctx) error {
		var settings config.SettingsConfiguration
		if err := c.BodyParser(&settings); err != nil {
			return c.Status(http.StatusOK).JSON(apiError("Invalid argument. error: " + err.Error()))
		}

		key := c.Params("key")
		switch key {
		case "notification":
			config.Settings.Notification = settings.Notification
		case "task":
			config.Settings.Task = settings.Task
			if err := task.ReloadTask(); err != nil {
				return c.Status(http.StatusOK).JSON(apiError("时间格式错误: " + err.Error()))
			}
		}

		config.SaveSettings()
		return c.Status(http.StatusOK).JSON(apiSuccess(true))
	})

	api.Get("/devices", func(c *fiber.Ctx) error {
		manager.ReloadDevices()

		devices, err := manager.GetDevices()
		if err != nil {
			return c.Status(500).JSON(apiError(err.Error()))
		} else {
			return c.Status(http.StatusOK).JSON(apiSuccess(devices))
		}
	})

	api.Get("/devices/:id", func(c *fiber.Ctx) error {
		id := c.Params("id")
		if device, ok := manager.GetDeviceByID(id); ok {
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

	api.Get("/scan", func(c *fiber.Ctx) error {
		manager.ScanDevices()

		devices, err := manager.GetDevices()
		if err != nil {
			return c.Status(500).JSON(apiError(err.Error()))
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
			return c.Status(500).JSON(apiError(err.Error()))
		} else {
			return c.Status(http.StatusOK).JSON(apiSuccess(devices))
		}
	})

	api.Post("/upload", func(c *fiber.Ctx) error {
		form, _ := c.MultipartForm()
		files := form.File["files"]

		result := []model.IpaFile{}
		for _, file := range files {
			saveDir := filepath.Join(cfg.Server.WorkDir, "tmp")
			if err := os.MkdirAll(saveDir, os.ModePerm); err != nil {
				return c.Status(500).JSON(apiError("failed to create directory :" + saveDir))
			}

			name := service.GetValidName(utils.FileNameWithoutExt(file.Filename))
			dstName := fmt.Sprintf("%s_%d%s", name, time.Now().Unix(), filepath.Ext(file.Filename))
			dst := filepath.Join(saveDir, dstName)

			// Upload the file to specific dst.
			if err := c.SaveFile(file, dst); err != nil {
				return c.Status(500).JSON(apiError(err.Error()))
			}

			ipaFile := model.IpaFile{
				Name: file.Filename,
				Path: dst,
			}

			info, err := ipa.ParseFile(dst)
			if err != nil {
				return c.Status(500).JSON(apiError(err.Error()))
			}

			ipaFile.Name = info.Name()
			ipaFile.BundleIdentifier = info.Identifier()
			ipaFile.Version = info.Version()
			if err := os.MkdirAll(saveDir, os.ModePerm); err != nil {
				return c.Status(500).JSON(apiError("failed to create directory :" + saveDir))
			}

			// 保存icon
			if info.Icon() != nil {
				iconName := fmt.Sprintf("%s_%d%s", name, time.Now().Unix(), ".png")
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
			return c.Status(500).JSON(apiError(err.Error()))
		} else {
			return c.Status(http.StatusOK).JSON(apiSuccess(apps))
		}
	})

	api.Get("/apps/installing", func(c *fiber.Ctx) error {
		app := task.GetCurrentInstallingApp()
		return c.Status(http.StatusOK).JSON(apiSuccess(app))
	})

	api.Post("/apps", func(c *fiber.Ctx) error {
		var installApp model.InstalledApp
		if err := c.BodyParser(&installApp); err != nil {
			return c.Status(500).JSON(apiError(err.Error()))
		}

		ipa, err := service.SaveApp(installApp)
		if err != nil {
			return c.Status(500).JSON(apiError(err.Error()))
		} else {
			return c.Status(http.StatusOK).JSON(apiSuccess(ipa))
		}

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
		var settings config.SettingsConfiguration
		if err := c.BodyParser(&settings); err != nil {
			return c.Status(http.StatusOK).JSON(apiError("Invalid argument. error: " + err.Error()))
		}

		if err := notify.SendWithConfig("test", "content", settings); err != nil {
			return c.Status(http.StatusOK).JSON(apiError(err.Error()))
		} else {
			return c.Status(http.StatusOK).JSON(apiSuccess(true))
		}
	})

}
