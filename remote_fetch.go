package embeddedpostgres

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/ioutil"
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

		downloadURL := fmt.Sprintf("%s/io/zonky/test/postgres/embedded-postgres-binaries-%s-%s/%s/embedded-postgres-binaries-%s-%s-%s.jar",
			remoteFetchHost,
			operatingSystem,
			architecture,
			version,
			operatingSystem,
			architecture,
			version)

		resp, err := http.Get(downloadURL)
		if err != nil {
			return fmt.Errorf("unable to connect to %s", remoteFetchHost)
		}

		defer closeBody(resp)()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("no version found matching %s", version)
		}

		bodyBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return errorFetchingPostgres(err)
		}

		downloadSHAURL := fmt.Sprintf("%s.sha256", downloadURL)
		shaResp, err := http.Get(downloadSHAURL)

		defer closeBody(shaResp)()

		if err == nil && shaResp.StatusCode == http.StatusOK {
			shaBytes, err := ioutil.ReadAll(shaResp.Body)
			if err == nil {
				bodyBytesHash := sha256.Sum256(bodyBytes)
				downloadedHash := hex.EncodeToString(bodyBytesHash[:])

				if !strings.EqualFold(string(shaBytes), downloadedHash) {
					return errors.New("downloaded checksums do not match")
				}
			}
		}

		return decompressResponse(bodyBytes, resp.ContentLength, cacheLocator, downloadURL)
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

			archiveBytes, err := ioutil.ReadAll(archiveReader)
			if err != nil {
				return errorExtractingPostgres(err)
			}

			cacheLocation, _ := cacheLocator()

			if err := os.MkdirAll(filepath.Dir(cacheLocation), 0755); err != nil {
				return errorExtractingPostgres(err)
			}

			if err := ioutil.WriteFile(cacheLocation, archiveBytes, file.FileHeader.Mode()); err != nil {
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
