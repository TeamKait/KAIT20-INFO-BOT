package bot

import (
	"os"
	"strings"
	"testing"

	"infobot/internal/vault"
)

func TestRenderMarkdownLink(t *testing.T) {
	got := renderMarkdown("Смотри [сайт](https://example.com/page?a=1&b=2).")

	want := `<a href="https://example.com/page?a=1&amp;b=2">сайт</a>`
	if !strings.Contains(got, want) {
		t.Errorf("renderMarkdown() = %q, want it to contain %q", got, want)
	}
}

func TestRenderMarkdownRejectsUnsafeLink(t *testing.T) {
	got := renderMarkdown("[тык](javascript:alert)")

	if strings.Contains(got, "<a ") {
		t.Errorf("unsafe scheme produced a link: %q", got)
	}
	if !strings.Contains(got, "тык") {
		t.Errorf("link label was lost: %q", got)
	}
}

func TestRenderMarkdownTable(t *testing.T) {
	src := "| Зачёт | Работа |\n| --- | --- |\n| [[ОП.05]] | [сайт](https://example.com) |"
	got := renderMarkdown(src)

	for _, want := range []string{"<pre>", "Зачёт", "Работа", "ОП.05", "│"} {
		if !strings.Contains(got, want) {
			t.Errorf("table output does not contain %q:\n%s", want, got)
		}
	}
	for _, unwanted := range []string{"| ---", "[[", "]]"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("table output still contains %q:\n%s", unwanted, got)
		}
	}
}

func TestRenderMarkdownInline(t *testing.T) {
	got := renderMarkdown("**жирный**, *курсив*, `код` и ~~старое~~")

	for _, want := range []string{"<b>жирный</b>", "<i>курсив</i>", "<code>код</code>", "<s>старое</s>"} {
		if !strings.Contains(got, want) {
			t.Errorf("renderMarkdown() = %q, want %q", got, want)
		}
	}
}

func TestRenderMarkdownHeadingAndList(t *testing.T) {
	got := renderMarkdown("### Почта: v@x\n- один\n- два\n- [x] готово")

	for _, want := range []string{"<b>Почта: v@x</b>", "• один", "• два", "☑ готово"} {
		if !strings.Contains(got, want) {
			t.Errorf("renderMarkdown() = %q, want %q", got, want)
		}
	}
}

func TestRenderMarkdownEscapesHTML(t *testing.T) {
	got := renderMarkdown("<script>alert(1)</script>")

	if strings.Contains(got, "<script>") {
		t.Errorf("raw HTML leaked into the output: %q", got)
	}
	if !strings.Contains(got, "&lt;script&gt;") {
		t.Errorf("output is not escaped: %q", got)
	}
}

func TestRenderMarkdownCallout(t *testing.T) {
	got := renderMarkdown("> [!WARNING]\n> Не подтверждено")

	if !strings.Contains(got, "<blockquote>Не подтверждено</blockquote>") {
		t.Errorf("callout was not rendered: %q", got)
	}
	if strings.Contains(got, "[!WARNING]") {
		t.Errorf("callout marker leaked: %q", got)
	}
}

func TestRenderMarkdownWikilink(t *testing.T) {
	got := renderMarkdown("[[МДК.11.01|Базы данных]]")

	if !strings.Contains(got, "Базы данных") || strings.Contains(got, "[[") {
		t.Errorf("wikilink was not unwrapped: %q", got)
	}
}

// TestRenderRealNote is an optional check against a real vault. It logs the
// rendered table so a failed formatting change is easy to inspect.
func TestRenderRealNote(t *testing.T) {
	dir := os.Getenv("VAULT_TEST_DIR")
	if dir == "" {
		t.Skip("set VAULT_TEST_DIR to render a real note")
	}

	idx, err := vault.BuildIndex(dir, vault.Dirs{})
	if err != nil {
		t.Fatal(err)
	}
	for _, note := range idx.Notes {
		if note.Title != "Аттестация" {
			continue
		}
		rendered := renderMarkdown(note.Body)
		t.Logf("rendered:\n%s", rendered)
		if strings.Contains(rendered, "| ---") {
			t.Error("separator row was not converted")
		}
		return
	}
	t.Skip("note Аттестация not found")
}
