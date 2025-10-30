package main

import (
	"strings"
)

// formatSection properly indents a text section.
func formatSection(header string, content string) string {
	var out strings.Builder

	// Add section header
	if header != "" {
		_, _ = out.WriteString(header + ":\n")
	}

	// Indent the content
	for line := range strings.SplitSeq(content, "\n") {
		if line != "" {
			_, _ = out.WriteString("  ")
		}

		_, _ = out.WriteString(line + "\n")
	}

	if header != "" {
		// Section separator (when rendering a full section
		_, _ = out.WriteString("\n")

		return out.String()
	}

	// Remove last newline when rendering partial section
	return strings.TrimSuffix(out.String(), "\n")
}
