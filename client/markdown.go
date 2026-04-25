package client

import (
	"fmt"
	"regexp"
	"strings"
)

// ANSI escape codes for terminal formatting.
const (
	ansiBold       = "\033[1m"
	ansiItalic     = "\033[3m"
	ansiReset      = "\033[0m"
	ansiUnderline  = "\033[4m"
	ansiNoUnderline = "\033[24m"
	ansiGrey       = "\033[90m"
	ansiBrightBlue = "\033[94m"
	ansiCyan       = "\033[96m"
	ansiStrike     = "\033[1;2m" // bold + dim (approximation of strikethrough)
	ansiGreen      = "\033[32m"
)

// FormatMarkdown converts markdown to ANSI-formatted text for terminal display.
func FormatMarkdown(input string) string {
	lines := strings.Split(input, "\n")
	var result []string
	i := 0
	inCodeBlock := false
	var codeBlockLines []string

	for i < len(lines) {
		line := lines[i]

		// Handle code blocks
		if strings.HasPrefix(line, "```") {
			if inCodeBlock {
				if len(codeBlockLines) > 0 {
					result = append(result, ansiGrey+strings.Join(codeBlockLines, "\n")+ansiReset)
				}
				codeBlockLines = nil
				inCodeBlock = false
			} else {
				inCodeBlock = true
			}
			i++
			continue
		}
		if inCodeBlock {
			codeBlockLines = append(codeBlockLines, line)
			i++
			continue
		}

		// Handle tables
		if isTableRow(line) {
			tableLines := []string{line}
			i++
			for i < len(lines) && isTableRow(lines[i]) {
				tableLines = append(tableLines, lines[i])
				i++
			}
			result = append(result, formatTable(tableLines)...)
			continue
		}

		result = append(result, formatInline(line))
		i++
	}

	// Handle unclosed code block
	if inCodeBlock && len(codeBlockLines) > 0 {
		result = append(result, ansiGrey+strings.Join(codeBlockLines, "\n")+ansiReset)
	}

	return strings.Join(result, "\n")
}

// isTableRow checks if a line looks like a table row (contains at least 2 pipes).
func isTableRow(line string) bool {
	return strings.Count(line, "|") >= 2
}

// stripANSI removes ANSI escape codes from a string and returns the visible length.
func stripANSI(s string) int {
	ansiRe := regexp.MustCompile(`\033\[[0-9;]*m`)
	stripped := ansiRe.ReplaceAllString(s, "")
	return len(stripped)
}

// formatTable formats a series of table rows with aligned columns and bold headers.
func formatTable(lines []string) []string {
	if len(lines) == 0 {
		return nil
	}

	// Parse rows into cells, keeping separator rows
	type row struct {
		cells   []string
		isSep   bool
		rawLine string
	}
	var rows []row
	for _, line := range lines {
		if isSeparatorRow(line) {
			rows = append(rows, row{isSep: true, rawLine: line})
			continue
		}
		cells := parseTableRow(line)
		rows = append(rows, row{cells: cells})
	}

	// Determine number of columns from data rows
	numCols := 0
	for _, r := range rows {
		if !r.isSep && len(r.cells) > numCols {
			numCols = len(r.cells)
		}
	}

	// Format cells with inline markdown
	for i := range rows {
		if !rows[i].isSep {
			for j := range rows[i].cells {
				rows[i].cells[j] = formatInline(rows[i].cells[j])
			}
		}
	}

	// Calculate column widths based on visible (ANSI-stripped) length
	widths := make([]int, numCols)
	for _, r := range rows {
		if r.isSep {
			continue
		}
		for i, cell := range r.cells {
			if i < len(widths) {
				vlen := stripANSI(cell)
				if vlen > widths[i] {
					widths[i] = vlen
				}
			}
		}
	}

	// Format rows with alignment
	var result []string
	for i, r := range rows {
		if r.isSep {
			// Rebuild separator row to match padded widths
			result = append(result, rebuildSeparator(r.rawLine, widths))
			continue
		}
		var cells []string
		for j, cell := range r.cells {
			if j < len(widths) {
				vlen := stripANSI(cell)
				pad := widths[j] - vlen
				if pad > 0 {
					cell = cell + strings.Repeat(" ", pad)
				}
			}
			cells = append(cells, cell)
		}

		formatted := "| " + strings.Join(cells, " | ") + " |"

		// Bold header row (first row)
		if i == 0 {
			formatted = ansiBold + formatted + ansiReset
		}

		result = append(result, formatted)
	}

	return result
}

