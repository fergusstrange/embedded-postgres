package embeddedpostgres

import (
	"encoding/base64"
	"os"
	"testing"

	"go.uber.org/goleak"
)

func createTempXzArchive() (string, func()) {
	return writeFileWithBase64Content("remote_fetch_test*.txz", "/Td6WFoAAATm1rRGAgAhARYAAAB0L+Wj4Av/AKZdADIaSqdFdWDG5Dyin7tszujmfm9YJn6/1REVUfqW8HwXvgwbrrcDDc4Q2ql+L+ybLTxJ+QNhhaKnawviRjKhUOT3syXi2Ye8k4QMkeurnnCu4a8eoCV+hqNFWkk8/w8MzyMzQZ2D3wtvoaZV/KqJ8jyLbNVj+vsKrzqg5vbSGz5/h7F37nqN1V8ZsdCnKnDMZPzovM8RwtelDd0g3fPC0dG/W9PH4wAAAAC2dqs1k9ZA0QABwgGAGAAAIQZ5XbHEZ/sCAAAAAARZWg==")
}

func createTempZipArchive() (string, func()) {
	return writeFileWithBase64Content("remote_fetch_test*.zip", "UEsDBBQACAAIAExBSlMAAAAAAAAAAAAAAAAaAAkAcmVtb3RlX2ZldGNoX3Rlc3Q4MDA0NjE5MDVVVAUAAfCfYmEBAAD//1BLBwgAAAAABQAAAAAAAABQSwMEFAAIAAAATEFKUwAAAAAAAAAAAAAAABUACQByZW1vdGVfZmV0Y2hfdGVzdC50eHpVVAUAAfCfYmH9N3pYWgAABObWtEYCACEBFgAAAHQv5aPgBf8Abl0AORlJ/tq+A8rMBye1kCuXLnw2aeeO0gdfXeVHCWpF8/VeZU/MTVkdLzI+XgKLEMlHJukIdxP7iSAuKts+v7aDrJu68RHNgIsXGrGouAjf780FXjTUjX4vXDh08vNY1yOBayt9z9dKHdoG9AeAIgAAAAAOKMpgA1Mm3wABigGADAAAjIVdpbHEZ/sCAAAAAARZWlBLBwhkmQgRsAAAALAAAABQSwECFAMUAAgACABMQUpTAAAAAAUAAAAAAAAAGgAJAAAAAAAAAAAAgIEAAAAAcmVtb3RlX2ZldGNoX3Rlc3Q4MDA0NjE5MDVVVAUAAfCfYmFQSwECFAMUAAgAAABMQUpTZJkIEbAAAACwAAAAFQAJAAAAAAAAAAAApIFWAAAAcmVtb3RlX2ZldGNoX3Rlc3QudHh6VVQFAAHwn2JhUEsFBgAAAAACAAIAnQAAAFIBAAAAAA==")
}

func writeFileWithBase64Content(filename, base64Content string) (string, func()) {
	tempFile, err := os.CreateTemp("", filename)
	if err != nil {
		panic(err)
	}

	byteContent, err := base64.StdEncoding.DecodeString(base64Content)
	if err != nil {
		panic(err)
	}

	if err := os.WriteFile(tempFile.Name(), byteContent, 0666); err != nil {
		panic(err)
	}

	return tempFile.Name(), func() {
		if err := os.RemoveAll(tempFile.Name()); err != nil {
			panic(err)
		}
	}
}

func shutdownDBAndFail(t *testing.T, err error, db *EmbeddedPostgres) {
	if db.started {
		if stopErr := db.Stop(); stopErr != nil {
			t.Errorf("Failed to shutdown server with error %s", stopErr)
		}
	}

	t.Errorf("Failed for version %s with error %s", db.config.version, err)
}

func testVersionStrategy() VersionStrategy {
	return func() (string, string, PostgresVersion) {
		return "darwin", "amd64", "1.2.3"
	}
}

func testCacheLocator() CacheLocator {
	return func() (s string, b bool) {
		return "", false
	}
}

func verifyLeak(t *testing.T) {
	// Ideally, there should be no exceptions here.
	goleak.VerifyNone(t, goleak.IgnoreTopFunction("internal/poll.runtime_pollWait"))
}
