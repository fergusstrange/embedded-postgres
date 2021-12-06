package embeddedpostgres

import (
	"archive/zip"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_defaultRemoteFetchStrategy_ErrorWhenHttpGet(t *testing.T) {
	defer verifyLeak(t)

	remoteFetchStrategy := defaultRemoteFetchStrategy("http://localhost:1234",
		testVersionStrategy(),
		testCacheLocator())

	err := remoteFetchStrategy()

	assert.EqualError(t, err, "unable to connect to http://localhost:1234")
}

func Test_defaultRemoteFetchStrategy_ErrorWhenHttpStatusNot200(t *testing.T) {
	defer verifyLeak(t)

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
	defer verifyLeak(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1")
	}))
	defer server.Close()

	remoteFetchStrategy := defaultRemoteFetchStrategy(server.URL,
		testVersionStrategy(),
		testCacheLocator())

	err := remoteFetchStrategy()

	assert.EqualError(t, err, "error fetching postgres: unexpected EOF")
}

func Test_defaultRemoteFetchStrategy_ErrorWhenCannotUnzipSubFile(t *testing.T) {
	defer verifyLeak(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

	}))
	defer server.Close()

	remoteFetchStrategy := defaultRemoteFetchStrategy(server.URL,
		testVersionStrategy(),
		testCacheLocator())

	err := remoteFetchStrategy()

	assert.EqualError(t, err, "error fetching postgres: zip: not a valid zip file")
}

func Test_defaultRemoteFetchStrategy_ErrorWhenCannotUnzip(t *testing.T) {
	defer verifyLeak(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write([]byte("lolz")); err != nil {
			panic(err)
		}
	}))
	defer server.Close()

	remoteFetchStrategy := defaultRemoteFetchStrategy(server.URL,
		testVersionStrategy(),
		testCacheLocator())

	err := remoteFetchStrategy()

	assert.EqualError(t, err, "error fetching postgres: zip: not a valid zip file")
}

func Test_defaultRemoteFetchStrategy_ErrorWhenNoSubTarArchive(t *testing.T) {
	defer verifyLeak(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		MyZipWriter := zip.NewWriter(w)

		if err := MyZipWriter.Close(); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()

	remoteFetchStrategy := defaultRemoteFetchStrategy(server.URL,
		testVersionStrategy(),
		testCacheLocator())

	err := remoteFetchStrategy()

	assert.EqualError(t, err, "error fetching postgres: cannot find binary in archive retrieved from "+server.URL+"/maven2/io/zonky/test/postgres/embedded-postgres-binaries-darwin-amd64/1.2.3/embedded-postgres-binaries-darwin-amd64-1.2.3.jar")
}

func Test_defaultRemoteFetchStrategy_ErrorWhenCannotExtractSubArchive(t *testing.T) {
	defer verifyLeak(t)

	jarFile, cleanUp := createTempZipArchive()
	defer cleanUp()

	dirBlockingExtract := filepath.Join(filepath.Dir(jarFile), "some_dir")

	if err := os.MkdirAll(dirBlockingExtract, 0400); err != nil {
		panic(err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bytes, err := ioutil.ReadFile(jarFile)
		if err != nil {
			panic(err)
		}
		if _, err := w.Write(bytes); err != nil {
			panic(err)
		}
	}))
	defer server.Close()

	remoteFetchStrategy := defaultRemoteFetchStrategy(server.URL,
		testVersionStrategy(),
		func() (s string, b bool) {
			return dirBlockingExtract, false
		})

	err := remoteFetchStrategy()

	assert.Regexp(t, "^unable to extract postgres archive:.+$", err)
}

func Test_defaultRemoteFetchStrategy_ErrorWhenCannotCreateCacheDirectory(t *testing.T) {
	defer verifyLeak(t)

	jarFile, cleanUp := createTempZipArchive()
	defer cleanUp()

	fileBlockingExtractDirectory := filepath.Join(filepath.Dir(jarFile), "a_file_blocking_extract")

	if _, err := os.Create(fileBlockingExtractDirectory); err != nil {
		panic(err)
	}

	cacheLocation := filepath.Join(fileBlockingExtractDirectory, "cache_file.jar")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bytes, err := ioutil.ReadFile(jarFile)
		if err != nil {
			panic(err)
		}
		if _, err := w.Write(bytes); err != nil {
			panic(err)
		}
	}))

	defer server.Close()

	remoteFetchStrategy := defaultRemoteFetchStrategy(server.URL,
		testVersionStrategy(),
		func() (s string, b bool) {
			return cacheLocation, false
		})

	err := remoteFetchStrategy()

	assert.Regexp(t, "^unable to extract postgres archive:.+$", err)
}

func Test_defaultRemoteFetchStrategy_ErrorWhenCannotCreateSubArchiveFile(t *testing.T) {
	defer verifyLeak(t)

	jarFile, cleanUp := createTempZipArchive()
	defer cleanUp()

	cacheLocation := filepath.Join(filepath.Dir(jarFile), "extract_directory", "cache_file.jar")

	if err := os.MkdirAll(cacheLocation, 0755); err != nil {
		panic(err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bytes, err := ioutil.ReadFile(jarFile)
		if err != nil {
			panic(err)
		}
		if _, err := w.Write(bytes); err != nil {
			panic(err)
		}
	}))
	defer server.Close()

	remoteFetchStrategy := defaultRemoteFetchStrategy(server.URL,
		testVersionStrategy(),
		func() (s string, b bool) {
			return cacheLocation, false
		})

	err := remoteFetchStrategy()

	assert.Regexp(t, "^unable to extract postgres archive:.+$", err)
}

func Test_defaultRemoteFetchStrategy(t *testing.T) {
	defer verifyLeak(t)

	jarFile, cleanUp := createTempZipArchive()
	defer cleanUp()

	cacheLocation := filepath.Join(filepath.Dir(jarFile), "extract_location", "cache.jar")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bytes, err := ioutil.ReadFile(jarFile)
		if err != nil {
			panic(err)
		}
		if _, err := w.Write(bytes); err != nil {
			panic(err)
		}
	}))
	defer server.Close()

	remoteFetchStrategy := defaultRemoteFetchStrategy(server.URL,
		testVersionStrategy(),
		func() (s string, b bool) {
			return cacheLocation, false
		})

	err := remoteFetchStrategy()

	assert.NoError(t, err)
	assert.FileExists(t, cacheLocation)
}
