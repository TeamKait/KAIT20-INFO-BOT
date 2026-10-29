package bot

import (
	"context"
	"strings"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"infobot/internal/search"
)

const (
	maxMessageRunes = 3900
	maxNoteRunes    = 3300
)

// reply sends HTML-formatted text, splitting it into several messages when it
// does not fit into one.
func (a *App) reply(ctx context.Context, chatID int64, text string) {
	for _, part := range chunkText(text, maxMessageRunes) {
		_, err := a.tg.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:             chatID,
			Text:               part,
			ParseMode:          models.ParseModeHTML,
			LinkPreviewOptions: &models.LinkPreviewOptions{IsDisabled: tgbot.True()},
		})
		if err != nil {
			a.log.Warn("send message", "chat_id", chatID, "err", err)
			return
		}
	}
}

// sendNote sends the note in its original Markdown form as a Rich Message.
// Only Obsidian-specific syntax is adapted; the author's headings, tables,
// lists, quotes and links are kept as written. Long notes are sent as a .md
// file, and the HTML renderer is used only as a fallback.
func (a *App) sendNote(ctx context.Context, chatID int64, doc search.DocMatch) {
	a.sendNoteTitled(ctx, chatID, "", doc, nil)
}

// sendNoteTitled is sendNote with a large heading built from the note title
// (the file name), used by /teacher and /subject.
func (a *App) sendNoteTitled(ctx context.Context, chatID int64, title string, doc search.DocMatch, markup models.ReplyMarkup) {
	body := doc.Body
	if title != "" {
		body = "# " + mdTitle(title) + "\n\n" + body
	}

	message := a.richMessage(body)
	if len([]rune(message.Markdown)) > maxNoteRunes {
		header := a.noteHeader(doc.Title, doc.Kind, doc.Path)
		a.sendNoteFile(ctx, chatID, doc.Title, header, doc.Body)
		return
	}

	if err := a.sendRich(ctx, chatID, message, markup); err != nil {
		a.log.Warn("send rich note", "chat_id", chatID, "err", err)
		a.reply(ctx, chatID, renderMarkdown(message.Markdown))
	}
}

func (a *App) sendNoteFile(ctx context.Context, chatID int64, title, caption, body string) {
	_, err := a.tg.SendDocument(ctx, &tgbot.SendDocumentParams{
		ChatID:    chatID,
		Document:  &models.InputFileUpload{Filename: fileName(title), Data: strings.NewReader(body)},
		Caption:   caption,
		ParseMode: models.ParseModeHTML,
	})
	if err != nil {
		a.log.Warn("send document", "chat_id", chatID, "err", err)
	}
}

// chunkText splits text near the limit without breaking runes. The limit is
// counted in runes because Telegram counts message length that way.
func chunkText(text string, limit int) []string {
	if len([]rune(text)) <= limit {
		return []string{text}
	}

	var (
		out   []string
		buf   strings.Builder
		inBuf int
	)

	flush := func() {
		if buf.Len() == 0 {
			return
		}
		out = append(out, buf.String())
		buf.Reset()
		inBuf = 0
	}

	for _, line := range strings.Split(text, "\n") {
		runes := []rune(line)
		for len(runes) > limit {
			flush()
			out = append(out, string(runes[:limit]))
			runes = runes[limit:]
		}
		line = string(runes)
		lineLen := len([]rune(line))

		if inBuf > 0 && inBuf+lineLen+1 > limit {
			flush()
		}
		if inBuf > 0 {
			buf.WriteByte('\n')
			inBuf++
		}
		buf.WriteString(line)
		inBuf += lineLen
	}
	flush()

	if len(out) == 0 {
		return []string{text}
	}
	return out
}

func fileName(title string) string {
	var b strings.Builder
	for _, r := range title {
		switch r {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|':
			b.WriteRune('_')
		default:
			b.WriteRune(r)
		}
	}

	name := strings.TrimSpace(b.String())
	if name == "" {
		name = "note"
	}
	if runes := []rune(name); len(runes) > 60 {
		name = string(runes[:60])
	}
	return name + ".md"
}
