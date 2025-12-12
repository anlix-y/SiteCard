package logger

import (
	"log"
	"os"
	"strings"
)

// Level represents logging level
type Level int

const (
	LevelError Level = iota
	LevelInfo
	LevelDebug
)

var (
	enabled bool
	level   Level = LevelError
)

// Init configures logger from environment variables.
// Supported vars:
//
//	LOG=1            -> enable at info level
//	LOG_LEVEL=debug  -> enable at debug level (info/error also supported)
func Init() {
	if os.Getenv("LOG") == "1" {
		enabled = true
		level = LevelInfo
	}
	if lv := strings.ToLower(strings.TrimSpace(os.Getenv("LOG_LEVEL"))); lv != "" {
		enabled = true
		switch lv {
		case "debug":
			level = LevelDebug
		case "info":
			level = LevelInfo
		case "error":
			level = LevelError
		case "off", "none", "0":
			enabled = false
		default:
			// unknown -> keep enabled but at error level
			level = LevelError
		}
	}
}

func Enabled() bool { return enabled }

func Debugf(format string, v ...any) {
	if !enabled || level < LevelDebug {
		return
	}
	log.Printf("[DEBUG] "+format, v...)
}

func Infof(format string, v ...any) {
	if !enabled || level < LevelInfo {
		return
	}
	log.Printf("[INFO] "+format, v...)
}

func Errorf(format string, v ...any) {
	if !enabled || level < LevelError {
		return
	}
	log.Printf("[ERROR] "+format, v...)
}
