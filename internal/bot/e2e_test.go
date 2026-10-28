package bot

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"infobot/internal/config"
	"infobot/internal/vault"
)

// fakeTelegram implements just enough of the Bot API for an end-to-end test.
type fakeTelegram struct {
	mu       sync.Mutex
	sent     []string
	rich     []string
	updateID int64
	incoming string
}

func (f *fakeTelegram) handler(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	switch {
	case strings.HasSuffix(path, "/getMe"):
		writeJSON(w, map[string]any{"ok": true, "result": map[string]any{
			"id": 123, "is_bot": true, "first_name": "Test", "username": "test_bot",
		}})

	case strings.HasSuffix(path, "/setMyCommands"):
		writeJSON(w, map[string]any{"ok": true, "result": true})

	case strings.HasSuffix(path, "/getUpdates"):
		offset := int64(0)
		if r.Method == http.MethodGet {
			offset, _ = strconv.ParseInt(r.URL.Query().Get("offset"), 10, 64)
		} else if err := r.ParseMultipartForm(1 << 20); err == nil {
			offset, _ = strconv.ParseInt(r.FormValue("offset"), 10, 64)
		}

		f.mu.Lock()
		defer f.mu.Unlock()

		// The startup call with offset=-1 returns no backlog. Polling with
		// offset=1 gets exactly one new message.
		if offset == 1 && f.updateID == 0 {
			f.updateID = 1
			text := f.incoming
			if text == "" {
				text = "/teacher Медведев"
			}
			writeJSON(w, map[string]any{"ok": true, "result": []any{map[string]any{
				"update_id": 1,
				"message": map[string]any{
					"message_id": 1,
					"date":       time.Now().Unix(),
					"chat":       map[string]any{"id": 42, "type": "group"},
					"text":       text,
				},
			}}})
			return
		}
		writeJSON(w, map[string]any{"ok": true, "result": []any{}})

	case strings.HasSuffix(path, "/sendMessage"):
		_ = r.ParseMultipartForm(1 << 20)
		text := r.FormValue("text")

		f.mu.Lock()
		f.sent = append(f.sent, text)
		f.mu.Unlock()

		writeJSON(w, map[string]any{"ok": true, "result": map[string]any{
			"message_id": 2,
			"date":       time.Now().Unix(),
			"chat":       map[string]any{"id": 42, "type": "group"},
			"text":       text,
		}})

	case strings.HasSuffix(path, "/sendRichMessage"):
		_ = r.ParseMultipartForm(1 << 20)
		var payload struct {
			Markdown string `json:"markdown"`
		}
		_ = json.Unmarshal([]byte(r.FormValue("rich_message")), &payload)

		f.mu.Lock()
		f.rich = append(f.rich, payload.Markdown)
		f.mu.Unlock()

		writeJSON(w, map[string]any{"ok": true, "result": map[string]any{
			"message_id": 3,
			"date":       time.Now().Unix(),
			"chat":       map[string]any{"id": 42, "type": "group"},
		}})

	default:
		http.NotFound(w, r)
	}
}

func writeJSON(w http.ResponseWriter, payload any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(payload)
}

// startTestBot runs the bot against a fake Bot API and a temporary vault with
// one teacher and one subject.
func startTestBot(t *testing.T, api *fakeTelegram) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(api.handler))
	t.Cleanup(server.Close)

	dir := t.TempDir()
	writeVaultFile(t, dir, "Преподаватели/Медведев Владислав Сергеевич.md",
		"![[photo.png]]\n### Почта: vmedvedev@kait20.ru\n>[!WARNING] Не подтверждено\n### Что ведет:\n- [[МДК.11.01 Базы данных]]\n")
	writeVaultFile(t, dir, "Предметы/МДК.11.01 Базы данных.md",
		"### Кто ведет: [[Медведев Владислав Сергеевич]]\n### Аттестация\nЗачёт\n")
	writeVaultFile(t, dir, "Аттестация.md",
		"| Зачёт | Работа |\n| --- | --- |\n| [[ОП.05]] | текст |\n\n[сайт](https://example.com/page)\n")

	svc := vault.NewService(vault.NewSyncer("", "master", dir, ""), vault.Dirs{})
	if err := svc.LoadIndex(); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Token:        "123:test",
		APIURL:       server.URL,
		VaultDir:     dir,
		StartTime:    time.Now(),
		SyncInterval: time.Hour,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	app, err := New(ctx, cfg, svc, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	go app.Run(ctx)
}

