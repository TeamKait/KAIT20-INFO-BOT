package bot

import (
	"context"
	"strings"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"infobot/internal/vault"
)

// sendRichMarkdown sends a Rich Message with Markdown content. Unlike regular
// messages, Rich Messages render GitHub-Flavored Markdown natively: headings,
// tables with borders, lists, quotes and links.
func (a *App) sendRichMarkdown(ctx context.Context, chatID int64, markdown string) error {
	_, err := a.tg.SendRichMessage(ctx, &tgbot.SendRichMessageParams{
		ChatID:      chatID,
		RichMessage: models.InputRichMessage{Markdown: markdown},
	})
	return err
}

// noteMarkdown builds the rich document for /find.
func (a *App) noteMarkdown(title, kind, path, body string) string {
	text := preprocessObsidian(body)
	if text == "" {
		text = "(пустая заметка)"
	}

	var b strings.Builder
	b.WriteString("# " + mdEscape(title) + "\n")
	if kind != "" {
		b.WriteString("\n*" + mdEscape(kind) + "*\n")
	}
	if link := a.repo.fileURL(path); link != "" {
		b.WriteString("\n[Открыть на GitHub](" + link + ")\n")
	}
	b.WriteString("\n" + text + "\n")
	return strings.TrimSpace(b.String())
}

// subjectMarkdown builds the rich document for /subject.
func (a *App) subjectMarkdown(s *vault.Subject) string {
	var b strings.Builder
	b.WriteString("# " + mdEscape(s.Name) + "\n")

	if s.Teacher != "" {
		b.WriteString("\n*Кто ведёт:* " + mdEscape(s.Teacher) + "\n")
	}
	if len(s.Sections) == 0 {
		b.WriteString("\n*Информация не заполнена.*\n")
	}
	for _, section := range s.Sections {
		text := preprocessObsidian(section.Text)
		if text == "" {
			text = "—"
		}
		b.WriteString("\n## " + mdEscape(section.Title) + "\n\n" + text + "\n")
	}
	if link := a.repo.fileURL(s.Path); link != "" {
		b.WriteString("\n[Открыть на GitHub](" + link + ")\n")
	}
	return strings.TrimSpace(b.String())
}

// preprocessObsidian adapts vault-specific syntax to standard Markdown.
func preprocessObsidian(src string) string {
	src = strings.ReplaceAll(src, "\r\n", "\n")
	src = stripFrontMatter(src)
	src = mdImageEmbed.ReplaceAllString(src, "")

	src = mdWikiLink.ReplaceAllStringFunc(src, func(m string) string {
		sub := mdWikiLink.FindStringSubmatch(m)
		if len(sub) > 2 && strings.TrimSpace(sub[2]) != "" {
			return strings.TrimSpace(sub[2])
		}
		return strings.TrimSpace(sub[1])
	})

	lines := strings.Split(src, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, ">") {
			continue
		}
		content := strings.TrimSpace(strings.TrimPrefix(trimmed, ">"))
		// Obsidian callout headers ("[!WARNING]") are not Markdown.
		if strings.HasPrefix(content, "[!") {
			lines[i] = ""
		}
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func stripFrontMatter(src string) string {
	if !strings.HasPrefix(src, "---\n") {
		return src
	}
	rest := src[4:]
	if end := strings.Index(rest, "\n---"); end >= 0 {
		return strings.TrimPrefix(rest[end+4:], "\n")
	}
	return src
}

var mdTextEscaper = strings.NewReplacer(
	`\`, `\\`,
	`*`, `\*`,
	`_`, `\_`,
	`[`, `\[`,
	`]`, `\]`,
	"`", "\\`",
	`#`, `\#`,
	`|`, `\|`,
)

// mdEscape protects generated strings (titles, names) from being parsed as
// Markdown. Note bodies are intentionally left as-is: their formatting is the
// whole point of Rich Messages.
func mdEscape(s string) string {
	return mdTextEscaper.Replace(s)
}
