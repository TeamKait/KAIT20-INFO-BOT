package bot

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"unicode/utf8"
)

var (
	mdImageEmbed = regexp.MustCompile(`!\[\[[^\]]*\]\]`)
	mdImage      = regexp.MustCompile(`!\[([^\]]*)\]\(([^)\s]+)\)`)
	mdCodeSpan   = regexp.MustCompile("`([^`\\n]+)`")
	mdWikiLink   = regexp.MustCompile(`\[\[([^\]\|#]+)(?:#[^\]\|]*)?(?:\|([^\]]*))?\]\]`)
	mdLink       = regexp.MustCompile(`\[([^\]]+)\]\(([^)\s]+)\)`)

	mdBoldStar       = regexp.MustCompile(`\*\*([^*\n]+)\*\*`)
	mdBoldUnderscore = regexp.MustCompile(`__([^_\n]+)__`)
	mdStrike         = regexp.MustCompile(`~~([^~\n]+)~~`)
	mdItalicStar     = regexp.MustCompile(`\*([^*\n]+)\*`)
	mdItalicUnder    = regexp.MustCompile(`_([^_\n]+)_`)

	mdHeading = regexp.MustCompile(`^(#{1,6})\s+(.*)$`)
	mdList    = regexp.MustCompile(`^(\s*)[-*+]\s+(.*)$`)
	mdTask    = regexp.MustCompile(`^\[([ xX])\]\s+(.*)$`)
	mdRule    = regexp.MustCompile(`^\s*(?:-{3,}|\*{3,}|_{3,})\s*$`)
)

const maxTableColumn = 26

// renderMarkdown converts the light Markdown used by the vault into the HTML
// subset supported by Telegram. Tables have no native Telegram equivalent, so
// they are rendered as an aligned monospace block.
func renderMarkdown(src string) string {
	src = strings.ReplaceAll(src, "\r\n", "\n")
	lines := strings.Split(src, "\n")

	var out []string

	for i := 0; i < len(lines); {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "```") {
			i++
			var code []string
			for i < len(lines) && !strings.HasPrefix(strings.TrimSpace(lines[i]), "```") {
				code = append(code, lines[i])
				i++
			}
			i++ // closing fence
			out = append(out, "<pre>"+esc(strings.Join(code, "\n"))+"</pre>")
			continue
		}

		if strings.Contains(line, "|") && i+1 < len(lines) && isTableSeparator(lines[i+1]) {
			header := splitTableRow(line)
			rows := make([][]string, 0, 4)
			i += 2
			for i < len(lines) {
				row := strings.TrimSpace(lines[i])
				if row == "" || !strings.Contains(row, "|") || isTableSeparator(row) {
					break
				}
				rows = append(rows, splitTableRow(row))
				i++
			}
			out = append(out, renderTable(header, rows))
			continue
		}

		if mdRule.MatchString(trimmed) {
			out = append(out, "———")
			i++
			continue
		}

		if strings.HasPrefix(trimmed, ">") {
			var quoted []string
			for i < len(lines) {
				current := strings.TrimSpace(lines[i])
				if !strings.HasPrefix(current, ">") {
					break
				}
				quoted = append(quoted, strings.TrimSpace(strings.TrimPrefix(current, ">")))
				i++
			}
			// Callout headers like "> [!WARNING]" carry no useful text.
			if len(quoted) > 0 && strings.HasPrefix(quoted[0], "[!") {
				quoted = quoted[1:]
			}
			if inner := renderMarkdown(strings.Join(quoted, "\n")); inner != "" {
				out = append(out, "<blockquote>"+inner+"</blockquote>")
			}
			continue
		}

		if m := mdHeading.FindStringSubmatch(trimmed); m != nil {
			out = append(out, "<b>"+renderInline(m[2])+"</b>")
			i++
			continue
		}

		if m := mdList.FindStringSubmatch(line); m != nil {
			marker := "•"
			text := m[2]
			if task := mdTask.FindStringSubmatch(text); task != nil {
				marker = "☐"
				if strings.EqualFold(task[1], "x") {
					marker = "☑"
				}
				text = task[2]
			}
			indent := strings.ReplaceAll(m[1], "  ", "&nbsp;&nbsp;")
			out = append(out, indent+marker+" "+renderInline(text))
			i++
			continue
		}

		if trimmed == "" {
			out = append(out, "")
			i++
			continue
		}

		out = append(out, renderInline(line))
		i++
	}

	return strings.TrimSpace(strings.Join(out, "\n"))
}

// inlineBuilder protects already-rendered fragments (links, code) while the
// rest of the line is escaped and passed through emphasis patterns.
type inlineBuilder struct {
	parts []string
}

func (b *inlineBuilder) hold(html string) string {
	b.parts = append(b.parts, html)
	return fmt.Sprintf("\x00%d\x00", len(b.parts)-1)
}

func (b *inlineBuilder) restore(s string) string {
	for i := len(b.parts) - 1; i >= 0; i-- {
		s = strings.ReplaceAll(s, fmt.Sprintf("\x00%d\x00", i), b.parts[i])
	}
	return s
}

