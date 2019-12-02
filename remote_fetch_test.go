package embeddedpostgres

import (
	"github.com/mholt/archiver"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func Test_defaultRemoteFetchStrategy_ErrorWhenHttpGet(t *testing.T) {
	remoteFetchStrategy := defaultRemoteFetchStrategy("http://localhost:1234",
		func() (s string, s2 string, version PostgresVersion) {
			return "1", "", "123"
		},
		func() (s string, b bool) {
			return "", false
		})

	err := remoteFetchStrategy()

	assert.EqualError(t, err, "unable to connect to http://localhost:1234")
}

func Test_defaultRemoteFetchStrategy_ErrorWhenHttpStatusNot200(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	remoteFetchStrategy := defaultRemoteFetchStrategy(server.URL,
		func() (s string, s2 string, version PostgresVersion) {
			return "1", "", "123"
		},
		func() (s string, b bool) {
			return "", false
		})

	err := remoteFetchStrategy()

	assert.EqualError(t, err, "no version found matching 123")
}

func Test_defaultRemoteFetchStrategy_ErrorWhenBodyReadIssue(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1")
	}))
	defer server.Close()

	remoteFetchStrategy := defaultRemoteFetchStrategy(server.URL,
		func() (s string, s2 string, version PostgresVersion) {
			return "1", "", "123"
		},
		func() (s string, b bool) {
			return "", false
		})

	err := remoteFetchStrategy()

	assert.EqualError(t, err, "error fetching postgres: unexpected EOF")
}

func Test_defaultRemoteFetchStrategy_ErrorWhenCannotUnzipSubfile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

	}))
	defer server.Close()

	remoteFetchStrategy := defaultRemoteFetchStrategy(server.URL,
		func() (s string, s2 string, version PostgresVersion) {
			return "1", "", "123"
		},
		func() (s string, b bool) {
			return "", false
		})

	err := remoteFetchStrategy()

	assert.EqualError(t, err, "error fetching postgres: creating reader: zip: not a valid zip file")
}

func Test_defaultRemoteFetchStrategy_ErrorWhenCannotUnzip(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write([]byte("lolz")); err != nil {
			panic(err)
		}
	}))
	defer server.Close()

	remoteFetchStrategy := defaultRemoteFetchStrategy(server.URL,
		func() (s string, s2 string, version PostgresVersion) {
			return "1", "", "123"
		},
		func() (s string, b bool) {
			return "", false
		})

	err := remoteFetchStrategy()

	assert.EqualError(t, err, "error fetching postgres: creating reader: zip: not a valid zip file")
}

func Test_defaultRemoteFetchStrategy_ErrorWhenNoSubTarArchive(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		zip := archiver.NewZip()
		defer func() {
			if err := zip.Close(); err != nil {
				panic(err)
			}
		}()
		if err := zip.Create(w); err != nil {
			panic(err)
		}
	}))
	defer server.Close()

	remoteFetchStrategy := defaultRemoteFetchStrategy(server.URL,
		func() (s string, s2 string, version PostgresVersion) {
			return "1", "darwin", "123"
		},
		func() (s string, b bool) {
			return "", false
		})

	err := remoteFetchStrategy()

	assert.EqualError(t, err, "error fetching postgres: cannot find binary in archive retrieved from "+server.URL+"/maven2/io/zonky/test/postgres/embedded-postgres-binaries-1-darwin/123/embedded-postgres-binaries-1-darwin-123.jar")
}

func Test_defaultRemoteFetchStrategy_ErrorWhenCannotExtractSubArchive(t *testing.T) {
	jarFile, cleanUp := createTempArchive()
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
		func() (s string, s2 string, version PostgresVersion) {
			return "1", "darwin", "123"
		},
		func() (s string, b bool) {
			return dirBlockingExtract, false
		})

	err := remoteFetchStrategy()

	assert.EqualError(t, err, "unable to extract postgres archive to "+dirBlockingExtract)
}

func Test_defaultRemoteFetchStrategy_ErrorWhenCannotCreateCacheDirectory(t *testing.T) {
	jarFile, cleanUp := createTempArchive()
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
		func() (s string, s2 string, version PostgresVersion) {
			return "1", "darwin", "123"
		},
		func() (s string, b bool) {
			return cacheLocation, false
		})

	err := remoteFetchStrategy()

	assert.EqualError(t, err, "unable to extract postgres archive to "+cacheLocation)
}

func Test_defaultRemoteFetchStrategy_ErrorWhenCannotCreateSubArchiveFile(t *testing.T) {
	jarFile, cleanUp := createTempArchive()
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
		func() (s string, s2 string, version PostgresVersion) {
			return "1", "darwin", "123"
		},
		func() (s string, b bool) {
			return cacheLocation, false
		})

	err := remoteFetchStrategy()

	assert.EqualError(t, err, "unable to extract postgres archive to "+cacheLocation)
}

func Test_defaultRemoteFetchStrategy(t *testing.T) {
	jarFile, cleanUp := createTempArchive()
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
		func() (s string, s2 string, version PostgresVersion) {
			return "1", "darwin", "123"
		},
		func() (s string, b bool) {
			return cacheLocation, false
		})

	err := remoteFetchStrategy()

	assert.NoError(t, err)
	assert.FileExists(t, cacheLocation)
}

func createTempArchive() (string, func()) {
	tempDir, err := ioutil.TempDir("", "remote_fetch_test")
	if err != nil {
		panic(err)
	}
	tempFile, err := ioutil.TempFile(tempDir, "remote_fetch_test")
	if err != nil {
		panic(err)
	}
	tarFile := filepath.Join(tempDir, "remote_fetch_test.txz")
	if err := archiver.NewTarXz().Archive([]string{tempFile.Name()}, tarFile); err != nil {
		panic(err)
	}
	jarFile := filepath.Join(tempDir, "remote_fetch_test.zip")
	if err := archiver.NewZip().Archive([]string{tempFile.Name(), tarFile}, jarFile); err != nil {
		panic(err)
	}
	return jarFile, func() {
		if err := os.RemoveAll(tempDir); err != nil {
			panic(err)
		}
	}
}
