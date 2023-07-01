package version

var (
	/*********Will auto update by ci build *********/
	Version   = "unknown"
	Commit    = "unknown"
	BuildDate = "unknown"
	/*********Will auto update by ci build *********/
)

// Info holds build information
type Info struct {
	Commit    string `json:"commit"`
	Version   string `json:"version"`
	BuildDate string `json:"build_date"`
}

// Get creates and initialized Info object
func Get() Info {
	return Info{
		Commit:    Commit,
		Version:   Version,
		BuildDate: BuildDate,
	}
}
