package embeddedpostgres

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_renameOrIgnore_NoErrorOnEEXIST(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test_dir")
	require.NoError(t, err)

	tmpFil, err := os.CreateTemp("", "test_file")
	require.NoError(t, err)

	// os.Rename would return an error here, ensure that the error is handled and returned as nil
	err = renameOrIgnore(tmpFil.Name(), tmpDir)
	assert.NoError(t, err)
}
