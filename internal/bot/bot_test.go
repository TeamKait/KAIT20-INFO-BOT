package bot

import (
	"strings"
	"testing"
)

func TestEsc(t *testing.T) {
	got := esc(`<b> & "кавычки"`)
	want := `&lt;b&gt; &amp; "кавычки"`
	if got != want {
		t.Errorf("esc() = %q, want %q", got, want)
	}
}

func TestRepoLink(t *testing.T) {
	link := newRepoLink("https://github.com/TeamKait/KAIT20-INFO.git", "main").
		fileURL("Преподаватели/Медведев Владислав Сергеевич.md")

	if !strings.HasPrefix(link, "https://github.com/TeamKait/KAIT20-INFO/blob/main/") {
		t.Fatalf("link = %q", link)
	}
	if strings.Contains(link, " ") {
		t.Errorf("link still contains spaces: %q", link)
	}
	if !strings.Contains(link, "%20") {
		t.Errorf("space is not percent-encoded: %q", link)
	}
}

func TestRepoLinkNotGitHub(t *testing.T) {
	if link := newRepoLink("https://gitlab.com/a/b.git", "main").fileURL("x.md"); link != "" {
		t.Errorf("link = %q, want empty for non-GitHub remotes", link)
	}
}

func TestChunkTextLongLine(t *testing.T) {
	text := strings.Repeat("абв", 5000) // 15000 runes, no newlines
	parts := chunkText(text, 1000)

	if len(parts) < 2 {
		t.Fatalf("got %d parts, want several", len(parts))
	}
	for _, part := range parts {
		if n := len([]rune(part)); n > 1000 {
			t.Errorf("part has %d runes, limit is 1000", n)
		}
		if part == "" {
			t.Error("empty part")
		}
	}
	if joined := strings.Join(parts, ""); joined != text {
		t.Error("parts do not reconstruct the original text")
	}
}

func TestChunkTextLines(t *testing.T) {
	text := strings.Repeat("строка\n", 1000) // 7000 runes
	parts := chunkText(text, 100)

	for _, part := range parts {
		if n := len([]rune(part)); n > 100 {
			t.Errorf("part has %d runes, limit is 100", n)
		}
	}
	if joined := strings.Join(parts, "\n"); joined != text {
		t.Error("lines were lost or duplicated")
	}
}

func TestFileName(t *testing.T) {
	got := fileName(`МДК.11.01: тест/путь? *`)
	if strings.ContainsAny(got, `/\:*?"<>|`) {
		t.Errorf("fileName() = %q still contains forbidden characters", got)
	}
	if !strings.HasSuffix(got, ".md") {
		t.Errorf("fileName() = %q, want .md suffix", got)
	}
}

func TestSplitCommand(t *testing.T) {
	cases := []struct {
		in   string
		cmd  string
		args string
	}{
		{"/start", "start", ""},
		{"/teacher@sharaga_bot Иванов email", "teacher", "Иванов email"},
		{"/teacher  Иванов", "teacher", "Иванов"},
		{"/find аттестация", "find", "аттестация"},
		{"просто текст", "", ""},
	}
	for _, tc := range cases {
		cmd, args := splitCommand(tc.in)
		if cmd != tc.cmd || args != tc.args {
			t.Errorf("splitCommand(%q) = (%q, %q), want (%q, %q)", tc.in, cmd, args, tc.cmd, tc.args)
		}
	}
}

func TestSplitTeacherField(t *testing.T) {
	cases := []struct {
		in    string
		name  string
		field string
	}{
		{"Медведев email", "Медведев", "email"},
		{"Медведев Владислав email", "Медведев Владислав", "email"},
		{"Медведев предметы", "Медведев", "subjects"},
		{"Медведев", "Медведев", ""},
		{"Медведев Владислав", "Медведев Владислав", ""},
		{"", "", ""},
	}
	for _, tc := range cases {
		name, field := splitTeacherField(tc.in)
		if name != tc.name || field != tc.field {
			t.Errorf("splitTeacherField(%q) = (%q, %q), want (%q, %q)", tc.in, name, field, tc.name, tc.field)
		}
	}
}
