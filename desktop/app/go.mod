module github.com/bitxeno/atvloadly/desktop

go 1.27.1

require (
	github.com/bitxeno/atvloadly v0.0.0
	github.com/egoist/mygo v0.6.3
)

require (
	github.com/andybalholm/brotli v1.1.0 // indirect
	github.com/betamos/zeroconf v0.1.8-0.20250208023331-d559d61612b7 // indirect
	github.com/bitxeno/atvloadly/pkg/bom v0.0.0-00010101000000-000000000000 // indirect
	github.com/bitxeno/lzfse-go v0.1.0 // indirect
	github.com/bradfitz/gomemcache v0.0.0-20220106215444-fb4bf637b56d // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/creasty/defaults v1.5.2 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/ebitengine/purego v0.11.1 // indirect
	github.com/fasthttp/websocket v1.5.3 // indirect
	github.com/fatih/color v1.9.0 // indirect
	github.com/fsnotify/fsnotify v1.6.0 // indirect
	github.com/ggwhite/go-masker/v2 v2.1.0 // indirect
	github.com/glebarez/go-sqlite v1.21.2 // indirect
	github.com/glebarez/sqlite v1.11.0 // indirect
	github.com/go-errors/errors v1.5.1 // indirect
	github.com/go-redis/redis/v8 v8.11.6-0.20220405070650-99c79f7041fc // indirect
	github.com/go-resty/resty/v2 v2.6.0 // indirect
	github.com/go-telegram-bot-api/telegram-bot-api v4.6.4+incompatible // indirect
	github.com/godbus/dbus/v5 v5.0.4 // indirect
	github.com/gofiber/contrib/websocket v1.0.0 // indirect
	github.com/gofiber/fiber/v2 v2.52.5 // indirect
	github.com/google/jsonschema-go v0.4.2 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/gookit/event v1.1.2 // indirect
	github.com/holoplot/go-avahi v1.0.1 // indirect
	github.com/iineva/CgbiPngFix v0.0.0-20210523041253-b8869b346914 // indirect
	github.com/iineva/ipa-server v0.0.0-20210613102156-5a07a87df9a8 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.17.9 // indirect
	github.com/knadh/koanf v1.5.0 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mattn/go-runewidth v0.0.15 // indirect
	github.com/miekg/dns v1.1.62 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/go-ps v1.0.0 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modelcontextprotocol/go-sdk v0.8.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/nicksnyder/go-i18n/v2 v2.4.0 // indirect
	github.com/nikoksr/notify v0.41.0 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/robfig/cron/v3 v3.0.1 // indirect
	github.com/rs/zerolog v1.29.1 // indirect
	github.com/savsgio/gotils v0.0.0-20230208104028-c358bd845dee // indirect
	github.com/silenceper/wechat/v2 v2.1.5 // indirect
	github.com/smallstep/pkcs7 v0.2.3 // indirect
	github.com/technoweenie/multipartstreamer v1.0.1 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/valyala/fasthttp v1.55.0 // indirect
	github.com/valyala/tcplisten v1.0.0 // indirect
	github.com/yosida95/uritemplate/v3 v3.0.2 // indirect
	golang.org/x/crypto v0.57.0 // indirect
	golang.org/x/image v0.46.0 // indirect
	golang.org/x/mod v0.41.0 // indirect
	golang.org/x/net v0.59.0 // indirect
	golang.org/x/sync v0.24.0 // indirect
	golang.org/x/sys v0.48.0 // indirect
	golang.org/x/text v0.43.0 // indirect
	golang.org/x/tools v0.51.0 // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.0.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	gorm.io/gorm v1.25.7 // indirect
	howett.net/plist v1.0.1 // indirect
	modernc.org/libc v1.22.5 // indirect
	modernc.org/mathutil v1.5.0 // indirect
	modernc.org/memory v1.5.0 // indirect
	modernc.org/sqlite v1.23.1 // indirect
	software.sslmate.com/src/go-pkcs12 v0.7.3 // indirect
)

replace github.com/bitxeno/atvloadly => ../../

// The bom fork is a nested module of the root module; resolve it there too.
replace github.com/bitxeno/atvloadly/pkg/bom => ../../pkg/bom
