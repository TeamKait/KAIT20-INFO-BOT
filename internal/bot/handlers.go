package bot

import (
	"context"
	"fmt"
	"strings"
	"time"

	"infobot/internal/search"
)

var teacherFieldAliases = map[string]string{
	"email":    "email",
	"почта":    "email",
	"mail":     "email",
	"subjects": "subjects",
	"предметы": "subjects",
}

// splitTeacherField separates the optional field from the name:
// "/teacher Медведев email" -> ("Медведев", "email").
func splitTeacherField(args string) (name, field string) {
	fields := strings.Fields(args)
	if len(fields) == 0 {
		return "", ""
	}
	if len(fields) == 1 {
		return fields[0], ""
	}
	if f, ok := teacherFieldAliases[strings.ToLower(fields[len(fields)-1])]; ok {
		return strings.Join(fields[:len(fields)-1], " "), f
	}
	return strings.Join(fields, " "), ""
}

func (a *App) handleStart(ctx context.Context, chatID int64) {
	text := "<b>Справочник по хранилищу Sharaga</b>\n\n" +
		"/teacher — список преподавателей\n" +
		"/teacher Иванов — заметка преподавателя: фото, почта, предметы\n" +
		"/teacher Иванов email — только почта\n" +
		"/teacher Иванов предметы — что ведёт\n" +
		"/subject — список предметов\n" +
		"/subject МДК.11.01 — информация о предмете\n" +
		"/find аттестация — найти заметку целиком\n" +
		"/sync — обновить данные из GitHub\n\n" +
		"Понимаю неполные имена и небольшие опечатки. Данные обновляются раз в час."

	if idx := a.vault.Index(); idx != nil {
		text += fmt.Sprintf("\n\nВ хранилище: %d преподавателей, %d предметов.", len(idx.Teachers), len(idx.Subjects))
	}
	a.reply(ctx, chatID, text)
}

func (a *App) handleTeacher(ctx context.Context, chatID int64, args string) {
	idx := a.vault.Index()
	if idx == nil {
		a.reply(ctx, chatID, "Данные ещё не загружены, попробуйте через минуту.")
		return
	}

	if strings.TrimSpace(args) == "" {
		if len(idx.Teachers) == 0 {
			a.reply(ctx, chatID, "В хранилище пока нет преподавателей.")
			return
		}
		a.reply(ctx, chatID, teachersList(idx.Teachers))
		return
	}

	name, field := splitTeacherField(args)
	if name == "" {
		a.reply(ctx, chatID, "Укажите имя: <code>/teacher Иванов</code>")
		return
	}

	matches := search.Teachers(idx, name)
	if len(matches) == 0 || matches[0].Score < 0.55 {
		a.replyTeacherNotFound(ctx, chatID, name, matches)
		return
	}

	best := matches[0]
	if len(matches) > 1 && best.Score < 0.97 && best.Score-matches[1].Score < 0.04 {
		var b strings.Builder
		b.WriteString("Уточните, кто из них:\n")
		for i, match := range matches {
			if i == 3 {
				break
			}
			fmt.Fprintf(&b, "%d. %s\n", i+1, esc(match.Teacher.Name))
		}
		a.reply(ctx, chatID, strings.TrimSpace(b.String()))
		return
	}

	switch field {
	case "email":
		a.reply(ctx, chatID, a.teacherEmailText(best.Teacher))
	case "subjects":
		a.reply(ctx, chatID, a.teacherSubjectsText(best.Teacher))
	default:
		a.sendTeacherNote(ctx, chatID, best.Teacher)
	}
}

func (a *App) replyTeacherNotFound(ctx context.Context, chatID int64, name string, matches []search.TeacherMatch) {
	var b strings.Builder
	b.WriteString("Не нашёл преподавателя «" + esc(name) + "».")

	var alternatives []string
	for _, match := range matches {
		if match.Score < 0.45 || len(alternatives) == 3 {
			break
		}
		alternatives = append(alternatives, match.Teacher.Name)
	}
	if len(alternatives) > 0 {
		b.WriteString("\nВозможно, вы имели в виду:")
		for _, alt := range alternatives {
			b.WriteString("\n• " + esc(alt))
		}
	}
	a.reply(ctx, chatID, b.String())
}

func (a *App) handleSubject(ctx context.Context, chatID int64, args string) {
	idx := a.vault.Index()
	if idx == nil {
		a.reply(ctx, chatID, "Данные ещё не загружены, попробуйте через минуту.")
		return
	}

	name := strings.TrimSpace(args)
	if name == "" {
		if len(idx.Subjects) == 0 {
			a.reply(ctx, chatID, "В хранилище пока нет предметов.")
			return
		}
		a.reply(ctx, chatID, subjectsList(idx.Subjects))
		return
	}

	matches := search.Subjects(idx, name)
	if len(matches) == 0 || matches[0].Score < 0.55 {
		var b strings.Builder
		b.WriteString("Не нашёл предмет «" + esc(name) + "».")
		added := 0
		for _, match := range matches {
			if match.Score < 0.4 || added == 3 {
				break
			}
			b.WriteString("\n• " + esc(match.Subject.Name))
			added++
		}
		a.reply(ctx, chatID, b.String())
		return
	}

	best := matches[0]
	if len(matches) > 1 && best.Score < 0.97 && best.Score-matches[1].Score < 0.04 {
		var b strings.Builder
		b.WriteString("Уточните, какой предмет:\n")
		for i, match := range matches {
			if i == 3 {
				break
			}
			fmt.Fprintf(&b, "%d. %s\n", i+1, esc(match.Subject.Name))
		}
		a.reply(ctx, chatID, strings.TrimSpace(b.String()))
		return
	}

	a.sendSubjectNote(ctx, chatID, best.Subject)
}

func (a *App) handleFind(ctx context.Context, chatID int64, args string) {
	query := strings.TrimSpace(args)
	if query == "" {
		a.reply(ctx, chatID, "Укажите, что искать: <code>/find аттестация</code>")
		return
	}

	idx := a.vault.Index()
	if idx == nil {
		a.reply(ctx, chatID, "Данные ещё не загружены, попробуйте через минуту.")
		return
	}

	docs := search.Documents(idx, query)
	if len(docs) == 0 {
		a.reply(ctx, chatID, "Ничего не нашёл по запросу «"+esc(query)+"».")
		return
	}

	best := docs[0]
	if best.Score >= 0.75 && (len(docs) == 1 || best.Score-docs[1].Score >= 0.05) {
		a.sendNote(ctx, chatID, best)
		return
	}

	var b strings.Builder
	b.WriteString("Нашёл несколько заметок — уточните запрос:")
	for i, doc := range docs {
		if i == 5 {
			break
		}
		fmt.Fprintf(&b, "\n%d. <b>%s</b> · <i>%s</i>", i+1, esc(doc.Title), esc(doc.Kind))
	}
	a.reply(ctx, chatID, b.String())
}

func (a *App) handleSync(ctx context.Context, chatID int64) {
	syncCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()

	updated, err := a.vault.Refresh(syncCtx)
	if err != nil {
		a.reply(ctx, chatID, "⚠️ Не удалось обновить хранилище: "+esc(err.Error()))
		return
	}

	text := "✅ Хранилище уже актуально."
	if updated {
		text = "✅ Хранилище обновлено."
	}
	if idx := a.vault.Index(); idx != nil {
		text += fmt.Sprintf("\nПреподавателей: %d, предметов: %d.", len(idx.Teachers), len(idx.Subjects))
	}
	a.reply(ctx, chatID, text)
}
