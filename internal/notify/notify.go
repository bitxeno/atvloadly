package notify

import (
	"context"
	"encoding/json"
	"errors"
	stdhttp "net/http"
	"net/url"
	"strings"

	"github.com/bitxeno/atvloadly/internal/app"
	"github.com/bitxeno/atvloadly/internal/notify/wecom"
	"github.com/bitxeno/atvloadly/internal/utils"
	"github.com/nikoksr/notify"
	"github.com/nikoksr/notify/service/bark"
	"github.com/nikoksr/notify/service/http"
	"github.com/nikoksr/notify/service/telegram"
	"github.com/silenceper/wechat/v2/cache"
)

func Send(title string, message string) error {
	return SendWithConfig(title, message, *app.Settings)
}

func SendWithConfig(title string, message string, settings app.SettingsConfiguration) error {
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
	case "webhook":
		if settings.Notification.Webhook.URL == "" {
			return errors.New("配置错误")
		}
		contentType := settings.Notification.Webhook.ContentType
		if contentType == "" {
			contentType = "application/json"
		}
		method := settings.Notification.Webhook.Method
		if method == "" {
			method = "POST"
		}
		httpService := http.New()
		webhookURL := settings.Notification.Webhook.URL
		webhookURL = strings.ReplaceAll(webhookURL, "{{title}}", url.QueryEscape(title))
		webhookURL = strings.ReplaceAll(webhookURL, "{{message}}", url.QueryEscape(message))
		headers := stdhttp.Header{}
		if settings.Notification.Webhook.Header != "" {
			for _, h := range strings.Split(settings.Notification.Webhook.Header, ";") {
				parts := strings.SplitN(h, ":", 2)
				if len(parts) == 2 {
					headers.Add(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
				}
			}
		}
		if headers.Get("User-Agent") == "" {
			headers.Set("User-Agent", "atvloadly")
		}
		webhook := &http.Webhook{
			URL:         webhookURL,
			Header:      headers,
			Method:      method,
			ContentType: contentType,
			BuildPayload: func(subject, message string) (payload any) {
				if isJSONContentType(contentType) {
					subject = sanitizeJSONTemplateValue(subject)
					message = sanitizeJSONTemplateValue(message)
				}
				body := settings.Notification.Webhook.Body
				body = strings.ReplaceAll(body, "{{title}}", subject)
				body = strings.ReplaceAll(body, "{{message}}", message)

				if strings.HasPrefix(strings.ToLower(contentType), "application/json") {
					var parsed any
					if err := json.Unmarshal([]byte(body), &parsed); err == nil {
						return parsed
					}
				}

				return body
			},
		}
		httpService.AddReceivers(webhook)
		no.UseServices(httpService)
	}

	return no.Send(
		context.Background(),
		title,
		message,
	)
}

func isJSONContentType(contentType string) bool {
	return strings.Contains(strings.ToLower(contentType), "json")
}

func sanitizeJSONTemplateValue(value string) string {
	normalized := strings.ToValidUTF8(value, "")
	quoted, err := json.Marshal(normalized)
	if err != nil || len(quoted) < 2 {
		return normalized
	}
	return string(quoted[1 : len(quoted)-1])
}
