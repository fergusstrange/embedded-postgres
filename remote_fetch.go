package embeddedpostgres

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"github.com/mholt/archiver"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type RemoteFetchStrategy func() error

func defaultRemoteFetchStrategy(versionStrategy VersionStrategy, cacheLocator CacheLocator) RemoteFetchStrategy {
	return func() error {
		operatingSystem, architecture, version := versionStrategy()
		downloadUrl := fmt.Sprintf("https://repo1.maven.org/maven2/io/zonky/test/postgres/embedded-postgres-binaries-%s-%s/%s/embedded-postgres-binaries-%s-%s-%s.jar",
			operatingSystem,
			architecture,
			version,
			operatingSystem,
			architecture,
			version)
		resp, err := http.Get(downloadUrl)
		defer func() {
			if err := resp.Body.Close(); err != nil {
				log.Fatal(resp.Body.Close())
			}
		}()
		if err != nil {
			return err
		}
		bodyBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		zipFile := archiver.NewZip()
		if err := zipFile.Open(bytes.NewReader(bodyBytes), resp.ContentLength); err != nil {
			return err
		}
		defer func() {
			if err := zipFile.Close(); err != nil {
				log.Fatal(err)
			}
		}()
		for {
			downloadedArchive, err := zipFile.Read()
			if err != nil {
				if errors.Is(err, io.EOF) {
					break
				} else {
					return err
				}
			}
			if header, ok := downloadedArchive.Header.(zip.FileHeader); !ok || !strings.HasSuffix(header.Name, ".txz") {
				continue
			}
			downloadedArchiveBytes, err := ioutil.ReadAll(downloadedArchive)
			if err != nil {
				return err
			}
			cacheLocation, _ := cacheLocator()
			if err := createArchiveFile(cacheLocation, downloadedArchiveBytes); err != nil {
				return err
			}
		}

		return nil
	}
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
