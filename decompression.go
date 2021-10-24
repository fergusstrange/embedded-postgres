package embeddedpostgres

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/xi2/xz"
)

func decompressTarXz(path, extractPath string) error {
	tarFile, err := os.Open(path)
	if err != nil {
		return errorUnableToExtract(path, extractPath)
	}

	defer func() {
		if err := tarFile.Close(); err != nil {
			panic(err)
		}
	}()

	xzReader, err := xz.NewReader(tarFile, 0)
	if err != nil {
		return errorUnableToExtract(path, extractPath)
	}

	tarReader := tar.NewReader(xzReader)

	for {
		header, err := tarReader.Next()

		if err == io.EOF {
			return nil
		}

		if err != nil {
			return errorExtractingPostgres(err)
		}

		targetPath := filepath.Join(extractPath, header.Name)

		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return errorExtractingPostgres(err)
		}

		switch header.Typeflag {
		case tar.TypeReg:
			outFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return errorExtractingPostgres(err)
			}

			if _, err := io.Copy(outFile, tarReader); err != nil {
				return errorExtractingPostgres(err)
			}

			if err := outFile.Close(); err != nil {
				return errorExtractingPostgres(err)
			}
		case tar.TypeSymlink:
			if err := os.RemoveAll(targetPath); err != nil {
				return errorExtractingPostgres(err)
			}

			if err := os.Symlink(header.Linkname, targetPath); err != nil {
				return errorExtractingPostgres(err)
			}
		}
	}
}

func errorUnableToExtract(cacheLocation, binariesPath string) error {
	return fmt.Errorf("unable to extract postgres archive %s to %s, if running parallel tests, configure RuntimePath to isolate testing directories", cacheLocation, binariesPath)
}
