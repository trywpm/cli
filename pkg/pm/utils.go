package pm

import (
	"bytes"
	"strings"
)

// DetectIndentation scans the first few lines to find the indentation style.
//
// Defaults to 2 spaces if it can't decide.
func DetectIndentation(data []byte) string {
	lines := strings.SplitSeq(string(data), "\n")

	for line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			continue
		}

		var buf bytes.Buffer
		for _, r := range line {
			if r == ' ' || r == '\t' {
				buf.WriteRune(r)
			} else {
				break
			}
		}

		if buf.Len() > 0 {
			return buf.String()
		}
	}

	return "  "
}
