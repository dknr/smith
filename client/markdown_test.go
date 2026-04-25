package client

import (
	"strings"
	"testing"
)

func TestFormatMarkdown_Bold(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "double asterisk bold",
			input:    "**bold text**",
			expected: "\033[1mbold text\033[0m",
		},
		{
			name:     "double underscore bold",
			input:    "__bold text__",
			expected: "\033[1mbold text\033[0m",
		},
		{
			name:     "multiple bold sections",
			input:    "first **bold** second **bold**",
			expected: "first \033[1mbold\033[0m second \033[1mbold\033[0m",
		},
		{
			name:     "bold with surrounding text",
			input:    "prefix **bold** suffix",
			expected: "prefix \033[1mbold\033[0m suffix",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatMarkdown(tt.input)
			if got != tt.expected {
				t.Errorf("FormatMarkdown(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestFormatMarkdown_Italic(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "single asterisk italic",
			input:    "*italic text*",
			expected: "\033[3mitalic text\033[0m",
		},
		{
			name:     "single underscore italic",
			input:    "_italic text_",
			expected: "\033[3mitalic text\033[0m",
		},
		{
			name:     "multiple italic sections",
			input:    "first *italic* second *italic*",
			expected: "first \033[3mitalic\033[0m second \033[3mitalic\033[0m",
		},
		{
			name:     "italic with surrounding text",
			input:    "prefix *italic* suffix",
			expected: "prefix \033[3mitalic\033[0m suffix",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatMarkdown(tt.input)
			if got != tt.expected {
				t.Errorf("FormatMarkdown(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestFormatMarkdown_InlineCode(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple inline code",
			input:    "use `code` here",
			expected: "use \033[1mcode\033[0m here",
		},
		{
			name:     "inline code with spaces",
			input:    "run `my command`",
			expected: "run \033[1mmy command\033[0m",
		},
		{
			name:     "multiple inline code",
			input:    "`a` and `b`",
			expected: "\033[1ma\033[0m and \033[1mb\033[0m",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatMarkdown(tt.input)
			if got != tt.expected {
				t.Errorf("FormatMarkdown(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestFormatMarkdown_CodeBlocks(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name: "simple code block",
			input: "```go\nfmt.Println(\"hello\")\n```",
			expected: "\033[90mfmt.Println(\"hello\")\033[0m",
		},
		{
			name: "code block with language",
			input: "```python\nprint('hello')\n```",
			expected: "\033[90mprint('hello')\033[0m",
		},
		{
			name: "text before and after code block",
			input: "before\n\n```\ncode\n```\n\nafter",
			expected: "before\n\n\033[90mcode\033[0m\n\nafter",
		},
		{
			name: "multiple code blocks",
			input: "```\na\n```\ntext\n```\nb\n```",
			expected: "\033[90ma\033[0m\ntext\n\033[90mb\033[0m",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatMarkdown(tt.input)
			if got != tt.expected {
				t.Errorf("FormatMarkdown(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestFormatMarkdown_Headers(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "h1 header",
			input:    "# Header 1",
			expected: "\033[1m\033[4m\033[94mHeader 1\033[0m",
		},
		{
			name:     "h2 header",
			input:    "## Header 2",
			expected: "\033[1m\033[4m\033[94mHeader 2\033[0m",
		},
		{
			name:     "h3 header",
			input:    "### Header 3",
			expected: "\033[1m\033[4m\033[90mHeader 3\033[0m",
		},
		{
			name:     "h4 header",
			input:    "#### Header 4",
			expected: "\033[1m\033[4m\033[90mHeader 4\033[0m",
		},
		{
			name:     "h5 header",
			input:    "##### Header 5",
			expected: "\033[1m\033[4m\033[90mHeader 5\033[0m",
		},
		{
			name:     "h6 header",
			input:    "###### Header 6",
			expected: "\033[1m\033[4m\033[90mHeader 6\033[0m",
		},
		{
			name:     "not a header (no space)",
			input:    "#Header",
			expected: "#Header",
		},
		{
			name:     "not a header (too many hashes)",
			input:    "####### Header 7",
			expected: "####### Header 7",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatMarkdown(tt.input)
			if got != tt.expected {
				t.Errorf("FormatMarkdown(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestFormatMarkdown_Links(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple link",
			input:    "[click here](https://example.com)",
			expected: "\033[94mclick here (\033[4mhttps://example.com\033[24m)\033[0m",
		},
		{
			name:     "link with surrounding text",
			input:    "see [docs](https://docs.example.com) for more",
			expected: "see \033[94mdocs (\033[4mhttps://docs.example.com\033[24m)\033[0m for more",
		},
		{
			name:     "multiple links",
			input:    "[a](url1) and [b](url2)",
			expected: "\033[94ma (\033[4murl1\033[24m)\033[0m and \033[94mb (\033[4murl2\033[24m)\033[0m",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatMarkdown(tt.input)
			if got != tt.expected {
				t.Errorf("FormatMarkdown(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestFormatMarkdown_ListItems(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "dash list item",
			input:    "- item one",
			expected: "\033[96m• \033[0mitem one",
		},
		{
			name:     "asterisk list item",
			input:    "* item two",
			expected: "\033[96m• \033[0mitem two",
		},
		{
			name:     "multiple list items",
			input:    "- first\n- second\n- third",
			expected: "\033[96m• \033[0mfirst\n\033[96m• \033[0msecond\n\033[96m• \033[0mthird",
		},
		{
			name:     "list item with text before",
			input:    "prefix - not a list",
			expected: "prefix - not a list",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatMarkdown(tt.input)
			if got != tt.expected {
				t.Errorf("FormatMarkdown(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestFormatMarkdown_EmptyInput(t *testing.T) {
	got := FormatMarkdown("")
	if got != "" {
		t.Errorf("FormatMarkdown(\"\") = %q, want empty string", got)
	}
}

func TestFormatMarkdown_NoFormatting(t *testing.T) {
	input := "plain text without any formatting"
	got := FormatMarkdown(input)
	if got != input {
		t.Errorf("FormatMarkdown(%q) = %q, want %q", input, got, input)
	}
}

func TestFormatMarkdown_Multiline(t *testing.T) {
	input := "line 1\nline 2\nline 3"
	expected := "line 1\nline 2\nline 3"
	got := FormatMarkdown(input)
	if got != expected {
		t.Errorf("FormatMarkdown(%q) = %q, want %q", input, got, expected)
	}
}

func TestFormatMarkdown_MixedFormatting(t *testing.T) {
	input := "**bold** and *italic* and `code`"
	got := FormatMarkdown(input)
	// Check that all formatting is present
	if !strings.Contains(got, "\033[1mbold\033[0m") {
		t.Errorf("missing bold in %q", got)
	}
	if !strings.Contains(got, "\033[3mitalic\033[0m") {
		t.Errorf("missing italic in %q", got)
	}
	if !strings.Contains(got, "\033[1mcode\033[0m") {
		t.Errorf("missing inline code in %q", got)
	}
}

func TestFormatMarkdown_BoldAndItalicCombined(t *testing.T) {
	input := "**bold and *italic* inside**"
	got := FormatMarkdown(input)
	// Bold should wrap the entire thing, italic should be inside
	if !strings.Contains(got, "\033[1m") {
		t.Errorf("missing bold in %q", got)
	}
}

func TestFormatMarkdown_CodeBlockWithFormatting(t *testing.T) {
	input := "```\n**not bold**\n*not italic*\n```"
	got := FormatMarkdown(input)
	// Inside code blocks, formatting should NOT be applied
	if strings.Contains(got, "\033[1m") {
		t.Errorf("bold should not be applied in code blocks, got %q", got)
	}
	if strings.Contains(got, "\033[3m") {
		t.Errorf("italic should not be applied in code blocks, got %q", got)
	}
}

func TestFormatMarkdown_HeaderWithFormatting(t *testing.T) {
	input := "# **Bold Header**"
	got := FormatMarkdown(input)
	// Header formatting should wrap bold formatting
	if !strings.Contains(got, "\033[1m") {
		t.Errorf("missing bold in header %q", got)
	}
}

func TestFormatMarkdown_LinkWithFormatting(t *testing.T) {
	input := "[**bold link**](https://example.com)"
	got := FormatMarkdown(input)
	// Link text should have both underline and bold
	if !strings.Contains(got, "\033[94m") {
		t.Errorf("missing blue in %q", got)
	}
	if !strings.Contains(got, "\033[4m") {
		t.Errorf("missing underline in %q", got)
	}
	if !strings.Contains(got, "\033[1m") {
		t.Errorf("missing bold in %q", got)
	}
}

func TestFormatMarkdown_ListWithFormatting(t *testing.T) {
	input := "- **bold item**"
	got := FormatMarkdown(input)
	// List item should have bullet and bold
	if !strings.Contains(got, "\033[96m•") {
		t.Errorf("missing bullet in %q", got)
	}
	if !strings.Contains(got, "\033[1m") {
		t.Errorf("missing bold in %q", got)
	}
}

func TestFormatMarkdown_UnmatchedAsterisks(t *testing.T) {
	// Odd number of asterisks: first two should be italic, third remains
	input := "*one* and *two* and *three"
	got := FormatMarkdown(input)
	if !strings.Contains(got, "\033[3mone\033[0m") {
		t.Errorf("missing italic 'one' in %q", got)
	}
	if !strings.Contains(got, "\033[3mtwo\033[0m") {
		t.Errorf("missing italic 'two' in %q", got)
	}
}

func TestFormatMarkdown_EmptyCodeBlock(t *testing.T) {
	input := "```\n```\n"
	got := FormatMarkdown(input)
	// Empty code block toggles flag, renders as empty string
	if got != "" {
		t.Errorf("FormatMarkdown(%q) = %q, want empty string", input, got)
	}
}

func TestFormatMarkdown_CodeBlockAtEnd(t *testing.T) {
	input := "text before\n```\ncode\n```"
	got := FormatMarkdown(input)
	expected := "text before\n\033[90mcode\033[0m"
	if got != expected {
		t.Errorf("FormatMarkdown(%q) = %q, want %q", input, got, expected)
	}
}

func TestFormatMarkdown_CodeBlockAtStart(t *testing.T) {
	input := "```\ncode\n```\nafter"
	got := FormatMarkdown(input)
	expected := "\033[90mcode\033[0m\nafter"
	if got != expected {
		t.Errorf("FormatMarkdown(%q) = %q, want %q", input, got, expected)
	}
}

func TestFormatMarkdown_MultipleHeaders(t *testing.T) {
	input := "# H1\n## H2\n### H3"
	got := FormatMarkdown(input)
	expected := "\033[1m\033[4m\033[94mH1\033[0m\n\033[1m\033[4m\033[94mH2\033[0m\n\033[1m\033[4m\033[90mH3\033[0m"
	if got != expected {
		t.Errorf("FormatMarkdown(%q) = %q, want %q", input, got, expected)
	}
}

func TestFormatMarkdown_MultipleLinks(t *testing.T) {
	input := "[link1](url1) [link2](url2)"
	got := FormatMarkdown(input)
	expected := "\033[94mlink1 (\033[4murl1\033[24m)\033[0m \033[94mlink2 (\033[4murl2\033[24m)\033[0m"
	if got != expected {
		t.Errorf("FormatMarkdown(%q) = %q, want %q", input, got, expected)
	}
}

func TestFormatMarkdown_MultipleListItems(t *testing.T) {
	input := "- item1\n- item2\n- item3"
	got := FormatMarkdown(input)
	expected := "\033[96m• \033[0mitem1\n\033[96m• \033[0mitem2\n\033[96m• \033[0mitem3"
	if got != expected {
		t.Errorf("FormatMarkdown(%q) = %q, want %q", input, got, expected)
	}
}

func TestFormatMarkdown_ComplexDocument(t *testing.T) {
	input := "# Title\n\nThis is **bold** and *italic*.\n\n- List item 1\n- List item 2\n\n```\ncode block\n```\n\n[Link](https://example.com)"
	got := FormatMarkdown(input)
	
	// Check each feature is present
	expecteds := []string{
		"\033[1m\033[4m\033[94mTitle\033[0m",
		"\033[1mbold\033[0m",
		"\033[3mitalic\033[0m",
		"\033[96m• \033[0mList item 1",
		"\033[96m• \033[0mList item 2",
		"\033[90mcode block\033[0m",
		"\033[94mLink (\033[4mhttps://example.com\033[24m)\033[0m",
	}
	
	for _, exp := range expecteds {
		if !strings.Contains(got, exp) {
			t.Errorf("missing %q in %q", exp, got)
		}
	}
}

func TestFormatMarkdown_DoubleBackticks(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple double backticks",
			input:    "``code``",
			expected: "\033[1mcode\033[0m",
		},
		{
			name:     "double backticks with surrounding text",
			input:    "use ``code`` here",
			expected: "use \033[1mcode\033[0m here",
		},
		{
			name:     "double backticks with spaces",
			input:    "run ``my command``",
			expected: "run \033[1mmy command\033[0m",
		},
		{
			name:     "multiple double backticks",
			input:    "``a`` and ``b``",
			expected: "\033[1ma\033[0m and \033[1mb\033[0m",
		},
		{
			name:     "single and double backticks",
			input:    "`single` and ``double``",
			expected: "\033[1msingle\033[0m and \033[1mdouble\033[0m",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatMarkdown(tt.input)
			if got != tt.expected {
				t.Errorf("FormatMarkdown(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestFormatMarkdown_NestedBoldItalic(t *testing.T) {
	input := "*bold **nested** italic*"
	got := FormatMarkdown(input)
	// Bold regex runs first: **nested** -> \033[1mnested\033[0m
	// Remaining * are paired for italic
	if !strings.Contains(got, "\033[3m") {
		t.Errorf("missing italic in %q", got)
	}
	if !strings.Contains(got, "\033[1mnested\033[0m") {
		t.Errorf("missing nested bold in %q", got)
	}
}

func TestFormatMarkdown_EmptyLines(t *testing.T) {
	input := "line1\n\nline3"
	got := FormatMarkdown(input)
	expected := "line1\n\nline3"
	if got != expected {
		t.Errorf("FormatMarkdown(%q) = %q, want %q", input, got, expected)
	}
}

func TestFormatMarkdown_CodeBlockPreservesSpaces(t *testing.T) {
	input := "```\n  indented\n    more indented\n```"
	got := FormatMarkdown(input)
	expected := "\033[90m  indented\n    more indented\033[0m"
	if got != expected {
		t.Errorf("FormatMarkdown(%q) = %q, want %q", input, got, expected)
	}
}

func TestFormatMarkdown_CodeBlockWithBackticks(t *testing.T) {
	input := "```\n```\n"
	got := FormatMarkdown(input)
	if got != "" {
		t.Errorf("FormatMarkdown(%q) = %q, want empty string", input, got)
	}
}

func TestFormatMarkdown_MultipleCodeBlocks(t *testing.T) {
	input := "```\na\n```\n\n```\nb\n```"
	got := FormatMarkdown(input)
	expected := "\033[90ma\033[0m\n\n\033[90mb\033[0m"
	if got != expected {
		t.Errorf("FormatMarkdown(%q) = %q, want %q", input, got, expected)
	}
}
