package i18n

import (
	"encoding/json"
	"io/fs"
	"path/filepath"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

var i18nBundle *i18n.Bundle
var i18nLocalizer *i18n.Localizer

func Init(localesFs fs.FS) {
	i18nBundle = i18n.NewBundle(language.English)
	i18nBundle.RegisterUnmarshalFunc("json", json.Unmarshal)

	root := "locales"
	files, err := fs.ReadDir(localesFs, root)
	if err != nil {
		panic(err)
	}
	for _, v := range files {
		path := filepath.Join(root, v.Name())
		if _, err := i18nBundle.LoadMessageFileFS(localesFs, path); err != nil {
			panic(err)
		}
	}

	i18nLocalizer = i18n.NewLocalizer(i18nBundle, "en")
}

func SetLanguage(lang string) {
	if lang == "" {
		return
	}
	i18nLocalizer = i18n.NewLocalizer(i18nBundle, lang)
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
