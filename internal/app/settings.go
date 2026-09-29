package app

import (
	"math"
	"os"
	"time"

	"github.com/bitxeno/atvloadly/internal/log"
	"github.com/bitxeno/atvloadly/internal/utils"
)

var (
	Settings *SettingsConfiguration
)
var saveTimer *time.Timer = time.NewTimer(math.MaxInt64)

const (
	OneDayAgoMode TaskMode = "1"
	CustomMode    TaskMode = "2"
)

type TaskMode string

type SettingsConfiguration struct {
	App struct {
		Language string `koanf:"language" json:"language"`
	} `koanf:"app" json:"app"`
	Task struct {
		Enabled       bool     `koanf:"enabled" json:"enabled" default:"true"`
		IphoneEnabled bool     `koanf:"iphone_enabled" json:"iphone_enabled" default:"true"`
		Mode          TaskMode `koanf:"mode" json:"mode" default:"1"`
		CrodTime      string   `koanf:"crod_time" json:"crod_time" default:"0,30 3-6 * * *"`
		AdvanceDays   int      `koanf:"advance_days" json:"advance_days" default:"1"`
	} `koanf:"task" json:"task"`
	Update struct {
		CheckInterval int `koanf:"check_interval" json:"check_interval" default:"6"` // hours between background update checks, 0 disables them
		// GitHubToken holds the plaintext token in memory only. It is never
		// written to settings.json nor exposed through the settings API.
		GitHubToken string `koanf:"-" json:"-"`
		// GitHubTokenSealed is the AES-256-GCM sealed token persisted in
		// settings.json. It is never exposed through the settings API; the
		// API only reports whether a token is configured.
		GitHubTokenSealed string `koanf:"github_token_sealed" json:"github_token_sealed"`
	} `koanf:"update" json:"update"`
	Notification struct {
		Enabled  bool   `koanf:"enabled" json:"enabled"`
		Type     string `koanf:"type" json:"type" default:"weixin"`
		Telegram struct {
			BotToken string `koanf:"bot_token" json:"bot_token"`
			ChatID   string `koanf:"chat_id" json:"chat_id"`
		} `koanf:"telegram" json:"telegram"`
		Weixin struct {
			CorpID     string `koanf:"corp_id" json:"corp_id"`
			CorpSecret string `koanf:"corp_secret" json:"corp_secret"`
			AgentID    string `koanf:"agent_id" json:"agent_id"`
			ToUser     string `koanf:"to_user" json:"to_user"`
		} `koanf:"weixin" json:"weixin"`
		Bark struct {
			BarkServer string `koanf:"bark_server" json:"bark_server" default:"https://api.day.app"`
			DeviceKey  string `koanf:"device_key" json:"device_key"`
		} `koanf:"bark" json:"bark"`
		Email struct {
			SMTPHost string `koanf:"smtp_host" json:"smtp_host"`
			SMTPPort int    `koanf:"smtp_port" json:"smtp_port" default:"587"`
			Username string `koanf:"username" json:"username"`
			Password string `koanf:"password" json:"password"`
			From     string `koanf:"from" json:"from"`
			To       string `koanf:"to" json:"to"`
		} `koanf:"email" json:"email"`
		Webhook struct {
			URL         string `koanf:"url" json:"url"`
			Method      string `koanf:"method" json:"method" default:"POST"`
			ContentType string `koanf:"content_type" json:"content_type" default:"application/json"`
			Header      string `koanf:"header" json:"header"`
			Body        string `koanf:"body" json:"body"`
		} `koanf:"webhook" json:"webhook"`
	} `koanf:"notification" json:"notification"`
	Network struct {
		ProxyEnabled bool   `koanf:"proxy_enabled" json:"proxy_enabled"`
		HTTPProxy    string `koanf:"http_proxy" json:"http_proxy"`
		HTTPSProxy   string `koanf:"https_proxy" json:"https_proxy"`
	} `koanf:"network" json:"network"`
}

func SaveSettings() {
	saveTimer.Reset(100 * time.Millisecond)
}

// redactSecret reports whether a secret value is set without revealing it.
func redactSecret(v string) string {
	if v == "" {
		return "(empty)"
	}
	return "(set)"
}

// printRedactedSettings logs the settings for --debug without leaking tokens,
// passwords or the sealed GitHub blob.
func printRedactedSettings() {
	if Settings == nil {
		return
	}
	redacted := map[string]any{
		"app":  Settings.App,
		"task": Settings.Task,
		"update": map[string]any{
			"check_interval": Settings.Update.CheckInterval,
			"github_token":   redactSecret(Settings.Update.GitHubTokenSealed),
		},
		"notification": map[string]any{
			"enabled":  Settings.Notification.Enabled,
			"type":     Settings.Notification.Type,
			"telegram": map[string]any{"bot_token": redactSecret(Settings.Notification.Telegram.BotToken), "chat_id": Settings.Notification.Telegram.ChatID},
			"weixin":   map[string]any{"corp_id": Settings.Notification.Weixin.CorpID, "corp_secret": redactSecret(Settings.Notification.Weixin.CorpSecret), "agent_id": Settings.Notification.Weixin.AgentID},
			"bark":     map[string]any{"bark_server": Settings.Notification.Bark.BarkServer, "device_key": redactSecret(Settings.Notification.Bark.DeviceKey)},
			"email":    map[string]any{"username": Settings.Notification.Email.Username, "password": redactSecret(Settings.Notification.Email.Password)},
			"webhook":  map[string]any{"url": Settings.Notification.Webhook.URL, "method": Settings.Notification.Webhook.Method},
		},
		"network": Settings.Network,
	}
	log.Infof("Load settings (redacted): %s", utils.ToJSON(redacted))
}

func startSaveSettingsJob(settingsPath string) {
	go func() {
		for {
			<-saveTimer.C
			log.Infof("Start to save settings... %s", settingsPath)

			if settingsPath == "" {
				log.Info("Setting path is empty.")
				continue
			}

			data := utils.ToIndentJSON(Settings)
			// Secrets are sealed before persisting, and the file itself is
			// readable only by the owner to avoid leaking them via backups.
			if err := os.WriteFile(settingsPath, data, 0o600); err != nil {
				log.Err(err).Msg("Save settings error.")
			} else {
				log.Infof("Save settings success. %s", settingsPath)
			}
		}
	}()
}
