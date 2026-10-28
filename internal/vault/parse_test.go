package vault

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const teacherFixture = `![[Волкова Юлия Александровна.png]]
### Почта: uvolkova@kait20.ru
>[!WARNING] Не подтверждено
>Почта не подтверждена преподавателем и была выведена из ФИО
### Что ведет:
- [[УП.14 Учебная практика]]
`

const subjectFixture = `### Кто ведет: [[Кузьмина Ольга Борисовна]]
### Как получать оценки
Сдавать практические.

### Посещаемость
*Неизвестно*

---

### ИУП
Нужно прийти и сдать.

---

## Аттестация
Итоговая отметка за семестр
`

func TestParseTeacher(t *testing.T) {
	got := parseTeacher("Волкова Юлия Александровна", "Преподаватели/Волкова Юлия Александровна.md", []byte(teacherFixture))

	if got.Email != "uvolkova@kait20.ru" {
		t.Errorf("email = %q, want uvolkova@kait20.ru", got.Email)
	}
	if !got.EmailUnconfirmed {
		t.Error("EmailUnconfirmed = false, want true")
	}
	if got.Photo != "Волкова Юлия Александровна.png" {
		t.Errorf("photo = %q", got.Photo)
	}
	if len(got.Subjects) != 1 || got.Subjects[0] != "УП.14 Учебная практика" {
		t.Errorf("subjects = %v", got.Subjects)
	}
}

func TestParseTeacherWithoutSection(t *testing.T) {
	body := "### Почта: skarpov@kait20.ru\n### Что ведет:\n- [[МДК.04.02 Обеспечение качества]]"
	got := parseTeacher("Карпов Семен Алексеевич", "Преподаватели/Карпов Семен Алексеевич.md", []byte(body))
	if got.EmailUnconfirmed {
		t.Error("EmailUnconfirmed = true, want false")
	}
	if len(got.Subjects) != 1 {
		t.Errorf("subjects = %v", got.Subjects)
	}
}

func TestParseSubject(t *testing.T) {
	got := parseSubject("УП.14 Учебная практика", "Предметы/УП.14 Учебная практика.md", []byte(subjectFixture))

	if got.Teacher != "Кузьмина Ольга Борисовна" {
		t.Errorf("teacher = %q", got.Teacher)
	}
	titles := make([]string, len(got.Sections))
	for i, section := range got.Sections {
		titles[i] = section.Title
	}
	want := []string{"Как получать оценки", "Посещаемость", "ИУП", "Аттестация"}
	if strings.Join(titles, "|") != strings.Join(want, "|") {
		t.Errorf("sections = %v, want %v", titles, want)
	}
	if !strings.Contains(got.Sections[2].Text, "Нужно прийти") {
		t.Errorf("section text = %q", got.Sections[2].Text)
	}
}

func TestBuildIndex(t *testing.T) {
	root := t.TempDir()
	write := func(rel, content string) {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	write("Преподаватели/Волкова Юлия Александровна.md", teacherFixture)
	write("Предметы/УП.14 Учебная практика.md", "### Кто ведет: [[Волкова Юлия Александровна]]\n### Аттестация\nЗачёт\n")
	write("Прочее/Шаблоны/ПРЕПОДАВАТЕЛЬ.md", teacherFixture) // templates must be skipped
	write("Аттестация.md", "| Дифференцированный зачет |\n| --- |")

	idx, err := BuildIndex(root, Dirs{})
	if err != nil {
		t.Fatal(err)
	}

	if len(idx.Teachers) != 1 {
		t.Errorf("teachers = %d, want 1", len(idx.Teachers))
	}
	if len(idx.Subjects) != 1 {
		t.Errorf("subjects = %d, want 1", len(idx.Subjects))
	}
	if len(idx.Notes) != 1 {
		t.Fatalf("notes = %d, want 1 (template must be skipped)", len(idx.Notes))
	}
	if idx.Notes[0].Kind != "" {
		t.Errorf("root note kind = %q, want empty", idx.Notes[0].Kind)
	}
}

func TestPlainText(t *testing.T) {
	in := "![[photo.png]]\n### Почта: [[Иван Петров]]\n>[!WARNING] Не подтверждено\n>Почта выведена из ФИО\n**жирный** и `код`"
	out := PlainText(in)

	for _, unwanted := range []string{"photo.png", "[[", "]]", "###", "**", "`", ">[!", ">Почта"} {
		if strings.Contains(out, unwanted) {
			t.Errorf("PlainText(%q) = %q, still contains %q", in, out, unwanted)
		}
	}
	if !strings.Contains(out, "Иван Петров") {
		t.Errorf("PlainText lost the wikilink label: %q", out)
	}
	if !strings.Contains(out, "Почта выведена из ФИО") {
		t.Errorf("PlainText lost the callout body: %q", out)
	}
}

func TestBuildIndexRealVault(t *testing.T) {
	dir := os.Getenv("VAULT_TEST_DIR")
	if dir == "" {
		t.Skip("set VAULT_TEST_DIR to run against a real vault")
	}

	idx, err := BuildIndex(dir, Dirs{})
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Teachers) == 0 {
		t.Fatal("no teachers parsed")
	}
	if len(idx.Subjects) == 0 {
		t.Fatal("no subjects parsed")
	}

	teacherFound := false
	for _, teacher := range idx.Teachers {
		if teacher.Name != "Медведев Владислав Сергеевич" {
			continue
		}
		teacherFound = true
		if teacher.Email != "vmedvedev@kait20.ru" {
			t.Errorf("email = %q", teacher.Email)
		}
		if !teacher.EmailUnconfirmed {
			t.Error("Медведев must be flagged as unconfirmed")
		}
		if len(teacher.Subjects) == 0 {
			t.Error("Медведев must have subjects")
		}
	}
	if !teacherFound {
		t.Error("Медведев not found")
	}

	subjectFound := false
	for _, subject := range idx.Subjects {
		if !strings.HasPrefix(subject.Name, "МДК.01.02") {
			continue
		}
		subjectFound = true
		if subject.Teacher != "Кузьмина Ольга Борисовна" {
			t.Errorf("teacher = %q", subject.Teacher)
		}
		if len(subject.Sections) == 0 {
			t.Error("subject has no sections")
		}
	}
	if !subjectFound {
		t.Error("МДК.01.02 not found")
	}
}
