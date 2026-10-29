package bot

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"strings"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"infobot/internal/search"
	"infobot/internal/vault"
)

const maxButtonRunes = 60

// noteID is a short, stable identifier for callback data (Telegram limits
// callback_data to 64 bytes, and note names are too long for that).
func noteID(path string) string {
	sum := sha1.Sum([]byte(path))
	return hex.EncodeToString(sum[:6])
}

// teacherButtons lists every subject of the teacher as a separate vertical row.
func (a *App) teacherButtons(teacher *vault.Teacher) *models.InlineKeyboardMarkup {
	if a.vault == nil {
		return nil
	}
	idx := a.vault.Index()
	if idx == nil {
		return nil
	}

	seen := make(map[string]bool)
	var rows [][]models.InlineKeyboardButton
	for _, name := range teacher.Subjects {
		subject := findSubjectByName(idx, name)
		if subject == nil {
			continue
		}
		id := noteID(subject.Path)
		if seen[id] {
			continue
		}
		seen[id] = true
		rows = append(rows, []models.InlineKeyboardButton{{
			Text:         buttonLabel("Карточка " + subject.Name),
			CallbackData: "s:" + id,
		}})
	}

	if len(rows) == 0 {
		return nil
	}
	return &models.InlineKeyboardMarkup{InlineKeyboard: rows}
}

func (a *App) subjectButtons(subject *vault.Subject) *models.InlineKeyboardMarkup {
	if a.vault == nil || subject.Teacher == "" {
		return nil
	}
	idx := a.vault.Index()
	if idx == nil {
		return nil
	}
	teacher := findTeacherByName(idx, subject.Teacher)
	if teacher == nil {
		return nil
	}
	return &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{{
		{
			Text:         "Карточка преподавателя",
			CallbackData: "t:" + noteID(teacher.Path),
		},
	}}}
}

func buttonLabel(text string) string {
	runes := []rune(text)
	if len(runes) <= maxButtonRunes {
		return text
	}
	return string(runes[:maxButtonRunes-1]) + "…"
}

func findTeacherByName(idx *vault.Index, name string) *vault.Teacher {
	for _, teacher := range idx.Teachers {
		if strings.EqualFold(teacher.Name, name) {
			return teacher
		}
	}
	matches := search.Teachers(idx, name)
	if len(matches) > 0 && matches[0].Score >= 0.8 {
		return matches[0].Teacher
	}
	return nil
}

func findSubjectByName(idx *vault.Index, name string) *vault.Subject {
	for _, subject := range idx.Subjects {
		if strings.EqualFold(subject.Name, name) {
			return subject
		}
	}
	matches := search.Subjects(idx, name)
	if len(matches) > 0 && matches[0].Score >= 0.8 {
		return matches[0].Subject
	}
	return nil
}

func findTeacherByID(idx *vault.Index, id string) *vault.Teacher {
	for _, teacher := range idx.Teachers {
		if noteID(teacher.Path) == id {
			return teacher
		}
	}
	return nil
}

func findSubjectByID(idx *vault.Index, id string) *vault.Subject {
	for _, subject := range idx.Subjects {
		if noteID(subject.Path) == id {
			return subject
		}
	}
	return nil
}

func (a *App) sendSubjectNote(ctx context.Context, chatID int64, subject *vault.Subject) {
	a.sendNoteTitled(ctx, chatID, subject.Name, search.DocMatch{
		Title: subject.Name,
		Kind:  "Предмет",
		Path:  subject.Path,
		Body:  subject.Body,
	}, a.subjectButtons(subject))
}

func (a *App) handleCallback(ctx context.Context, _ *tgbot.Bot, update *models.Update) {
	query := update.CallbackQuery
	if query == nil {
		return
	}

	var editErr error
	if a.vault != nil {
		if idx := a.vault.Index(); idx != nil {
			if chatID, messageID, ok := callbackTarget(query); ok {
				switch {
				case strings.HasPrefix(query.Data, "t:"):
					if teacher := findTeacherByID(idx, query.Data[2:]); teacher != nil {
						editErr = a.editTeacherNote(ctx, chatID, messageID, teacher)
					}
				case strings.HasPrefix(query.Data, "s:"):
					if subject := findSubjectByID(idx, query.Data[2:]); subject != nil {
						editErr = a.editSubjectNote(ctx, chatID, messageID, subject)
					}
				}
			}
		}
	}

	params := &tgbot.AnswerCallbackQueryParams{CallbackQueryID: query.ID}
	if editErr != nil {
		a.log.Warn("edit callback message", "err", editErr)
		params.Text = "Не удалось обновить сообщение"
		params.ShowAlert = true
	}
	if _, err := a.tg.AnswerCallbackQuery(ctx, params); err != nil {
		a.log.Warn("answer callback", "err", err)
	}
}

func (a *App) guardCallback(next tgbot.HandlerFunc) tgbot.HandlerFunc {
	return func(ctx context.Context, b *tgbot.Bot, update *models.Update) {
		query := update.CallbackQuery
		if query == nil {
			return
		}
		if len(a.cfg.AllowedChats) > 0 {
			chatID, _, ok := callbackTarget(query)
			if !ok {
				return
			}
			if _, allowed := a.cfg.AllowedChats[chatID]; !allowed {
				return
			}
		}
		next(ctx, b, update)
	}
}

func callbackTarget(query *models.CallbackQuery) (int64, int, bool) {
	if message := query.Message.Message; message != nil {
		return message.Chat.ID, message.ID, true
	}
	if message := query.Message.InaccessibleMessage; message != nil {
		return message.Chat.ID, message.MessageID, true
	}
	return 0, 0, false
}
