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
)

// FormatMarkdown converts markdown to ANSI-formatted text for terminal display.
func FormatMarkdown(input string) string {
	lines := strings.Split(input, "\n")
	var result []string
	inCodeBlock := false
	codeBlockLines := []string{}

	for _, line := range lines {
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
			continue
		}
		if inCodeBlock {
			codeBlockLines = append(codeBlockLines, line)
			continue
		}
		result = append(result, formatInline(line))
	}
	return strings.Join(result, "\n")
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
