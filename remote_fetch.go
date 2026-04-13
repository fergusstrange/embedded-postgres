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
type RemoteFetchStrategy func(progressLogger) error

var freeBSDBinaryRepositoryURL = "https://web.sintel.com.tr/downloads/siper/pg"

//nolint:funlen
func defaultRemoteFetchStrategy(remoteFetchHost string, versionStrategy VersionStrategy, cacheLocator CacheLocator) RemoteFetchStrategy {
	return func(logf progressLogger) error {
		operatingSystem, architecture, version := versionStrategy()
		cacheLocation, _ := cacheLocator()

		if freeBSDDirectDownloadURL, ok := freeBSDBundleDownloadURL(operatingSystem, architecture); ok {
			logProgress(logf, "downloading embedded postgres archive url=%s cache=%s", freeBSDDirectDownloadURL, cacheLocation)
			if err := downloadArchiveToCache(freeBSDDirectDownloadURL, cacheLocation, logf); err != nil {
				return err
			}
			return nil
		}

		jarDownloadURL := fmt.Sprintf("%s/io/zonky/test/postgres/embedded-postgres-binaries-%s-%s/%s/embedded-postgres-binaries-%s-%s-%s.jar",
			remoteFetchHost,
			operatingSystem,
			architecture,
			version,
			operatingSystem,
			architecture,
			version)
		logProgress(logf, "downloading embedded postgres bundle url=%s cache=%s", jarDownloadURL, cacheLocation)

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
		if err != nil {
			return fmt.Errorf("download sha256 from %s failed: %w", shaDownloadURL, err)
		}
		defer closeBody(shaDownloadResponse)()

		if err == nil && shaDownloadResponse.StatusCode == http.StatusOK {
			if shaBodyBytes, err := io.ReadAll(shaDownloadResponse.Body); err == nil {
				jarChecksum := sha256.Sum256(jarBodyBytes)
				if !bytes.Equal(shaBodyBytes, []byte(hex.EncodeToString(jarChecksum[:]))) {
					return errors.New("downloaded checksums do not match")
				}
			}
		}

		return decompressResponse(jarBodyBytes, jarDownloadResponse.ContentLength, cacheLocator, jarDownloadURL, logf)
	}
}

func freeBSDBundleDownloadURL(operatingSystem, architecture string) (string, bool) {
	switch {
	case operatingSystem == "freebsd13" && architecture == "amd64":
		return freeBSDBinaryRepositoryURL + "/postgres-freebsd13-x86_64.txz", true
	case operatingSystem == "freebsd14" && architecture == "amd64":
		return freeBSDBinaryRepositoryURL + "/postgres-freebsd14-x86_64.txz", true
	default:
		return "", false
	}
}

func downloadArchiveToCache(downloadURL, cacheLocation string, logf progressLogger) error {
	downloadResponse, err := http.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("unable to connect to %s", downloadURL)
	}
	defer closeBody(downloadResponse)()

	if downloadResponse.StatusCode != http.StatusOK {
		return fmt.Errorf("no version found matching archive at %s", downloadURL)
	}

	archiveBytes, err := io.ReadAll(downloadResponse.Body)
	if err != nil {
		return errorFetchingPostgres(err)
	}
	if err := os.MkdirAll(filepath.Dir(cacheLocation), 0755); err != nil {
		return errorExtractingPostgres(err)
	}
	logProgress(logf, "downloaded embedded postgres archive url=%s cache=%s bytes=%d", downloadURL, cacheLocation, len(archiveBytes))
	return writeArchiveAtomically(cacheLocation, archiveBytes)
}

func writeArchiveAtomically(cacheLocation string, archiveBytes []byte) error {
	renamed := false

	tmp, err := os.CreateTemp(filepath.Dir(cacheLocation), "temp_")
	if err != nil {
		return errorExtractingPostgres(err)
	}
	defer func() {
		if !renamed {
			if err := os.Remove(tmp.Name()); err != nil {
				panic(err)
			}
		}
	}()

	if _, err := tmp.Write(archiveBytes); err != nil {
		return errorExtractingPostgres(err)
	}
	if err := tmp.Close(); err != nil {
		return errorExtractingPostgres(err)
	}
	if err := renameOrIgnore(tmp.Name(), cacheLocation); err != nil {
		return errorExtractingPostgres(err)
	}
	renamed = true
	return nil
}

func closeBody(resp *http.Response) func() {
	return func() {
		if resp == nil || resp.Body == nil {
			return
		}
		if err := resp.Body.Close(); err != nil {
			log.Fatal(err)
		}
	}
}

func decompressResponse(bodyBytes []byte, contentLength int64, cacheLocator CacheLocator, downloadURL string, logf progressLogger) error {
	size := contentLength
	// if the content length is not set (i.e. chunked encoding),
	// we need to use the length of the bodyBytes otherwise
	// the unzip operation will fail
	if contentLength < 0 {
		size = int64(len(bodyBytes))
	}
	zipReader, err := zip.NewReader(bytes.NewReader(bodyBytes), size)
	if err != nil {
		return errorFetchingPostgres(err)
	}

	cacheLocation, _ := cacheLocator()

	if err := os.MkdirAll(filepath.Dir(cacheLocation), 0755); err != nil {
		return errorExtractingPostgres(err)
	}

	for _, file := range zipReader.File {
		if !file.FileHeader.FileInfo().IsDir() && strings.HasSuffix(file.FileHeader.Name, ".txz") {
			logProgress(logf, "writing embedded postgres archive from bundle url=%s entry=%s cache=%s", downloadURL, file.FileHeader.Name, cacheLocation)
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
	archiveReader, err := file.Open()
	if err != nil {
		return errorExtractingPostgres(err)
	}

	archiveBytes, err := io.ReadAll(archiveReader)
	if err != nil {
		return errorExtractingPostgres(err)
	}
	return writeArchiveAtomically(cacheLocation, archiveBytes)
}

func errorExtractingPostgres(err error) error {
	return fmt.Errorf("unable to extract postgres archive: %s", err)
}

func errorFetchingPostgres(err error) error {
	return fmt.Errorf("error fetching postgres: %s", err)
}
