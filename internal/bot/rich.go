package bot

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"infobot/internal/vault"
)

const (
	maxImageBytes       = 8 << 20
	maxImagesPerMessage = 6
)

var mdImageTarget = regexp.MustCompile(`!\[\[([^\]\|#]+)(?:#[^\]\|]*)?(?:\|[^\]]*)?\]\]`)

func (a *App) sendRich(ctx context.Context, chatID int64, message models.InputRichMessage, markup models.ReplyMarkup) error {
	_, err := a.tg.SendRichMessage(ctx, &tgbot.SendRichMessageParams{
		ChatID:      chatID,
		RichMessage: message,
		ReplyMarkup: markup,
	})
	return err
}

// richNoteMarkdown converts a note into Rich Markdown and uploads the images it
// references. Images live in one vault folder and are embedded through
// tg://photo links, which Rich Messages render as media blocks.
func (a *App) richNoteMarkdown(src string) (string, []models.InputRichMessageMedia) {
	src = strings.ReplaceAll(src, "\r\n", "\n")
	src = stripFrontMatter(src)

	var media []models.InputRichMessageMedia
	nextImage := 0

	if a.vault != nil {
		src = mdImageTarget.ReplaceAllStringFunc(src, func(m string) string {
			if len(media) >= maxImagesPerMessage {
				return ""
			}
			sub := mdImageTarget.FindStringSubmatch(m)
			if len(sub) < 2 {
				return ""
			}

			path, err := a.vault.ResolveImage(strings.TrimSpace(sub[1]))
			if err != nil {
				return ""
			}
			info, err := os.Stat(path)
			if err != nil || info.Size() > maxImageBytes {
				return ""
			}
			data, err := os.ReadFile(path)
			if err != nil || len(data) == 0 {
				return ""
			}

			nextImage++
			id := fmt.Sprintf("img%d", nextImage)
			ext := strings.ToLower(filepath.Ext(path))
			if ext == "" {
				ext = ".png"
			}
			media = append(media, models.InputRichMessageMedia{
				ID: id,
				Media: &models.InputMediaPhoto{
					Media:           "attach://" + id + ext,
					MediaAttachment: bytes.NewReader(data),
				},
			})
			return "\n![](tg://photo?id=" + id + ")\n"
		})
	}

	return preprocessObsidian(src), media
}

// sendTeacherNote sends the teacher note in its original form. The photo is
// embedded into the message when the referenced file exists in the vault.
func (a *App) sendTeacherNote(ctx context.Context, chatID int64, teacher *vault.Teacher) {
	body := teacher.Body
	// Some notes may miss the ![[...]] embed even though the photo exists.
	if a.vault != nil && teacher.Photo == "" {
		implicit := teacher.Name + ".png"
		if _, err := a.vault.ResolveImage(implicit); err == nil {
			body = "![[" + implicit + "]]\n\n" + body
		}
	}
	body = "# " + mdTitle(teacher.Name) + "\n\n" + body

	markdown, media := a.richNoteMarkdown(body)
	if markdown == "" {
		markdown = "(пустая заметка)"
	}

	if err := a.sendRich(ctx, chatID, models.InputRichMessage{Markdown: markdown, Media: media}, a.teacherButtons(teacher)); err != nil {
		a.log.Warn("send rich teacher", "chat_id", chatID, "err", err)
		a.reply(ctx, chatID, a.teacherCard(teacher))
	}
}

// preprocessObsidian adapts vault-specific syntax to standard Markdown. The
// note keeps its own structure: headings, tables, lists, quotes and links are
// not rewritten.
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

var mdTitleEscaper = strings.NewReplacer(
	`\`, `\\`,
	`*`, `\*`,
	`_`, `\_`,
	`[`, `\[`,
	`]`, `\]`,
	"`", "\\`",
	`#`, `\#`,
	`|`, `\|`,
)

// mdTitle makes a note title safe inside a Markdown heading.
func mdTitle(title string) string {
	title = strings.TrimSpace(strings.ReplaceAll(title, "\n", " "))
	return mdTitleEscaper.Replace(title)
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
