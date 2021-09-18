package embeddedpostgres

import (
	"bufio"
	"io"
	"io/ioutil"
	"log"
	"os"
)

type syncedLogger struct {
	logger io.Writer
	file   *os.File
}

func newSyncedLogger(logger io.Writer) (*syncedLogger, error) {
	file, err := ioutil.TempFile("", "embedded_postgres_log")
	if err != nil {
		return nil, err
	}

	s := syncedLogger{
		logger: logger,
		file:   file,
	}

	if logger != nil {
		go syncLogFileAndCustomWriter(file, logger)
	}

	return &s, nil
}

func syncLogFileAndCustomWriter(file *os.File, logger io.Writer) {
	offset := int64(0)

	for {
		fileToCopy, err := os.Open(file.Name())
		if err != nil {
			log.Print(err)
			break
		}

		if _, err := fileToCopy.Seek(offset, io.SeekStart); err != nil {
			log.Print(err)
			break
		}

		reader := bufio.NewReader(fileToCopy)

		line, err := reader.ReadBytes('\n')
		if err != nil && err != io.EOF {
			log.Print(err)
			break
		}

		offset += int64(len(line))

		if len(line) != 0 {
			if _, writeErr := logger.Write(line); writeErr != nil {
				log.Print(err)
				break
			}
		}

		if err := fileToCopy.Close(); err != nil {
			log.Print(err)
			break
		}
	}
}