// rebuildSeparator reconstructs a separator row with padding that matches the given column widths.
func rebuildSeparator(rawLine string, widths []int) string {
	// Parse the separator to get the number of columns
	line := strings.TrimSpace(rawLine)
	if strings.HasPrefix(line, "|") {
		line = line[1:]
	}
	if strings.HasSuffix(line, "|") {
		line = line[:len(line)-1]
	}
	parts := strings.Split(line, "|")

	var cells []string
	for i, part := range parts {
		part = strings.TrimSpace(part)
		// Determine the separator style for this cell
		var sep string
		if strings.Contains(part, ":") {
			// Contains alignment marker (e.g., ":-:", "-:", ":-")
			sep = ":"
		} else {
			sep = "-"
		}
		// Calculate the length of the separator content (excluding colons)
		sepLen := 0
		for _, ch := range part {
			if ch != '-' && ch != ':' {
				continue
			}
			sepLen++
		}
		if i < len(widths) {
			sepLen = widths[i]
		}
		cells = append(cells, strings.Repeat(sep, sepLen))
	}

	return "| " + strings.Join(cells, " | ") + " |"
}

// isSeparatorRow checks if a line is a table separator (e.g., |------|).
func isSeparatorRow(line string) bool {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "|") && !strings.HasSuffix(line, "|") {
		return false
	}
	// Remove leading/trailing pipes and split
	if strings.HasPrefix(line, "|") {
		line = line[1:]
	}
	if strings.HasSuffix(line, "|") {
		line = line[:len(line)-1]
	}
	// Check if all cells are only dashes, colons, or underscores
	cells := strings.Split(line, "|")
	for _, cell := range cells {
		cell = strings.TrimSpace(cell)
		if cell == "" {
			continue
		}
		valid := true
		for _, ch := range cell {
			if ch != '-' && ch != ':' && ch != '_' {
				valid = false
				break
			}
		}
		if !valid {
			return false
		}
	}
	return true
}

// parseTableRow splits a table row into cells.
func parseTableRow(line string) []string {
	// Remove leading and trailing pipes
	line = strings.TrimSpace(line)
	if strings.HasPrefix(line, "|") {
		line = line[1:]
	}
	if strings.HasSuffix(line, "|") {
		line = line[:len(line)-1]
	}

	parts := strings.Split(line, "|")
	var cells []string
	for _, p := range parts {
		cells = append(cells, strings.TrimSpace(p))
	}
	return cells
}

