package bot

import (
	"context"
	"strings"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"infobot/internal/search"
	"infobot/internal/vault"
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

// sendNote sends the whole note: as a Rich Message when it fits, as a .md file
// when it is too long for Telegram. If the Rich Messages API rejects the
// message, the HTML renderer is used as a fallback.
func (a *App) sendNote(ctx context.Context, chatID int64, doc search.DocMatch) {
	plain := vault.PlainText(doc.Body)

	if len([]rune(plain)) > maxNoteRunes {
		header := a.noteHeader(doc.Title, doc.Kind, doc.Path)
		a.sendNoteFile(ctx, chatID, doc.Title, header, doc.Body)
		return
	}

	if err := a.sendRichMarkdown(ctx, chatID, a.noteMarkdown(doc.Title, doc.Kind, doc.Path, doc.Body)); err != nil {
		a.log.Warn("send rich note", "chat_id", chatID, "err", err)
		header := a.noteHeader(doc.Title, doc.Kind, doc.Path)
		body := renderMarkdown(doc.Body)
		if body == "" {
			body = "(пустая заметка)"
		}
		a.reply(ctx, chatID, header+"\n\n"+body)
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
