package embeddedpostgres

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"github.com/stretchr/testify/require"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_defaultRemoteFetchStrategy_ErrorWhenHttpGet(t *testing.T) {
	remoteFetchStrategy := defaultRemoteFetchStrategy("http://localhost:1234/maven2",
		testVersionStrategy(),
		testCacheLocator())

	err := remoteFetchStrategy()

	assert.EqualError(t, err, "unable to connect to http://localhost:1234/maven2")
}

func Test_defaultRemoteFetchStrategy_ErrorWhenHttpStatusNot200(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	remoteFetchStrategy := defaultRemoteFetchStrategy(server.URL,
		testVersionStrategy(),
		testCacheLocator())

	err := remoteFetchStrategy()

	assert.EqualError(t, err, "no version found matching 1.2.3")
}

func Test_defaultRemoteFetchStrategy_ErrorWhenBodyReadIssue(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1")
	}))
	defer server.Close()

	remoteFetchStrategy := defaultRemoteFetchStrategy(server.URL+"/maven2",
		testVersionStrategy(),
		testCacheLocator())

	err := remoteFetchStrategy()

	assert.EqualError(t, err, "error fetching postgres: unexpected EOF")
}

func Test_defaultRemoteFetchStrategy_ErrorWhenCannotUnzipSubFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.RequestURI, ".sha256") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
	}))
	defer server.Close()

	remoteFetchStrategy := defaultRemoteFetchStrategy(server.URL+"/maven2",
		testVersionStrategy(),
		testCacheLocator())

	err := remoteFetchStrategy()

	assert.EqualError(t, err, "error fetching postgres: zip: not a valid zip file")
}

func Test_defaultRemoteFetchStrategy_ErrorWhenCannotUnzip(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.RequestURI, ".sha256") {
			w.WriteHeader(404)
			return
		}

		if _, err := w.Write([]byte("lolz")); err != nil {
			panic(err)
		}
	}))
	defer server.Close()

	remoteFetchStrategy := defaultRemoteFetchStrategy(server.URL+"/maven2",
		testVersionStrategy(),
		testCacheLocator())

	err := remoteFetchStrategy()

	assert.EqualError(t, err, "error fetching postgres: zip: not a valid zip file")
}

func Test_defaultRemoteFetchStrategy_ErrorWhenNoSubTarArchive(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.RequestURI, ".sha256") {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		MyZipWriter := zip.NewWriter(w)

		if err := MyZipWriter.Close(); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()

	remoteFetchStrategy := defaultRemoteFetchStrategy(server.URL+"/maven2",
		testVersionStrategy(),
		testCacheLocator())

	err := remoteFetchStrategy()

	assert.EqualError(t, err, "error fetching postgres: cannot find binary in archive retrieved from "+server.URL+"/maven2/io/zonky/test/postgres/embedded-postgres-binaries-darwin-amd64/1.2.3/embedded-postgres-binaries-darwin-amd64-1.2.3.jar")
}

func Test_defaultRemoteFetchStrategy_ErrorWhenCannotExtractSubArchive(t *testing.T) {
	jarFile, cleanUp := createTempZipArchive()
	defer cleanUp()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.RequestURI, ".sha256") {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		bytes, err := os.ReadFile(jarFile)
		if err != nil {
			panic(err)
		}
		if _, err := w.Write(bytes); err != nil {
			panic(err)
		}
	}))
	defer server.Close()

	remoteFetchStrategy := defaultRemoteFetchStrategy(server.URL+"/maven2",
		testVersionStrategy(),
		func() (s string, b bool) {
			return filepath.FromSlash("/invalid"), false
		})

	err := remoteFetchStrategy()

	assert.Regexp(t, "^unable to extract postgres archive:.+$", err)
}

func Test_defaultRemoteFetchStrategy_ErrorWhenCannotCreateCacheDirectory(t *testing.T) {
	jarFile, cleanUp := createTempZipArchive()
	defer cleanUp()

	fileBlockingExtractDirectory := filepath.Join(filepath.Dir(jarFile), "a_file_blocking_extract")

	if _, err := os.Create(fileBlockingExtractDirectory); err != nil {
		panic(err)
	}

	cacheLocation := filepath.Join(fileBlockingExtractDirectory, "cache_file.jar")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.RequestURI, ".sha256") {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		bytes, err := os.ReadFile(jarFile)
		if err != nil {
			panic(err)
		}
		if _, err := w.Write(bytes); err != nil {
			panic(err)
		}
	}))

	defer server.Close()

	remoteFetchStrategy := defaultRemoteFetchStrategy(server.URL+"/maven2",
		testVersionStrategy(),
		func() (s string, b bool) {
			return cacheLocation, false
		})

	err := remoteFetchStrategy()

	assert.Regexp(t, "^unable to extract postgres archive:.+$", err)
}

func Test_defaultRemoteFetchStrategy_ErrorWhenCannotCreateSubArchiveFile(t *testing.T) {
	jarFile, cleanUp := createTempZipArchive()
	defer cleanUp()

	cacheLocation := filepath.Join(filepath.Dir(jarFile), "extract_directory", "cache_file.jar")

	if err := os.MkdirAll(cacheLocation, os.ModePerm); err != nil {
		panic(err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.RequestURI, ".sha256") {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		bytes, err := os.ReadFile(jarFile)
		if err != nil {
			panic(err)
		}
		if _, err := w.Write(bytes); err != nil {
			panic(err)
		}
	}))
	defer server.Close()

	remoteFetchStrategy := defaultRemoteFetchStrategy(server.URL+"/maven2",
		testVersionStrategy(),
		func() (s string, b bool) {
			return "/\\000", false
		})

	err := remoteFetchStrategy()

	assert.Regexp(t, "^unable to extract postgres archive:.+$", err)
}

