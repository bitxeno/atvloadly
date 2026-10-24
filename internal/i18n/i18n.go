package i18n

import (
	"encoding/json"
	"io/fs"
	"path"
	"sync"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

var (
	mu               sync.RWMutex
	i18nBundle       *i18n.Bundle
	i18nLocalizer    *i18n.Localizer
	languageHandlers []func(string)
)

func Init(localesFs fs.FS) {
	i18nBundle = i18n.NewBundle(language.English)
	i18nBundle.RegisterUnmarshalFunc("json", json.Unmarshal)

	root := "locales"
	files, err := fs.ReadDir(localesFs, root)
	if err != nil {
		panic(err)
	}
	for _, v := range files {
		// fs.FS paths always use forward slashes, also on Windows.
		file := path.Join(root, v.Name())
		if _, err := i18nBundle.LoadMessageFileFS(localesFs, file); err != nil {
			panic(err)
		}
	}

	mu.Lock()
	i18nLocalizer = i18n.NewLocalizer(i18nBundle, "en")
	mu.Unlock()
}

// SetLanguage makes the app show the language of lang and calls the
// handlers registered with OnLanguageChanged.
func SetLanguage(lang string) {
	if lang == "" {
		return
	}
	mu.Lock()
	i18nLocalizer = i18n.NewLocalizer(i18nBundle, lang)
	handlers := append([]func(string){}, languageHandlers...)
	mu.Unlock()
	for _, handler := range handlers {
		handler(lang)
	}
}

// OnLanguageChanged registers a handler called with the language whenever
// SetLanguage changes it. The app uses it to relabel its native menu.
func OnLanguageChanged(handler func(lang string)) {
	if handler == nil {
		return
	}
	mu.Lock()
	languageHandlers = append(languageHandlers, handler)
	mu.Unlock()
}

func Localize(key string) string {
	mu.RLock()
	localizer := i18nLocalizer
	mu.RUnlock()
	return localizer.MustLocalize(&i18n.LocalizeConfig{
		MessageID: key,
	})
}

func LocalizeF(key string, data map[string]any) string {
	mu.RLock()
	localizer := i18nLocalizer
	mu.RUnlock()
	return localizer.MustLocalize(&i18n.LocalizeConfig{
		MessageID:    key,
		TemplateData: data,
	})
}
