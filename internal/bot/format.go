package bot

import (
	"fmt"
	"net/url"
	"strings"

	"infobot/internal/vault"
)

var htmlEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")

// esc makes user-controlled text safe for Telegram's HTML parse mode.
func esc(s string) string {
	return htmlEscaper.Replace(s)
}

// repoLink builds clickable GitHub URLs for notes.
type repoLink struct {
	base   string
	branch string
}

func newRepoLink(raw, branch string) repoLink {
	if strings.TrimSpace(branch) == "" {
		branch = "main"
	}
	raw = strings.TrimSpace(raw)
	raw = strings.TrimSuffix(raw, "/")
	raw = strings.TrimSuffix(raw, ".git")

	if rest, ok := strings.CutPrefix(raw, "git@github.com:"); ok {
		raw = "https://github.com/" + rest
	}
	if rest, ok := strings.CutPrefix(raw, "https://github.com/"); ok {
		parts := strings.Split(rest, "/")
		if len(parts) >= 2 && parts[0] != "" && parts[1] != "" {
			return repoLink{base: "https://github.com/" + parts[0] + "/" + parts[1], branch: branch}
		}
	}
	return repoLink{branch: branch}
}

func (r repoLink) fileURL(rel string) string {
	if r.base == "" || rel == "" {
		return ""
	}
	segments := strings.Split(rel, "/")
	for i, segment := range segments {
		segments[i] = url.PathEscape(segment)
	}
	return r.base + "/blob/" + url.PathEscape(r.branch) + "/" + strings.Join(segments, "/")
}

func (a *App) teacherCard(t *vault.Teacher) string {
	var b strings.Builder
	b.WriteString("👤 <b>" + esc(t.Name) + "</b>\n")

	if t.Email != "" {
		b.WriteString("📧 <code>" + esc(t.Email) + "</code>")
		if t.EmailUnconfirmed {
			b.WriteString(" <i>(не подтверждена)</i>")
		}
		b.WriteString("\n")
	} else {
		b.WriteString("📧 почта не указана\n")
	}

	if len(t.Subjects) > 0 {
		b.WriteString("📚 <b>Ведёт:</b>\n")
		for _, subject := range t.Subjects {
			b.WriteString("• " + esc(subject) + "\n")
		}
	}

	if link := a.repo.fileURL(t.Path); link != "" {
		b.WriteString("🔗 <a href=\"" + link + "\">заметка на GitHub</a>")
	}
	return strings.TrimSpace(b.String())
}

func (a *App) teacherEmailText(t *vault.Teacher) string {
	if t.Email == "" {
		return "У «" + esc(t.Name) + "» почта не указана."
	}
	text := "📧 <code>" + esc(t.Email) + "</code>"
	if t.EmailUnconfirmed {
		text += "\n⚠️ <i>Не подтверждена преподавателем</i>"
	}
	return text
}

func (a *App) teacherSubjectsText(t *vault.Teacher) string {
	if len(t.Subjects) == 0 {
		return "У «" + esc(t.Name) + "» не указаны предметы."
	}
	var b strings.Builder
	b.WriteString("📚 <b>" + esc(t.Name) + " ведёт:</b>\n")
	for _, subject := range t.Subjects {
		b.WriteString("• " + esc(subject) + "\n")
	}
	return strings.TrimSpace(b.String())
}

func (a *App) subjectCard(s *vault.Subject) string {
	var b strings.Builder
	b.WriteString("📘 <b>" + esc(s.Name) + "</b>\n")

	if s.Teacher != "" {
		b.WriteString("👩‍🏫 <b>Кто ведёт:</b> " + esc(s.Teacher) + "\n")
	}

	if len(s.Sections) == 0 {
		b.WriteString("\n<i>Информация не заполнена.</i>")
	}
	for _, section := range s.Sections {
		text := renderMarkdown(section.Text)
		if text == "" {
			text = "—"
		}
		b.WriteString("\n▫️ <b>" + esc(section.Title) + "</b>\n")
		b.WriteString(text + "\n")
	}

	if link := a.repo.fileURL(s.Path); link != "" {
		b.WriteString("\n🔗 <a href=\"" + link + "\">заметка на GitHub</a>")
	}
	return strings.TrimSpace(b.String())
}

func (a *App) noteHeader(title, kind, path string) string {
	var b strings.Builder
	b.WriteString("📄 <b>" + esc(title) + "</b>")
	if kind != "" {
		b.WriteString(" · <i>" + esc(kind) + "</i>")
	}
	if link := a.repo.fileURL(path); link != "" {
		b.WriteString("\n🔗 <a href=\"" + link + "\">открыть на GitHub</a>")
	}
	return b.String()
}

// teachersList renders the "no arguments" answer for /teacher.
func teachersList(teachers []*vault.Teacher) string {
	var b strings.Builder
	fmt.Fprintf(&b, "👥 <b>Преподаватели</b> — %d\n\n", len(teachers))
	for _, teacher := range teachers {
		b.WriteString("• " + esc(teacher.Name) + "\n")
	}
	b.WriteString("\nПодробнее: <code>/teacher Фамилия</code> или <code>/teacher Фамилия email</code>")
	return b.String()
}

// subjectsList renders the "no arguments" answer for /subject.
func subjectsList(subjects []*vault.Subject) string {
	var b strings.Builder
	fmt.Fprintf(&b, "📚 <b>Предметы</b> — %d\n\n", len(subjects))
	for _, subject := range subjects {
		b.WriteString("• " + esc(subject.Name) + "\n")
	}
	b.WriteString("\nПодробнее: <code>/subject Предмет</code>")
	return b.String()
}
