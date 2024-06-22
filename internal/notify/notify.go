package notify

import (
	"context"
	"errors"

	"github.com/bitxeno/atvloadly/internal/app"
	"github.com/bitxeno/atvloadly/internal/notify/wecom"
	"github.com/bitxeno/atvloadly/internal/utils"
	"github.com/nikoksr/notify"
	"github.com/nikoksr/notify/service/bark"
	"github.com/nikoksr/notify/service/telegram"
	"github.com/silenceper/wechat/v2/cache"
)

func Send(title string, message string) error {
	return SendWithConfig(title, message, *app.Settings)
}

func SendWithConfig(title string, message string, settings app.SettingsConfiguration) error {
	if !settings.Notification.Enabled {
		return errors.New("未启用")
	}

	no := notify.New()
	switch settings.Notification.Type {
	case "bark":
		deviceKey := settings.Notification.Bark.DeviceKey
		barkServer := settings.Notification.Bark.BarkServer
		if deviceKey == "" || barkServer == "" {
			return errors.New("配置错误")
		}
		barkService := bark.NewWithServers(deviceKey, barkServer)
		no.UseServices(barkService)
	case "telegram":
		chatId := utils.MustParseInt64(settings.Notification.Telegram.ChatID)
		if chatId == 0 || settings.Notification.Telegram.BotToken == "" {
			return errors.New("配置错误")
		}
		telegramService, _ := telegram.New(settings.Notification.Telegram.BotToken)
		telegramService.AddReceivers(chatId)
		no.UseServices(telegramService)
	case "weixin":
		if settings.Notification.Weixin.CorpID == "" ||
			settings.Notification.Weixin.CorpSecret == "" ||
			settings.Notification.Weixin.ToUser == "" ||
			settings.Notification.Weixin.AgentID == "" {
			return errors.New("配置错误")
		}
		wecomService := wecom.New(&wecom.Config{
			CorpID:     settings.Notification.Weixin.CorpID,
			CorpSecret: settings.Notification.Weixin.CorpSecret,
			AgentID:    settings.Notification.Weixin.AgentID,
			Cache:      cache.NewMemory(),
		})
		wecomService.AddReceivers(settings.Notification.Weixin.ToUser)
		no.UseServices(wecomService)
	}

	return no.Send(
		context.Background(),
		title,
		message,
	)
}
