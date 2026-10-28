package bot

import (
	"strings"
	"testing"

	"infobot/internal/vault"
)

func TestPreprocessObsidian(t *testing.T) {
	in := "---\ntags: test\n---\n![[photo.png]]\n### Почта: [[Иван|Иван Петров]]\n> [!INFO]\n> Текст"
	out := preprocessObsidian(in)

	for _, unwanted := range []string{"photo.png", "[[", "]]", "[!INFO]", "tags: test"} {
		if strings.Contains(out, unwanted) {
			t.Errorf("preprocessObsidian() = %q, still contains %q", out, unwanted)
		}
	}
	for _, want := range []string{"Иван Петров", "> Текст"} {
		if !strings.Contains(out, want) {
			t.Errorf("preprocessObsidian() = %q, want %q", out, want)
		}
	}
}

func TestNoteMarkdown(t *testing.T) {
	app := testApp()
	md := app.noteMarkdown("Аттестация", "", "Аттестация.md",
		"| Зачёт | Работа |\n| --- | --- |\n| [[ОП.05]] | [сайт](https://example.com/page) |")

	for _, want := range []string{
		"# Аттестация",
		"| --- | --- |",
		"[Открыть на GitHub](https://github.com/TeamKait/KAIT20-INFO/blob/master/",
		"ОП.05",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("noteMarkdown() does not contain %q:\n%s", want, md)
		}
	}
	if strings.Contains(md, "[[") {
		t.Errorf("noteMarkdown() left wikilinks:\n%s", md)
	}
}

func TestSubjectMarkdown(t *testing.T) {
	app := testApp()
	md := app.subjectMarkdown(&vault.Subject{
		Name:    "МДК.11.01 Базы данных",
		Teacher: "Медведев Владислав Сергеевич",
		Sections: []vault.Section{
			{Title: "Аттестация", Text: "Зачёт"},
			{Title: "Посещаемость", Text: ""},
		},
		Path: "Предметы/МДК.11.01 Базы данных.md",
	})

	for _, want := range []string{
		"# МДК.11.01 Базы данных",
		"*Кто ведёт:* Медведев Владислав Сергеевич",
		"## Аттестация",
		"Зачёт",
		"## Посещаемость",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("subjectMarkdown() does not contain %q:\n%s", want, md)
		}
	}
}

func TestStripFrontMatter(t *testing.T) {
	cases := map[string]string{
		"---\ntags: x\n---\ntext":   "text",
		"---\ntags: x\n---\ntext\n": "text\n",
		"no front matter":           "no front matter",
	}
	for in, want := range cases {
		if got := stripFrontMatter(in); got != want {
			t.Errorf("stripFrontMatter(%q) = %q, want %q", in, got, want)
		}
	}
}
