package bot

import (
	"strings"
	"testing"

	"infobot/internal/vault"
)

func newButtonsTestApp(t *testing.T) (*App, *vault.Index) {
	t.Helper()

	root := t.TempDir()
	writeVaultFile(t, root, "Преподаватели/Иванов Иван Иванович.md",
		"### Почта: ivanov@example.com\n### Что ведет:\n- [[Математика]]\n- [[Физика]]\n")
	writeVaultFile(t, root, "Предметы/Математика.md", "### Кто ведет: [[Иванов Иван Иванович]]\n")
	writeVaultFile(t, root, "Предметы/Физика.md", "### Кто ведет: [[Иванов Иван Иванович]]\n")

	svc := vault.NewService(vault.NewSyncer("", "master", root, ""), vault.Dirs{})
	if err := svc.LoadIndex(); err != nil {
		t.Fatal(err)
	}
	return &App{vault: svc}, svc.Index()
}

func findTeacher(t *testing.T, idx *vault.Index, name string) *vault.Teacher {
	t.Helper()
	for _, teacher := range idx.Teachers {
		if teacher.Name == name {
			return teacher
		}
	}
	t.Fatalf("teacher %q not found", name)
	return nil
}

func TestTeacherButtonsVertical(t *testing.T) {
	app, idx := newButtonsTestApp(t)
	teacher := findTeacher(t, idx, "Иванов Иван Иванович")

	markup := app.teacherButtons(teacher)
	if markup == nil {
		t.Fatal("teacherButtons returned nil")
	}
	if len(markup.InlineKeyboard) != 2 {
		t.Fatalf("rows = %d, want 2 (one per subject)", len(markup.InlineKeyboard))
	}

	for _, row := range markup.InlineKeyboard {
		if len(row) != 1 {
			t.Errorf("row has %d buttons, want 1 (vertical layout)", len(row))
		}
		if !strings.HasPrefix(row[0].Text, "Карточка ") {
			t.Errorf("button text = %q, want Карточка <subject>", row[0].Text)
		}
		if !strings.HasPrefix(row[0].CallbackData, "s:") {
			t.Errorf("callback data = %q, want s: prefix", row[0].CallbackData)
		}
	}
}

func TestSubjectButtons(t *testing.T) {
	app, idx := newButtonsTestApp(t)
	if len(idx.Subjects) == 0 {
		t.Fatal("no subjects parsed")
	}

	markup := app.subjectButtons(idx.Subjects[0])
	if markup == nil || len(markup.InlineKeyboard) != 1 || len(markup.InlineKeyboard[0]) != 1 {
		t.Fatalf("unexpected keyboard: %#v", markup)
	}

	button := markup.InlineKeyboard[0][0]
	if button.Text != "Карточка преподавателя" {
		t.Errorf("button text = %q", button.Text)
	}
	if !strings.HasPrefix(button.CallbackData, "t:") {
		t.Errorf("callback data = %q, want t: prefix", button.CallbackData)
	}
}

func TestButtonLabelTruncates(t *testing.T) {
	label := buttonLabel(strings.Repeat("я", 100))
	if got := len([]rune(label)); got != maxButtonRunes {
		t.Errorf("label has %d runes, want %d", got, maxButtonRunes)
	}
	if !strings.HasSuffix(label, "…") {
		t.Errorf("label = %q, want ellipsis", label)
	}
}
