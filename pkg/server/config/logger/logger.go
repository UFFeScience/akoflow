package logger

import (
	"io"
	"log"
	"os"
)

type Logger struct {
	file   *os.File
	logger *log.Logger
}

func NewStdLogger() *Logger {
	return &Logger{
		logger: log.New(os.Stdout, "", log.Ldate|log.Ltime|log.Lshortfile),
	}
}

func NewLogger(filePath string) (*Logger, error) {

	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}

	multiWriter := io.MultiWriter(os.Stdout, file)
	logger := log.New(multiWriter, "", log.Ldate|log.Ltime|log.Lshortfile)

	return &Logger{
		file:   file,
		logger: logger,
	}, nil

}

func (l *Logger) Info(v ...interface{}) {
	l.logger.SetPrefix("INFO: ")
	l.logger.Println(v...)
}

func (l *Logger) Infof(format string, v ...interface{}) {
	l.logger.SetPrefix("INFO: ")
	l.logger.Printf(format, v...)
}

func (l *Logger) Warning(v ...interface{}) {
	l.logger.SetPrefix("WARNING: ")
	l.logger.Println(v...)
}

func (l *Logger) Error(v ...interface{}) {
	l.logger.SetPrefix("ERROR: ")
	l.logger.Println(v...)
}

func (l *Logger) Close() error {
	return l.file.Close()
}
