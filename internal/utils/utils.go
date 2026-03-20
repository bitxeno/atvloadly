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
		if before, after, ok := strings.Cut(e, "="); ok {
			envMap[before] = after
		}
	}
	for _, e := range override {
		if before, after, ok := strings.Cut(e, "="); ok {
			envMap[before] = after
		}
	}
	merged := make([]string, 0, len(envMap))
	for k, v := range envMap {
		merged = append(merged, k+"="+v)
	}
	return merged
}

func MaskEmail(email string) string {
	email = strings.TrimSpace(email)
	if email == "" {
		return email
	}

	localPart, domain, found := strings.Cut(email, "@")
	if !found {
		if len(email) <= 2 {
			return strings.Repeat("*", len(email))
		}
		return email[:1] + strings.Repeat("*", len(email)-2) + email[len(email)-1:]
	}

	prefixLen := 1
	switch {
	case len(localPart) >= 8:
		prefixLen = 3
	case len(localPart) >= 5:
		prefixLen = 2
	}

	if len(localPart) <= 2 {
		localPart = strings.Repeat("*", len(localPart))
	} else {
		localPart = localPart[:prefixLen] + strings.Repeat("*", len(localPart)-prefixLen-1) + localPart[len(localPart)-1:]
	}

	return localPart + "@" + domain
}

func RemoveAllFiles(dir string, pattern string) {
	pat := filepath.Join(dir, pattern)
	matches, _ := filepath.Glob(pat)
	for _, m := range matches {
		_ = os.RemoveAll(m)
	}
}
