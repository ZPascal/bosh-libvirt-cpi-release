package testhelpers

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateTempFile(t *testing.T) {
	content := "test content"
	path := CreateTempFile(content)
	defer os.Remove(path)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, content, string(data))
}

func TestCreateTempFileEmpty(t *testing.T) {
	path := CreateTempFile("")
	defer os.Remove(path)

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, int64(0), info.Size())
}

func TestCreateTempFileExists(t *testing.T) {
	path := CreateTempFile("content")
	defer os.Remove(path)

	_, err := os.Stat(path)
	assert.NoError(t, err)
}

func TestCreateTempFileMultiple(t *testing.T) {
	paths := []string{}
	defer func() {
		for _, p := range paths {
			os.Remove(p)
		}
	}()

	for i := 0; i < 3; i++ {
		p := CreateTempFile("test content")
		paths = append(paths, p)
	}

	assert.Equal(t, 3, len(paths))
	for i, p := range paths {
		assert.NotEmpty(t, p)
		// Verify each file is unique
		if i > 0 {
			assert.NotEqual(t, p, paths[i-1])
		}
	}
}
