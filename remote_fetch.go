package embeddedpostgres

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/fergusstrange/embedded-postgres/atomic"
)

// RemoteFetchStrategy provides a strategy to fetch a Postgres binary so that it is available for use.
type RemoteFetchStrategy func() error

//nolint:funlen
func defaultRemoteFetchStrategy(remoteFetchHost string, versionStrategy VersionStrategy, cacheLocator CacheLocator) RemoteFetchStrategy {
	return func() error {
		operatingSystem, architecture, version := versionStrategy()

		jarDownloadURL := fmt.Sprintf("%s/io/zonky/test/postgres/embedded-postgres-binaries-%s-%s/%s/embedded-postgres-binaries-%s-%s-%s.jar",
			remoteFetchHost,
			operatingSystem,
			architecture,
			version,
			operatingSystem,
			architecture,
			version)

		jarDownloadResponse, err := http.Get(jarDownloadURL)
		if err != nil {
			return fmt.Errorf("unable to connect to %s", remoteFetchHost)
		}

		defer closeBody(jarDownloadResponse)()

		if jarDownloadResponse.StatusCode != http.StatusOK {
			return fmt.Errorf("no version found matching %s", version)
		}

		jarBodyBytes, err := io.ReadAll(jarDownloadResponse.Body)
		if err != nil {
			return errorFetchingPostgres(err)
		}

		shaDownloadURL := fmt.Sprintf("%s.sha256", jarDownloadURL)
		shaDownloadResponse, err := http.Get(shaDownloadURL)

		defer closeBody(shaDownloadResponse)()

		if err == nil && shaDownloadResponse.StatusCode == http.StatusOK {
			if shaBodyBytes, err := io.ReadAll(shaDownloadResponse.Body); err == nil {
				jarChecksum := sha256.Sum256(jarBodyBytes)
				if !bytes.Equal(shaBodyBytes, []byte(hex.EncodeToString(jarChecksum[:]))) {
					return errors.New("downloaded checksums do not match")
				}
			}
		}

		return decompressResponse(jarBodyBytes, jarDownloadResponse.ContentLength, cacheLocator, jarDownloadURL)
	}
}

func closeBody(resp *http.Response) func() {
	return func() {
		if err := resp.Body.Close(); err != nil {
			log.Fatal(err)
		}
	}
}

func decompressResponse(bodyBytes []byte, contentLength int64, cacheLocator CacheLocator, downloadURL string) error {
	zipReader, err := zip.NewReader(bytes.NewReader(bodyBytes), contentLength)
	if err != nil {
		return errorFetchingPostgres(err)
	}

	for _, file := range zipReader.File {
		if !file.FileHeader.FileInfo().IsDir() && strings.HasSuffix(file.FileHeader.Name, ".txz") {
			archiveReader, err := file.Open()
			if err != nil {
				return errorExtractingPostgres(err)
			}

			archiveBytes, err := io.ReadAll(archiveReader)
			if err != nil {
				return errorExtractingPostgres(err)
			}

			// to prevent file corruption when multiple processes attempt to extract at the same time we download
			// first to a cache location, and then move the file into place.
			tmp, err := os.CreateTemp("", "embedded_postgres")
			if err != nil {
				return errorExtractingPostgres(err)
			}

			if err := os.WriteFile(tmp.Name(), archiveBytes, file.FileHeader.Mode()); err != nil {
				return errorExtractingPostgres(err)
			}

			cacheLocation, _ := cacheLocator()
			if err := os.MkdirAll(filepath.Dir(cacheLocation), os.ModePerm); err != nil {
				return errorExtractingPostgres(err)
			}

			if err := atomic.ReplaceFile(tmp.Name(), cacheLocation); err != nil {
				if strings.Contains(err.Error(), "The process cannot access the file because it is being used by another process") {
					fmt.Printf("WARN: %v\n", err)
					return nil
				}

				return errorExtractingPostgres(err)
			}

			return nil
		}
	}

	return fmt.Errorf("error fetching postgres: cannot find binary in archive retrieved from %s", downloadURL)
}

func errorExtractingPostgres(err error) error {
	return fmt.Errorf("unable to extract postgres archive: %s", err)
}

func errorFetchingPostgres(err error) error {
	return fmt.Errorf("error fetching postgres: %s", err)
}
