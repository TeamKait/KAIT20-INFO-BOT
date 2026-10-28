package bot

import (
	"os"
	"path/filepath"
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

func TestPreprocessObsidianKeepsStructure(t *testing.T) {
	in := "### Кто ведет: [[Кузьмина Ольга Борисовна]]\n\n| A | B |\n| --- | --- |\n| 1 | 2 |\n\n- пункт"
	out := preprocessObsidian(in)

	for _, want := range []string{
		"### Кто ведет: Кузьмина Ольга Борисовна",
		"| A | B |",
		"| --- | --- |",
		"- пункт",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("preprocessObsidian() = %q, want %q", out, want)
		}
	}
}

func TestRichNoteMarkdownWithPhoto(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "Прочее", "картинки")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Иванов.png"), []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}

	app := &App{vault: vault.NewService(vault.NewSyncer("", "master", root, ""), vault.Dirs{})}
	markdown, media := app.richNoteMarkdown("![[Иванов.png]]\n### Почта: ivanov@example.com")

	if !strings.Contains(markdown, "![](tg://photo?id=img1)") {
		t.Errorf("photo reference is missing:\n%s", markdown)
	}
	if strings.Contains(markdown, "![[") {
		t.Errorf("image embed was not replaced:\n%s", markdown)
	}
	if len(media) != 1 || media[0].ID != "img1" {
		t.Fatalf("media = %v, want one item with id img1", media)
	}
	if got := media[0].Media.GetMedia(); got != "attach://img1.png" {
		t.Errorf("media attachment = %q, want attach://img1.png", got)
	}
	if media[0].Media.Attachment() == nil {
		t.Error("media attachment reader is nil")
	}
}

func TestRichNoteMarkdownDropsMissingPhoto(t *testing.T) {
	app := &App{vault: vault.NewService(vault.NewSyncer("", "master", t.TempDir(), ""), vault.Dirs{})}
	markdown, media := app.richNoteMarkdown("![[нет.png]]\nТекст")

	if strings.Contains(markdown, "нет.png") {
		t.Errorf("missing image leaked into markdown:\n%s", markdown)
	}
	if len(media) != 0 {
		t.Errorf("media = %v, want none", media)
	}
	if !strings.Contains(markdown, "Текст") {
		t.Errorf("note text was lost:\n%s", markdown)
	}
}

func TestRichNoteRealTeacher(t *testing.T) {
	dir := os.Getenv("VAULT_TEST_DIR")
	if dir == "" {
		t.Skip("set VAULT_TEST_DIR to check a real teacher note")
	}

	idx, err := vault.BuildIndex(dir, vault.Dirs{})
	if err != nil {
		t.Fatal(err)
	}
	for _, teacher := range idx.Teachers {
		if teacher.Name != "Медведев Владислав Сергеевич" {
			continue
		}

		app := &App{vault: vault.NewService(vault.NewSyncer("", "master", dir, ""), vault.Dirs{})}
		markdown, media := app.richNoteMarkdown(teacher.Body)
		t.Logf("markdown:\n%s", markdown)

		if len(media) != 1 || media[0].ID != "img1" {
			t.Fatalf("media = %v, want one photo", media)
		}
		if !strings.Contains(markdown, `tg://photo?id=img1`) {
			t.Errorf("photo reference is missing")
		}
		if !strings.Contains(markdown, "### Почта:") {
			t.Errorf("teacher note text is missing")
		}
		return
	}
	t.Skip("teacher not found")
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
