package logger

import (
	"fmt"
	"github.com/rs/zerolog"
	"gopkg.in/natefinch/lumberjack.v2"
	"io"
	"os"
	"strings"
)

type CommonLogger struct {
	Info    zerolog.Logger
	Error   zerolog.Logger
	Trace   zerolog.Logger
	Warning zerolog.Logger
	Stream  zerolog.Logger
}

type AppLogger struct {
	Http CommonLogger
	WS   CommonLogger
}

func NewLogger() *AppLogger {
	_ = os.MkdirAll("logs", 0755)

	zerolog.TimeFieldFormat = "2006-01-02 15:04:05.000"

	consoleWriter := consoleConfWriter()

	log := &AppLogger{}

	log.Http.Stream = newMultiLogger(consoleWriter, "logs/stream.log")
	log.Http.Info = newMultiLogger(consoleWriter, "logs/info.log")
	log.Http.Trace = newMultiLogger(consoleWriter, "logs/trace.log")
	log.Http.Warning = newMultiLogger(consoleWriter, "logs/warning.log")
	log.Http.Error = newMultiLogger(consoleWriter, "logs/error.log")

	log.WS.Stream = newMultiLogger(consoleWriter, "logs/ws.stream.log")
	log.WS.Info = newMultiLogger(consoleWriter, "logs/ws.info.log")
	log.WS.Trace = newMultiLogger(consoleWriter, "logs/ws.trace.log")
	log.WS.Warning = newMultiLogger(consoleWriter, "logs/ws.warning.log")
	log.WS.Error = newMultiLogger(consoleWriter, "logs/ws.error.log")

	return log
}

func newMultiLogger(console zerolog.ConsoleWriter, filepath string) zerolog.Logger {
	multi := io.MultiWriter(console, fileConsoleWriter(filepath))

	return zerolog.New(multi).With().Timestamp().Logger()
}

func consoleConfWriter() zerolog.ConsoleWriter {
	consoleWriter := zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: "2006-01-02 15:04:05.000",
		NoColor:    false,
		FormatTimestamp: func(i interface{}) string {
			return fmt.Sprintf("[%s]", i)
		},
		FormatLevel: func(i interface{}) string {
			return fmt.Sprintf("[%s]", strings.ToUpper(i.(string)))
		},
		FormatMessage: func(i interface{}) string {
			return fmt.Sprintf("%s", i)
		},
	}
	return consoleWriter
}

func fileConsoleWriter(filename string) io.Writer {
	return zerolog.ConsoleWriter{
		Out: &lumberjack.Logger{
			Filename:   filename,
			MaxSize:    5,
			MaxAge:     20,
			MaxBackups: 5,
			Compress:   true,
		},
		NoColor:    true,
		TimeFormat: "2006-01-02 15:04:05.000",
		FormatTimestamp: func(i interface{}) string {
			return fmt.Sprintf("[%s]", i)
		},
		FormatLevel: func(i interface{}) string {
			return fmt.Sprintf("[%s]", strings.ToUpper(i.(string)))
		},
		FormatMessage: func(i interface{}) string {
			return fmt.Sprintf("%s", i)
		},
		FormatFieldName: func(i interface{}) string {
			return fmt.Sprintf("%s=", i)
		},
		FormatFieldValue: func(i interface{}) string {
			return fmt.Sprintf("%v", i)
		},
	}
}
