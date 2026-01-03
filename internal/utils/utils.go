package utils

import (
	"os"
	"path/filepath"
	"strings"
)

func Exists(name string) bool {
	if _, err := os.Stat(name); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}

func Ext(path string) string {
	return strings.ToLower(filepath.Ext(path))
}

func FileNameWithoutExt(fileName string) string {
	return fileName[:len(fileName)-len(filepath.Ext(fileName))]
}

func MergeEnvs(system, override []string) []string {
	envMap := make(map[string]string, len(system)+len(override))
	for _, e := range system {
		if i := strings.IndexByte(e, '='); i >= 0 {
			envMap[e[:i]] = e[i+1:]
		}
	}
	for _, e := range override {
		if i := strings.IndexByte(e, '='); i >= 0 {
			envMap[e[:i]] = e[i+1:]
		}
	}
	merged := make([]string, 0, len(envMap))
	for k, v := range envMap {
		merged = append(merged, k+"="+v)
	}
	return merged
}
