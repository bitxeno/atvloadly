package app

import "github.com/bitxeno/atvloadly/internal/app/build"

const (
	DevelopmentMode AppMode = "development"
	ProductionMode  AppMode = "production"
	TestMode        AppMode = "test"
)

var Mode = AppMode(build.Mode)

type AppMode string

func IsDevelopmentMode() bool {
	return build.Mode == string(DevelopmentMode)
}

func IsTestMode() bool {
	return build.Mode == string(TestMode)
}

func IsProductionMode() bool {
	return build.Mode == string(ProductionMode)
}
