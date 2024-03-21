package embeddedpostgres

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/ulikunitz/xz"
)

func defaultTarReader(xzReader *xz.Reader) (func() (*tar.Header, error), func() io.Reader) {
	tarReader := tar.NewReader(xzReader)

	return func() (*tar.Header, error) {
			return tarReader.Next()
		}, func() io.Reader {
			return tarReader
		}
}

func decompressTarXz(tarReader func(*xz.Reader) (func() (*tar.Header, error), func() io.Reader), path, extractPath string) error {
	tempExtractPath, err := os.MkdirTemp(filepath.Dir(extractPath), "temp_")
	if err != nil {
		return errorUnableToExtract(path, extractPath, err)
	}
	defer func() {
		if err := os.RemoveAll(tempExtractPath); err != nil {
			panic(err)
		}
	}()

	tarFile, err := os.Open(path)
	if err != nil {
		return errorUnableToExtract(path, extractPath, err)
	}

	defer func() {
		if err := tarFile.Close(); err != nil {
			panic(err)
		}
	}()

	xzReader, err := xz.NewReader(tarFile)
	if err != nil {
		return errorUnableToExtract(path, extractPath, err)
	}

	readNext, reader := tarReader(xzReader)

	for {
		header, err := readNext()

		if err == io.EOF {
			break
		}

		if err != nil {
			return errorExtractingPostgres(err)
		}

		targetPath := filepath.Join(tempExtractPath, header.Name)
		finalPath := filepath.Join(extractPath, header.Name)

		if err := os.MkdirAll(filepath.Dir(targetPath), os.ModePerm); err != nil {
			return errorExtractingPostgres(err)
		}

		if err := os.MkdirAll(filepath.Dir(finalPath), os.ModePerm); err != nil {
			return errorExtractingPostgres(err)
		}

		switch header.Typeflag {
		case tar.TypeReg:
			outFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return errorExtractingPostgres(err)
			}

			if _, err := io.Copy(outFile, reader()); err != nil {
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

		case tar.TypeDir:
			if err := os.MkdirAll(finalPath, os.FileMode(header.Mode)); err != nil {
				return errorExtractingPostgres(err)
			}
			continue
		}

		if err := renameOrIgnore(targetPath, finalPath); err != nil {
			return errorExtractingPostgres(err)
		}
	}

	return nil
}

func errorUnableToExtract(cacheLocation, binariesPath string, err error) error {
	return fmt.Errorf("unable to extract postgres archive %s to %s, if running parallel tests, configure RuntimePath to isolate testing directories, %w",
		cacheLocation,
		binariesPath,
		err,
	)
}
