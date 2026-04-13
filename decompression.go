package embeddedpostgres

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/xi2/xz"
)

type progressLogger func(format string, args ...any)

func defaultTarReader(xzReader *xz.Reader) (func() (*tar.Header, error), func() io.Reader) {
	tarReader := tar.NewReader(xzReader)

	return func() (*tar.Header, error) {
			return tarReader.Next()
		}, func() io.Reader {
			return tarReader
		}
}

func decompressTarXz(tarReader func(*xz.Reader) (func() (*tar.Header, error), func() io.Reader), path, extractPath string, logf progressLogger) error {
	extractDirectory := filepath.Dir(extractPath)

	if err := os.MkdirAll(extractDirectory, os.ModePerm); err != nil {
		return errorUnableToExtract(path, extractPath, err)
	}

	tempExtractPath, err := os.MkdirTemp(extractDirectory, "temp_")
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

	xzReader, err := xz.NewReader(tarFile, 0)
	if err != nil {
		return errorUnableToExtract(path, extractPath, err)
	}

	readNext, reader := tarReader(xzReader)
	entryCount := 0

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
			logProgress(logf, "extracting embedded postgres entry archive=%s entry=%s type=file target=%s size=%d", path, header.Name, finalPath, header.Size)
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
			logProgress(logf, "extracting embedded postgres entry archive=%s entry=%s type=symlink target=%s link=%s", path, header.Name, finalPath, header.Linkname)
			if err := os.RemoveAll(targetPath); err != nil {
				return errorExtractingPostgres(err)
			}

			if err := os.Symlink(header.Linkname, targetPath); err != nil {
				return errorExtractingPostgres(err)
			}

		case tar.TypeDir:
			logProgress(logf, "extracting embedded postgres entry archive=%s entry=%s type=dir target=%s", path, header.Name, finalPath)
			if err := os.MkdirAll(finalPath, os.FileMode(header.Mode)); err != nil {
				return errorExtractingPostgres(err)
			}
			entryCount++
			continue
		}

		if err := renameOrIgnore(targetPath, finalPath); err != nil {
			return errorExtractingPostgres(err)
		}
		entryCount++
	}

	logProgress(logf, "finished extracting embedded postgres archive archive=%s destination=%s entries=%d", path, extractPath, entryCount)

	return nil
}

func logProgress(logf progressLogger, format string, args ...any) {
	if logf == nil {
		return
	}
	logf(format, args...)
}

func errorUnableToExtract(cacheLocation, binariesPath string, err error) error {
	return fmt.Errorf("unable to extract postgres archive %s to %s, if running parallel tests, configure RuntimePath to isolate testing directories, %w",
		cacheLocation,
		binariesPath,
		err,
	)
}
