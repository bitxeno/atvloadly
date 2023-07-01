package mode

var (
	/*********Will auto update by ci build *********/
	Mode = "development"
	/*********Will auto update by ci build *********/
)

const (
	DevelopmentMode AppMode = "development"
	ProductionMode  AppMode = "production"
	TestMode        AppMode = "test"
)

type AppMode string

func Get() AppMode {
	return AppMode(Mode)
}

func IsDevelopmentMode() bool {
	return Mode == string(DevelopmentMode)
}

func IsTestMode() bool {
	return Mode == string(TestMode)
}

func IsProductionMode() bool {
	return Mode == string(ProductionMode)
}
