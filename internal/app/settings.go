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
		Enabled  bool     `koanf:"enabled" json:"enabled" default:"true"`
		Mode     TaskMode `koanf:"mode" json:"mode" default:"1"`
		CrodTime string   `koanf:"crod_time" json:"crod_time" default:"0,30 3-6 * * *"`
	} `koanf:"task" json:"task"`
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
	} `koanf:"notification" json:"notification"`
}

func SaveSettings() {
	saveTimer.Reset(100 * time.Millisecond)
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
			if err := os.WriteFile(settingsPath, data, os.ModePerm); err != nil {
				log.Err(err).Msg("Save settings error.")
			} else {
				log.Infof("Save settings success. %s", settingsPath)
			}
		}
	}()
}
