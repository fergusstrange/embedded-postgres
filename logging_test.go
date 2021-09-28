package embeddedpostgres

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

type customLogger struct {
	logLines []byte
}

func (cl *customLogger) Write(p []byte) (n int, err error) {
	cl.logLines = append(cl.logLines, p...)
	return len(p), nil
}

func Test_SyncedLogger_CreateError(t *testing.T) {
	logger := customLogger{}
	_, err := newSyncedLogger("/not-exists-anywhere", &logger)

	assert.Error(t, err)
}

func Test_SyncedLogger_ErrorDuringFlush(t *testing.T) {
	logger := customLogger{}

	sl, slErr := newSyncedLogger("", &logger)

	assert.NoError(t, slErr)

	rmFileErr := os.Remove(sl.file.Name())

	assert.NoError(t, rmFileErr)

	err := sl.flush()

	assert.Error(t, err)
}

func Test_SyncedLogger_NoErrorDuringFlush(t *testing.T) {
	logger := customLogger{}

	sl, slErr := newSyncedLogger("", &logger)

	assert.NoError(t, slErr)

	err := ioutil.WriteFile(sl.file.Name(), []byte("some logs\non a new line"), os.ModeAppend)

	assert.NoError(t, err)

	err = sl.flush()

	assert.NoError(t, err)

	assert.Equal(t, "some logs\non a new line", string(logger.logLines))
}