// formatInline applies inline markdown formatting to a single line.
func formatInline(line string) string {
	type codeSpan struct{ content string }
	var codes []codeSpan

	// Handle double backticks first
	codeRe := regexp.MustCompile("``([^`]+)``")
	result := codeRe.ReplaceAllStringFunc(line, func(match string) string {
		content := match[2 : len(match)-2]
		codes = append(codes, codeSpan{content: content})
		idx := len(codes) - 1
		return fmt.Sprintf("\x00CODE%d\x00", idx)
	})

	// Handle single backticks
	codeRe = regexp.MustCompile("`([^`]+)`")
	result = codeRe.ReplaceAllStringFunc(result, func(match string) string {
		content := match[1 : len(match)-1]
		codes = append(codes, codeSpan{content: content})
		idx := len(codes) - 1
		return fmt.Sprintf("\x00CODE%d\x00", idx)
	})

	// Apply strikethrough: ~~text~~
	result = regexp.MustCompile(`~~(.+?)~~`).ReplaceAllStringFunc(result, func(m string) string {
		return ansiStrike + m[2:len(m)-2] + ansiReset
	})

	// Apply bold: **text** or __text__
	result = regexp.MustCompile(`\*\*(.+?)\*\*`).ReplaceAllStringFunc(result, func(m string) string {
		return ansiBold + m[2:len(m)-2] + ansiReset
	})
	result = regexp.MustCompile(`__(.+?)__`).ReplaceAllStringFunc(result, func(m string) string {
		return ansiBold + m[2:len(m)-2] + ansiReset
	})

	// Apply italic: pair up remaining single * characters
	result = italicAsterisks(result)

	// Apply italic: _text_
	result = regexp.MustCompile(`_(.+?)_`).ReplaceAllStringFunc(result, func(m string) string {
		return ansiItalic + m[1:len(m)-1] + ansiReset
	})

	// Restore inline code
	for i, code := range codes {
		placeholder := fmt.Sprintf("\x00CODE%d\x00", i)
		result = strings.Replace(result, placeholder, ansiBold+code.content+ansiReset, 1)
	}

	result = formatHeader(result)
	result = formatLinks(result)
	result = formatBlockquote(result)
	result = formatNumberedList(result)

	if strings.HasPrefix(result, "- ") {
		result = ansiCyan + "• " + ansiReset + result[2:]
	} else if strings.HasPrefix(result, "* ") {
		result = ansiCyan + "• " + ansiReset + result[2:]
	}

	return result
}

// italicAsterisks pairs up single asterisks for italic.
func italicAsterisks(s string) string {
	var stars []int
	for i := 0; i < len(s); i++ {
		if s[i] == '*' {
			stars = append(stars, i)
		}
	}
	var result strings.Builder
	lastEnd := 0
	for i := 0; i+1 < len(stars); i += 2 {
		start := stars[i]
		end := stars[i+1]
		result.WriteString(s[lastEnd:start])
		result.WriteString(ansiItalic)
		result.WriteString(s[start+1 : end])
		result.WriteString(ansiReset)
		lastEnd = end + 1
	}
	result.WriteString(s[lastEnd:])
	return result.String()
}

func formatHeader(line string) string {
	prefixes := []struct{ prefix, color string }{
		{"# ", ansiBold + ansiUnderline + ansiBrightBlue},
		{"## ", ansiBold + ansiUnderline + ansiBrightBlue},
		{"### ", ansiBold + ansiUnderline + ansiGrey},
		{"#### ", ansiBold + ansiUnderline + ansiGrey},
		{"##### ", ansiBold + ansiUnderline + ansiGrey},
		{"###### ", ansiBold + ansiUnderline + ansiGrey},
	}
	for _, p := range prefixes {
		if strings.HasPrefix(line, p.prefix) {
			return p.color + line[len(p.prefix):] + ansiReset
		}
	}
	return line
}

func formatLinks(line string) string {
	linkRe := regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
	return linkRe.ReplaceAllStringFunc(line, func(m string) string {
		start := strings.Index(m, "[")
		end := strings.Index(m, "](")
		linkText := m[start+1 : end]
		url := m[end+2 : len(m)-1]
		return ansiBrightBlue + linkText + " (" + ansiUnderline + url + ansiNoUnderline + ")" + ansiReset
	})
}

// formatBlockquote formats a blockquote line (starts with "> ").
func formatBlockquote(line string) string {
	if strings.HasPrefix(line, "> ") {
		return ansiGrey + "│ " + ansiReset + line[2:]
	}
	return line
}

// formatNumberedList formats a numbered list item (starts with digits followed by ". ").
func formatNumberedList(line string) string {
	numRe := regexp.MustCompile(`^(\d+)\.\s`)
	if numRe.MatchString(line) {
		return ansiGreen + "→ " + ansiReset + numRe.ReplaceAllString(line, "")
	}
	return line
}
