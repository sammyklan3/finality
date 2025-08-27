package utils

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

var (
	projectDir string
	SecretsDir string
)

func init() {
	_, file, _, _ := runtime.Caller(0)
	utilsDir := filepath.Dir(file)
	projectDir = filepath.Dir(utilsDir)
	SecretsDir = filepath.Join(projectDir, "secrets")
}

func LoadEnv() error {
	envFile := filepath.Join(projectDir, ".env")
	data, err := os.ReadFile(envFile)
	if err != nil {
		return err
	}

	lines := strings.Split(string(data), "\n")
	isEmpty := func(value string) bool {
		return strings.TrimSpace(value) == ""
	}

	for _, line := range lines {
		if isEmpty(line) {
			continue
		}
		fields := strings.Split(line, "=")
		if len(fields) != 2 {
			continue
		}

		key := fields[0]
		value := fields[1]
		value = strings.Trim(value, "\"")
		os.Setenv(key, value)
	}
	return nil
}
