package db

type Config struct {
	Path     string `koanf:"path"`
	FileName string `koanf:"filename" default:"app.db"`
	Debug    bool   `koanf:"debug" default:"false"`
}
