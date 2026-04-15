package logger

import (
	"os"

	"github.com/rs/zerolog"
)

// New создаёт логгер приложения.
func New() zerolog.Logger {
	return zerolog.New(os.Stdout).Level(zerolog.InfoLevel).With().Timestamp().Logger()
}
