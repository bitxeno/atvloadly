package i18n

import (
	"embed"
	"encoding/json"
	"path/filepath"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

//go:embed all:locales/*
var localesFs embed.FS
var i18nBundle *i18n.Bundle
var i18nLocalizer *i18n.Localizer
var currentLang = "en"

func init() {
	i18nBundle = i18n.NewBundle(language.English)
	i18nBundle.RegisterUnmarshalFunc("json", json.Unmarshal)

	root := "locales"
	files, err := localesFs.ReadDir(root)
	if err != nil {
		panic(err)
	}
	for _, v := range files {
		path := filepath.Join(root, v.Name())
		i18nBundle.LoadMessageFileFS(localesFs, path)
	}

	i18nLocalizer = i18n.NewLocalizer(i18nBundle, currentLang)
}

func SetLanguage(lang string) {
	if lang == "" {
		return
	}
	if currentLang != lang {
		currentLang = lang
		i18nLocalizer = i18n.NewLocalizer(i18nBundle, currentLang)
	}
}

func Localize(key string) string {
	return i18nLocalizer.MustLocalize(&i18n.LocalizeConfig{
		MessageID: key,
	})
}

func LocalizeF(key string, data map[string]interface{}) string {
	return i18nLocalizer.MustLocalize(&i18n.LocalizeConfig{
		MessageID:    key,
		TemplateData: data,
	})
}