func writeVaultFile(t *testing.T, root, rel, content string) {
	t.Helper()

	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func waitForReply(t *testing.T, api *fakeTelegram) string {
	t.Helper()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		api.mu.Lock()
		if len(api.sent) > 0 {
			reply := api.sent[0]
			api.mu.Unlock()
			return reply
		}
		api.mu.Unlock()
		time.Sleep(20 * time.Millisecond)
	}

	t.Fatal("bot sent no reply")
	return ""
}

func waitForRich(t *testing.T, api *fakeTelegram) string {
	t.Helper()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		api.mu.Lock()
		if len(api.rich) > 0 {
			markdown := api.rich[0]
			api.mu.Unlock()
			return markdown
		}
		api.mu.Unlock()
		time.Sleep(20 * time.Millisecond)
	}

	t.Fatal("bot sent no rich message")
	return ""
}

func TestEndToEndTeacherLookup(t *testing.T) {
	api := &fakeTelegram{incoming: "/teacher Медведев"}
	startTestBot(t, api)

	reply := waitForReply(t, api)
	for _, want := range []string{"Медведев", "vmedvedev@kait20.ru", "не подтверждена"} {
		if !strings.Contains(reply, want) {
			t.Errorf("reply does not contain %q:\n%s", want, reply)
		}
	}
}

func TestEndToEndTeacherList(t *testing.T) {
	api := &fakeTelegram{incoming: "/teacher"}
	startTestBot(t, api)

	reply := waitForReply(t, api)
	for _, want := range []string{"Преподаватели", "Медведев Владислав Сергеевич", "/teacher Фамилия"} {
		if !strings.Contains(reply, want) {
			t.Errorf("reply does not contain %q:\n%s", want, reply)
		}
	}
	if strings.Contains(reply, "Укажите имя") {
		t.Errorf("empty /teacher must list options, got:\n%s", reply)
	}
}

func TestEndToEndSubjectList(t *testing.T) {
	api := &fakeTelegram{incoming: "/subject"}
	startTestBot(t, api)

	reply := waitForReply(t, api)
	for _, want := range []string{"Предметы", "МДК.11.01 Базы данных", "/subject Предмет"} {
		if !strings.Contains(reply, want) {
			t.Errorf("reply does not contain %q:\n%s", want, reply)
		}
	}
	if strings.Contains(reply, "Укажите предмет") {
		t.Errorf("empty /subject must list options, got:\n%s", reply)
	}
}

func TestEndToEndFindUsesRichMessage(t *testing.T) {
	api := &fakeTelegram{incoming: "/find Аттестация"}
	startTestBot(t, api)

	markdown := waitForRich(t, api)
	for _, want := range []string{
		"# Аттестация",
		"| Зачёт | Работа |",
		"| --- | --- |",
		"ОП.05",
		"[сайт](https://example.com/page)",
	} {
		if !strings.Contains(markdown, want) {
			t.Errorf("rich markdown does not contain %q:\n%s", want, markdown)
		}
	}
	if strings.Contains(markdown, "[[") {
		t.Errorf("wikilinks were not preprocessed:\n%s", markdown)
	}
}

func TestEndToEndSubjectUsesRichMessage(t *testing.T) {
	api := &fakeTelegram{incoming: "/subject МДК.11.01"}
	startTestBot(t, api)

	markdown := waitForRich(t, api)
	for _, want := range []string{"# МДК.11.01 Базы данных", "## Аттестация", "Зачёт"} {
		if !strings.Contains(markdown, want) {
			t.Errorf("rich markdown does not contain %q:\n%s", want, markdown)
		}
	}
}
