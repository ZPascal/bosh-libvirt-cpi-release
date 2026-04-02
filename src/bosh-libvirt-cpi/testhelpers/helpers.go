package testhelpers

import (
	"os"
)

// CreateTempFile creates a temporary file with the given content
func CreateTempFile(content string) string {
	tmpFile, err := os.CreateTemp("", "test-*")
	if err != nil {
		panic(err)
	}
	defer func() {
		_ = tmpFile.Close()
	}()

	if content != "" {
		_, err = tmpFile.WriteString(content)
		if err != nil {
			panic(err)
		}
	}

	return tmpFile.Name()
}