func renderInline(src string) string {
	b := &inlineBuilder{}
	s := src

	s = mdImageEmbed.ReplaceAllString(s, "")
	s = mdImage.ReplaceAllString(s, "$1")

	s = mdCodeSpan.ReplaceAllStringFunc(s, func(m string) string {
		sub := mdCodeSpan.FindStringSubmatch(m)
		return b.hold("<code>" + esc(sub[1]) + "</code>")
	})

	s = mdWikiLink.ReplaceAllStringFunc(s, func(m string) string {
		sub := mdWikiLink.FindStringSubmatch(m)
		label := sub[1]
		if len(sub) > 2 && strings.TrimSpace(sub[2]) != "" {
			label = sub[2]
		}
		return b.hold(esc(strings.TrimSpace(label)))
	})

	s = mdLink.ReplaceAllStringFunc(s, func(m string) string {
		sub := mdLink.FindStringSubmatch(m)
		label, target := sub[1], sub[2]
		if link, ok := safeURL(target); ok {
			return b.hold(`<a href="` + link + `">` + renderInline(label) + `</a>`)
		}
		return b.hold(esc(label))
	})

	s = esc(s)
	s = mdBoldStar.ReplaceAllString(s, "<b>$1</b>")
	s = mdBoldUnderscore.ReplaceAllString(s, "<b>$1</b>")
	s = mdStrike.ReplaceAllString(s, "<s>$1</s>")
	s = mdItalicStar.ReplaceAllString(s, "<i>$1</i>")
	s = mdItalicUnder.ReplaceAllString(s, "<i>$1</i>")

	return b.restore(s)
}

// safeURL accepts only schemes that make sense in a Telegram message.
func safeURL(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" {
		return "", false
	}

	switch strings.ToLower(u.Scheme) {
	case "http", "https", "tg", "mailto":
	default:
		return "", false
	}

	link := u.String()
	link = strings.ReplaceAll(link, "&", "&amp;")
	link = strings.ReplaceAll(link, `"`, "%22")
	link = strings.ReplaceAll(link, "<", "%3C")
	link = strings.ReplaceAll(link, ">", "%3E")
	link = strings.ReplaceAll(link, " ", "%20")
	return link, true
}

func isTableSeparator(line string) bool {
	line = strings.TrimSpace(line)
	if !strings.Contains(line, "|") || !strings.Contains(line, "-") {
		return false
	}
	for _, r := range line {
		switch r {
		case '|', '-', ':', ' ', '\t':
		default:
			return false
		}
	}
	return true
}

func splitTableRow(line string) []string {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "|")
	line = strings.TrimSuffix(line, "|")

	parts := strings.Split(line, "|")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

// cellPlain removes inline markdown because table cells are rendered inside a
// monospace block where HTML formatting is not available.
func cellPlain(s string) string {
	s = mdImageEmbed.ReplaceAllString(s, "")
	s = mdImage.ReplaceAllString(s, "$1")
	s = mdLink.ReplaceAllString(s, "$1")
	s = mdWikiLink.ReplaceAllStringFunc(s, func(m string) string {
		sub := mdWikiLink.FindStringSubmatch(m)
		if len(sub) > 2 && strings.TrimSpace(sub[2]) != "" {
			return sub[2]
		}
		return sub[1]
	})
	s = strings.NewReplacer("**", "", "__", "", "~~", "", "`", "", "*", "").Replace(s)
	return strings.TrimSpace(s)
}

func renderTable(header []string, rows [][]string) string {
	cols := len(header)
	for _, row := range rows {
		if len(row) > cols {
			cols = len(row)
		}
	}
	if cols == 0 {
		return ""
	}

	widths := make([]int, cols)
	measure := func(cells []string) {
		for c, cell := range cells {
			if c >= cols {
				break
			}
			w := utf8.RuneCountInString(cellPlain(cell))
			if w > widths[c] {
				widths[c] = w
			}
		}
	}
	measure(header)
	for _, row := range rows {
		measure(row)
	}
	for c := range widths {
		if widths[c] > maxTableColumn {
			widths[c] = maxTableColumn
		}
		if widths[c] < 3 {
			widths[c] = 3
		}
	}

	var b strings.Builder
	b.WriteString("<pre>")

	emit := func(cells []string) {
		wrapped := make([][]string, cols)
		height := 1
		for c := 0; c < cols; c++ {
			value := ""
			if c < len(cells) {
				value = cellPlain(cells[c])
			}
			wrapped[c] = wrapCell(value, widths[c])
			if len(wrapped[c]) > height {
				height = len(wrapped[c])
			}
		}

		for h := 0; h < height; h++ {
			var line strings.Builder
			for c := 0; c < cols; c++ {
				part := ""
				if h < len(wrapped[c]) {
					part = wrapped[c][h]
				}
				line.WriteString(padCell(esc(part), widths[c]))
				if c < cols-1 {
					line.WriteString(" │ ")
				}
			}
			b.WriteString(strings.TrimRight(line.String(), " "))
			b.WriteString("\n")
		}
	}

	emit(header)

	for c := 0; c < cols; c++ {
		b.WriteString(strings.Repeat("─", widths[c]))
		if c < cols-1 {
			b.WriteString("─┼─")
		}
	}
	b.WriteString("\n")

	for _, row := range rows {
		emit(row)
	}

	b.WriteString("</pre>")
	return b.String()
}

func wrapCell(s string, width int) []string {
	words := strings.Fields(s)
	if len(words) == 0 {
		return []string{""}
	}

	var lines []string
	current := ""

	for _, word := range words {
		runes := []rune(word)
		if len(runes) > width {
			if current != "" {
				lines = append(lines, current)
				current = ""
			}
			for len(runes) > width {
				lines = append(lines, string(runes[:width]))
				runes = runes[width:]
			}
			current = string(runes)
			continue
		}
		if current == "" {
			current = word
		} else if utf8.RuneCountInString(current)+1+utf8.RuneCountInString(word) <= width {
			current += " " + word
		} else {
			lines = append(lines, current)
			current = word
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}

func padCell(s string, width int) string {
	pad := width - utf8.RuneCountInString(s)
	if pad <= 0 {
		return s
	}
	return s + strings.Repeat(" ", pad)
}
