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
	offset := 0

	for {
		fileToCopy, err := os.Open(file.Name())
		if err != nil {
			log.Fatal(err)
		}

		reader := bufio.NewReader(fileToCopy)
		if _, err := reader.Discard(offset); err != nil {
			log.Fatal(err)
		}

		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				continue
			}

			log.Fatal(err)
		}

		offset += len(line)

		if len(line) != 0 {
			if _, writeErr := logger.Write(line); writeErr != nil {
				log.Fatal(writeErr)
			}
		}
	}
}
