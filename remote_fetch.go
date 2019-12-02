package embeddedpostgres

import (
	"archive/zip"
	"bytes"
	"fmt"
	"github.com/mholt/archiver"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type RemoteFetchStrategy func() error

func defaultRemoteFetchStrategy(remoteFetchHost string, versionStrategy VersionStrategy, cacheLocator CacheLocator) RemoteFetchStrategy {
	return func() error {
		operatingSystem, architecture, version := versionStrategy()
		downloadUrl := fmt.Sprintf("%s/maven2/io/zonky/test/postgres/embedded-postgres-binaries-%s-%s/%s/embedded-postgres-binaries-%s-%s-%s.jar",
			remoteFetchHost,
			operatingSystem,
			architecture,
			version,
			operatingSystem,
			architecture,
			version)
		resp, err := http.Get(downloadUrl)
		if err != nil {
			return fmt.Errorf("unable to connect to %s", remoteFetchHost)
		}
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("no version found matching %s", version)
		}
		defer func() {
			if err := resp.Body.Close(); err != nil {
				log.Fatal(resp.Body.Close())
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
				return errorExtractingBinary(downloadUrl)
			}
			if header, ok := downloadedArchive.Header.(zip.FileHeader); !ok || !strings.HasSuffix(header.Name, ".txz") {
				continue
			}
			downloadedArchiveBytes, err := ioutil.ReadAll(downloadedArchive)
			if err == nil {
				cacheLocation, _ := cacheLocator()
				if err := createArchiveFile(cacheLocation, downloadedArchiveBytes); err != nil {
					return fmt.Errorf("unable to extract postgres archive to %s", cacheLocation)
				}
				break
			}
		}

		return nil
	}
}

func errorExtractingBinary(downloadUrl string) error {
	return fmt.Errorf("error fetching postgres: cannot find binary in archive retrieved from %s", downloadUrl)
}

func errorFetchingPostgres(err error) error {
	return fmt.Errorf("error fetching postgres: %s", err)
}

func createArchiveFile(archiveLocation string, archiveBytes []byte) error {
	if err := os.MkdirAll(filepath.Dir(archiveLocation), 0755); err != nil {
		return err
	}
	filesystemArchive, err := os.Create(archiveLocation)
	defer func() {
		log.Println(archiveLocation)
		if err := filesystemArchive.Close(); err != nil {
			log.Println(err)
		}
	}()
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile(filesystemArchive.Name(), archiveBytes, 0666); err != nil {
		return err
	}
	return nil
}
