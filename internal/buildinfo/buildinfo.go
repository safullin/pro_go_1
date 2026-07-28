// Package buildinfo formats application build metadata.
package buildinfo

import (
	"fmt"
	"io"
)

const unavailable = "N/A"

// Print writes build metadata to w.
func Print(w io.Writer, version, date, commit string) {
	fmt.Fprintf(
		w,
		"Build version: %s\nBuild date: %s\nBuild commit: %s\n",
		value(version),
		value(date),
		value(commit),
	)
}

func value(buildValue string) string {
	if buildValue == "" {
		return unavailable
	}
	return buildValue
}
