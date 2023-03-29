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

	cacheLocation, _ := cacheLocator()

	if err := os.MkdirAll(filepath.Dir(cacheLocation), 0755); err != nil {
		return errorExtractingPostgres(err)
	}

	for _, file := range zipReader.File {
		if !file.FileHeader.FileInfo().IsDir() && strings.HasSuffix(file.FileHeader.Name, ".txz") {
			if err := decompressSingleFile(file, cacheLocation); err != nil {
				return err
			}

			// we have successfully found the file, return early
			return nil
		}
	}

	return fmt.Errorf("error fetching postgres: cannot find binary in archive retrieved from %s", downloadURL)
}

func decompressSingleFile(file *zip.File, cacheLocation string) error {
	renamed := false

	archiveReader, err := file.Open()
	if err != nil {
		return errorExtractingPostgres(err)
	}

	archiveBytes, err := io.ReadAll(archiveReader)
	if err != nil {
		return errorExtractingPostgres(err)
	}

	// if multiple processes attempt to extract
	// to prevent file corruption when multiple processes attempt to extract at the same time
	// first to a cache location, and then move the file into place.
	tmp, err := os.CreateTemp(filepath.Dir(cacheLocation), "temp_")
	if err != nil {
		return errorExtractingPostgres(err)
	}
	defer func() {
		// if anything failed before the rename then the temporary file should be cleaned up.
		// if the rename was successful then there is no temporary file to remove.
		if !renamed {
			if err := os.Remove(tmp.Name()); err != nil {
				panic(err)
			}
		}
	}()

	if _, err := tmp.Write(archiveBytes); err != nil {
		return errorExtractingPostgres(err)
	}

	// Windows cannot rename a file if is it still open.
	// The file needs to be manually closed to allow the rename to happen
	if err := tmp.Close(); err != nil {
		return errorExtractingPostgres(err)
	}

	if err := renameOrIgnore(tmp.Name(), cacheLocation); err != nil {
		return errorExtractingPostgres(err)
	}
	renamed = true

	return nil
}

func errorExtractingPostgres(err error) error {
	return fmt.Errorf("unable to extract postgres archive: %s", err)
}

func errorFetchingPostgres(err error) error {
	return fmt.Errorf("error fetching postgres: %s", err)
}
