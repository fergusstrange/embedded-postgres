package embeddedpostgres

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/mholt/archiver/v3"
)

// RemoteFetchStrategy provides a strategy to fetch a Postgres binary so that it is available for use.
type RemoteFetchStrategy func() error

//nolint:funlen
func defaultRemoteFetchStrategy(remoteFetchHost string, versionStrategy VersionStrategy, cacheLocator CacheLocator) RemoteFetchStrategy {
	return func() error {
		operatingSystem, architecture, version := versionStrategy()
		downloadURL := fmt.Sprintf("%s/maven2/io/zonky/test/postgres/embedded-postgres-binaries-%s-%s/%s/embedded-postgres-binaries-%s-%s-%s.jar",
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

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("no version found matching %s", version)
		}

		defer func() {
			if err := resp.Body.Close(); err != nil {
				log.Fatal(err)
			}
		}()

		bodyBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return errorFetchingPostgres(err)
		}

		zipFile := archiver.NewZip()

		if err := zipFile.Open(bytes.NewReader(bodyBytes), resp.ContentLength); err != nil {
			return errorFetchingPostgres(err)
		}

		defer func() {
			if err := zipFile.Close(); err != nil {
				log.Fatal(err)
			}
		}()

		for {
			downloadedArchive, err := zipFile.Read()
			if err != nil {
				return errorExtractingBinary(downloadURL)
			}

			if header, ok := downloadedArchive.Header.(zip.FileHeader); !ok || !strings.HasSuffix(header.Name, ".txz") {
				continue
			}

			downloadedArchiveBytes, err := ioutil.ReadAll(downloadedArchive)
			if err == nil {
				cacheLocation, _ := cacheLocator()

				if err := CreateArchiveFile(cacheLocation, downloadedArchiveBytes); err != nil {
					return fmt.Errorf("unable to extract postgres archive to %s", cacheLocation)
				}

				break
			}
		}

		return nil
	}
}

func errorExtractingBinary(downloadURL string) error {
	return fmt.Errorf("error fetching postgres: cannot find binary in archive retrieved from %s", downloadURL)
}

func errorFetchingPostgres(err error) error {
	return fmt.Errorf("error fetching postgres: %s", err)
}

func CreateArchiveFile(archiveLocation string, archiveBytes []byte) error {
	if err := os.MkdirAll(filepath.Dir(archiveLocation), 0755); err != nil {
		return err
	}

	if err := ioutil.WriteFile(archiveLocation, archiveBytes, 0666); err != nil {
		return err
	}

	return nil
}
