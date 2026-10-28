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
type fakeButton struct {
	Text         string `json:"text"`
	CallbackData string `json:"callback_data"`
}

type fakeTelegram struct {
	mu           sync.Mutex
	sent         []string
	rich         []string
	media        []string
	files        []string
	buttons      []fakeButton
	updateID     int64
	incoming     string
	callbackData string
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
			if f.callbackData != "" {
				writeJSON(w, map[string]any{"ok": true, "result": []any{map[string]any{
					"update_id": 1,
					"callback_query": map[string]any{
						"id":   "cb1",
						"from": map[string]any{"id": 7, "is_bot": false, "first_name": "Test"},
						"message": map[string]any{
							"message_id": 10,
							"date":       time.Now().Unix(),
							"chat":       map[string]any{"id": 42, "type": "group"},
						},
						"chat_instance": "test",
						"data":          f.callbackData,
					},
				}}})
				return
			}
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

	case strings.HasSuffix(path, "/answerCallbackQuery"):
		writeJSON(w, map[string]any{"ok": true, "result": true})

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
			Media    []struct {
				ID string `json:"id"`
			} `json:"media"`
		}
		_ = json.Unmarshal([]byte(r.FormValue("rich_message")), &payload)

		var markup struct {
			InlineKeyboard [][]fakeButton `json:"inline_keyboard"`
		}
		_ = json.Unmarshal([]byte(r.FormValue("reply_markup")), &markup)

		ids := make([]string, 0, len(payload.Media))
		for _, item := range payload.Media {
			ids = append(ids, item.ID)
		}

		f.mu.Lock()
		f.rich = append(f.rich, payload.Markdown)
		f.media = append(f.media, ids...)
		for _, row := range markup.InlineKeyboard {
			f.buttons = append(f.buttons, row...)
		}
		if r.MultipartForm != nil {
			for name := range r.MultipartForm.File {
				f.files = append(f.files, name)
			}
		}
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
		"![[Медведев Владислав Сергеевич.png]]\n### Почта: vmedvedev@kait20.ru\n>[!WARNING] Не подтверждено\n>Почта не подтверждена преподавателем\n### Что ведет:\n- [[МДК.11.01 Базы данных]]\n")
	writeVaultFile(t, dir, "Прочее/Картинки/Медведев Владислав Сергеевич.png", "fake-image-bytes")
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

func richMediaIDs(api *fakeTelegram) []string {
	api.mu.Lock()
	defer api.mu.Unlock()
	return append([]string(nil), api.media...)
}

func richFiles(api *fakeTelegram) []string {
	api.mu.Lock()
	defer api.mu.Unlock()
	return append([]string(nil), api.files...)
}

func richButtons(api *fakeTelegram) []fakeButton {
	api.mu.Lock()
	defer api.mu.Unlock()
	return append([]fakeButton(nil), api.buttons...)
}

func TestEndToEndTeacherLookup(t *testing.T) {
	api := &fakeTelegram{incoming: "/teacher Медведев"}
	startTestBot(t, api)

	markdown := waitForRich(t, api)
	for _, want := range []string{
		"# Медведев Владислав Сергеевич",
		"![](tg://photo?id=img1)",
		"### Почта: vmedvedev@kait20.ru",
		"### Что ведет",
		"МДК.11.01",
	} {
		if !strings.Contains(markdown, want) {
			t.Errorf("rich markdown does not contain %q:\n%s", want, markdown)
		}
	}
	if strings.Contains(markdown, "![[") {
		t.Errorf("image embed was not replaced:\n%s", markdown)
	}

	ids := richMediaIDs(api)
	if len(ids) != 1 || ids[0] != "img1" {
		t.Errorf("media ids = %v, want [img1]", ids)
	}
	files := richFiles(api)
	if len(files) != 1 || files[0] != "img1.png" {
		t.Errorf("uploaded files = %v, want [img1.png]", files)
	}

	buttons := richButtons(api)
	if len(buttons) != 1 || buttons[0].Text != "Карточка МДК.11.01 Базы данных" {
		t.Fatalf("buttons = %v, want one subject button", buttons)
	}
	if !strings.HasPrefix(buttons[0].CallbackData, "s:") {
		t.Errorf("callback data = %q, want s: prefix", buttons[0].CallbackData)
	}
}

