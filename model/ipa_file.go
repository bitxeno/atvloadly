package model

type IpaFile struct {
	Name             string `json:"name"`
	Path             string `json:"path"`
	Icon             string `json:"icon"`
	BundleIdentifier string `json:"bundle_identifier"`
	Version          string `json:"version"`
}
