package embeddedpostgres

import (
	"archive/tar"
	"io"
	"os"
	"path/filepath"

	"github.com/xi2/xz"
)

//nolint:funlen,gocognit
func unTar(path, extractPath string) error {
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

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, 0755); err != nil {
				return errorExtractingPostgres(err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return errorExtractingPostgres(err)
			}

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
			err := os.MkdirAll(filepath.Dir(targetPath), 0755)
			if err != nil {
				return errorExtractingPostgres(err)
			}

			_, err = os.Lstat(targetPath)

			if err == nil {
				if err := os.Remove(targetPath); err != nil {
					return errorExtractingPostgres(err)
				}
			}

			if err := os.Symlink(header.Linkname, targetPath); err != nil {
				return errorExtractingPostgres(err)
			}
		default:
			return errorExtractingPostgres(err)
		}
	}
}

// Add this soon
/*func checkPath(to, filename string) error {
	to, _ = filepath.Abs(to)
	dest := filepath.Join(to, filename)
	if !strings.HasPrefix(dest, to) {
		return errors.New("illegal path traversal")
	}
	return nil
}*/