func TestEndToEndEmail(t *testing.T) {
	api := &fakeTelegram{incoming: "/email Медведев"}
	startTestBot(t, api)

	reply := waitForReply(t, api)
	for _, want := range []string{"vmedvedev@kait20.ru", "Не подтверждена"} {
		if !strings.Contains(reply, want) {
			t.Errorf("reply does not contain %q:\n%s", want, reply)
		}
	}
}

func TestEndToEndStartUsesPlaceholders(t *testing.T) {
	api := &fakeTelegram{incoming: "/start"}
	startTestBot(t, api)

	reply := waitForReply(t, api)
	for _, want := range []string{
		"/teacher &lt;имя&gt;",
		"/email &lt;имя&gt;",
		"/subject &lt;название&gt;",
		"/find &lt;запрос&gt;",
	} {
		if !strings.Contains(reply, want) {
			t.Errorf("start text does not contain %q:\n%s", want, reply)
		}
	}
}

func TestEndToEndTeacherList(t *testing.T) {
	api := &fakeTelegram{incoming: "/teacher"}
	startTestBot(t, api)

	reply := waitForReply(t, api)
	for _, want := range []string{"Преподаватели", "Медведев Владислав Сергеевич", "/email &lt;имя&gt;"} {
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
	for _, want := range []string{"Предметы", "МДК.11.01 Базы данных", "/subject &lt;название&gt;"} {
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
		"| Зачёт | Работа |",
		"| --- | --- |",
		"ОП.05",
		"[сайт](https://example.com/page)",
	} {
		if !strings.Contains(markdown, want) {
			t.Errorf("rich markdown does not contain %q:\n%s", want, markdown)
		}
	}
	if !strings.HasPrefix(markdown, "| Зачёт | Работа |") {
		t.Errorf("note must be sent without a synthetic header:\n%s", markdown)
	}
	if strings.Contains(markdown, "Открыть на GitHub") {
		t.Errorf("note must keep its original form without the GitHub footer:\n%s", markdown)
	}
	if strings.Contains(markdown, "[[") {
		t.Errorf("wikilinks were not preprocessed:\n%s", markdown)
	}
}

func TestEndToEndSubjectUsesRichMessage(t *testing.T) {
	api := &fakeTelegram{incoming: "/subject МДК.11.01"}
	startTestBot(t, api)

	markdown := waitForRich(t, api)
	for _, want := range []string{
		"# МДК.11.01 Базы данных",
		"### Кто ведет: Медведев Владислав Сергеевич",
		"### Аттестация",
		"Зачёт",
	} {
		if !strings.Contains(markdown, want) {
			t.Errorf("rich markdown does not contain %q:\n%s", want, markdown)
		}
	}

	buttons := richButtons(api)
	if len(buttons) != 1 || buttons[0].Text != "Карточка преподавателя" {
		t.Fatalf("buttons = %v, want teacher card button", buttons)
	}
	if !strings.HasPrefix(buttons[0].CallbackData, "t:") {
		t.Errorf("callback data = %q, want t: prefix", buttons[0].CallbackData)
	}
}

func TestEndToEndCallbackOpensTeacher(t *testing.T) {
	api := &fakeTelegram{
		callbackData: "t:" + noteID("Преподаватели/Медведев Владислав Сергеевич.md"),
	}
	startTestBot(t, api)

	markdown := waitForRich(t, api)
	for _, want := range []string{"# Медведев Владислав Сергеевич", "### Почта: vmedvedev@kait20.ru"} {
		if !strings.Contains(markdown, want) {
			t.Errorf("rich markdown does not contain %q:\n%s", want, markdown)
		}
	}
}

func TestEndToEndCallbackOpensSubject(t *testing.T) {
	api := &fakeTelegram{
		callbackData: "s:" + noteID("Предметы/МДК.11.01 Базы данных.md"),
	}
	startTestBot(t, api)

	markdown := waitForRich(t, api)
	if !strings.Contains(markdown, "# МДК.11.01 Базы данных") {
		t.Errorf("rich markdown does not open the subject:\n%s", markdown)
	}
}