func Test_defaultRemoteFetchStrategy_ErrorWhenSHA256NotMatch(t *testing.T) {
	jarFile, cleanUp := createTempZipArchive()
	defer cleanUp()

	cacheLocation := filepath.Join(filepath.Dir(jarFile), "extract_location", "cache.jar")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bytes, err := os.ReadFile(jarFile)
		if err != nil {
			panic(err)
		}

		if strings.HasSuffix(r.RequestURI, ".sha256") {
			w.WriteHeader(200)
			if _, err := w.Write([]byte("literallyN3verGonnaWork")); err != nil {
				panic(err)
			}

			return
		}

		if _, err := w.Write(bytes); err != nil {
			panic(err)
		}
	}))
	defer server.Close()

	remoteFetchStrategy := defaultRemoteFetchStrategy(server.URL+"/maven2",
		testVersionStrategy(),
		func() (s string, b bool) {
			return cacheLocation, false
		})

	err := remoteFetchStrategy()

	assert.EqualError(t, err, "downloaded checksums do not match")
}

func Test_defaultRemoteFetchStrategy(t *testing.T) {
	jarFile, cleanUp := createTempZipArchive()
	defer cleanUp()

	cacheLocation := filepath.Join(filepath.Dir(jarFile), "extract_location", "cache.jar")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bytes, err := os.ReadFile(jarFile)
		if err != nil {
			panic(err)
		}

		if strings.HasSuffix(r.RequestURI, ".sha256") {
			w.WriteHeader(200)
			contentHash := sha256.Sum256(bytes)
			if _, err := w.Write([]byte(hex.EncodeToString(contentHash[:]))); err != nil {
				panic(err)
			}

			return
		}

		if _, err := w.Write(bytes); err != nil {
			panic(err)
		}
	}))
	defer server.Close()

	remoteFetchStrategy := defaultRemoteFetchStrategy(server.URL+"/maven2",
		testVersionStrategy(),
		func() (s string, b bool) {
			return cacheLocation, false
		})

	err := remoteFetchStrategy()

	assert.NoError(t, err)
	assert.FileExists(t, cacheLocation)
}

func Test_defaultRemoteFetchStrategyWithExistingDownload(t *testing.T) {
	jarFile, cleanUp := createTempZipArchive()
	defer cleanUp()

	// create a temp directory for testing
	tempFile, err := os.MkdirTemp("", "cache_output")
	if err != nil {
		panic(err)
	}
	// clean up once the test is finished.
	defer func() {
		if err := os.RemoveAll(tempFile); err != nil {
			panic(err)
		}
	}()

	cacheLocation := path.Join(tempFile, "temp.jar")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bytes, err := os.ReadFile(jarFile)
		if err != nil {
			panic(err)
		}

		if strings.HasSuffix(r.RequestURI, ".sha256") {
			w.WriteHeader(200)
			contentHash := sha256.Sum256(bytes)
			if _, err := w.Write([]byte(hex.EncodeToString(contentHash[:]))); err != nil {
				panic(err)
			}

			return
		}

		if _, err := w.Write(bytes); err != nil {
			panic(err)
		}
	}))
	defer server.Close()

	remoteFetchStrategy := defaultRemoteFetchStrategy(server.URL+"/maven2",
		testVersionStrategy(),
		func() (s string, b bool) {
			return cacheLocation, false
		})

	// call it the remoteFetchStrategy(). The output location should be empty and a new file created
	err = remoteFetchStrategy()
	assert.NoError(t, err)
	assert.FileExists(t, cacheLocation)
	out1, err := os.ReadFile(cacheLocation)

	// write some bad data to the file, this helps us test that the file is overwritten
	err = os.WriteFile(cacheLocation, []byte("invalid"), 0600)
	assert.NoError(t, err)

	// call the remoteFetchStrategy() again, this time the file should be overwritten
	err = remoteFetchStrategy()
	assert.NoError(t, err)
	assert.FileExists(t, cacheLocation)

	// ensure that the file contents are the same from both downloads, and that it doesn't contain the `invalid` data.
	out2, err := os.ReadFile(cacheLocation)
	assert.Equal(t, out1, out2)
}

func Test_defaultRemoteFetchStrategy_whenContentLengthNotSet(t *testing.T) {
	jarFile, cleanUp := createTempZipArchive()
	defer cleanUp()

	cacheLocation := filepath.Join(filepath.Dir(jarFile), "extract_location", "cache.jar")

	bytes, err := os.ReadFile(jarFile)
	if err != nil {
		require.NoError(t, err)
	}
	contentHash := sha256.Sum256(bytes)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.RequestURI, ".sha256") {
			w.WriteHeader(200)
			if _, err := w.Write([]byte(hex.EncodeToString(contentHash[:]))); err != nil {
				panic(err)
			}

			return
		}

		f, err := os.Open(jarFile)
		if err != nil {
			panic(err)
		}

		// stream the file back so that Go uses
		// chunked encoding and never sets Content-Length
		_, _ = io.Copy(w, f)
	}))
	defer server.Close()

	remoteFetchStrategy := defaultRemoteFetchStrategy(server.URL+"/maven2",
		testVersionStrategy(),
		func() (s string, b bool) {
			return cacheLocation, false
		})

	err = remoteFetchStrategy()

	assert.NoError(t, err)
	assert.FileExists(t, cacheLocation)
}

func Test_closeBody_NilResponse(t *testing.T) {
	assert.NotPanics(t, func() {
		closeBody(nil)()
	})
}
