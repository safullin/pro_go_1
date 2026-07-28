package buildinfo

import (
	"strings"
	"testing"
)

func TestPrint(t *testing.T) {
	tests := []struct {
		name    string
		version string
		date    string
		commit  string
		want    string
	}{
		{
			name:    "values",
			version: "v1.0.0",
			date:    "2026-07-29",
			commit:  "abcdef",
			want: "Build version: v1.0.0\n" +
				"Build date: 2026-07-29\n" +
				"Build commit: abcdef\n",
		},
		{
			name: "empty values",
			want: "Build version: N/A\n" +
				"Build date: N/A\n" +
				"Build commit: N/A\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var output strings.Builder

			Print(&output, test.version, test.date, test.commit)

			if output.String() != test.want {
				t.Errorf("Print() = %q, want %q", output.String(), test.want)
			}
		})
	}
}
