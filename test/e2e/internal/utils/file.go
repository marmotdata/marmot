package utils

import (
	"os"
)

func CreateTempConfigFile(content string) (string, error) {
	tmpFile, err := os.CreateTemp("", "marmot-test-*.yaml")
	if err != nil {
		return "", err
	}

	if _, err := tmpFile.Write([]byte(content)); err != nil {
		os.Remove(tmpFile.Name())
		return "", err
	}

	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpFile.Name())
		return "", err
	}

	return tmpFile.Name(), nil
}
