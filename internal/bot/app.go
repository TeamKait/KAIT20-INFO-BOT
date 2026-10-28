package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"infobot/internal/config"
	"infobot/internal/vault"
)

// App wires Telegram updates to the vault index.
type App struct {
	tg    *tgbot.Bot
	cfg   *config.Config
	vault *vault.Service
	start time.Time
	repo  repoLink
	log   *slog.Logger

	// dropOldByDate is enabled only when the pending-update offset could not be
	// read from Telegram. Normally the offset skips the backlog exactly, and a
	// clock-based filter could wrongly drop fresh messages on a skewed VPS.
	dropOldByDate bool
}

// New creates the bot, skipping updates that were already pending before the
// process started.
func New(ctx context.Context, cfg *config.Config, svc *vault.Service, log *slog.Logger) (*App, error) {
	app := &App{
		cfg:   cfg,
		vault: svc,
		start: cfg.StartTime,
		repo:  newRepoLink(cfg.VaultURL, cfg.VaultBranch),
		log:   log,
	}

	offset, err := lastUpdateID(ctx, cfg.APIURL, cfg.Token)
	if err != nil {
		log.Warn("could not read pending updates; old messages will be filtered by date", "err", err)
		app.dropOldByDate = true
	}
	if offset > 0 {
		log.Info("pending updates will be skipped", "last_update_id", offset)
	}

	tg, err := tgbot.New(cfg.Token,
		tgbot.WithServerURL(cfg.APIURL),
		tgbot.WithInitialOffset(offset),
		tgbot.WithErrorsHandler(func(err error) { log.Warn("telegram api", "err", err) }),
		tgbot.WithDefaultHandler(func(context.Context, *tgbot.Bot, *models.Update) {}),
	)
	if err != nil {
		return nil, err
	}
	app.tg = tg

	tg.RegisterHandler(tgbot.HandlerTypeMessageText, "/", tgbot.MatchTypePrefix, app.guard(app.dispatch))

	if _, err := tg.SetMyCommands(ctx, &tgbot.SetMyCommandsParams{
		Commands: []models.BotCommand{
			{Command: "start", Description: "Справка и примеры"},
			{Command: "teacher", Description: "Преподаватель: /teacher Иванов email"},
			{Command: "subject", Description: "Информация о предмете"},
			{Command: "find", Description: "Найти заметку целиком"},
			{Command: "sync", Description: "Обновить данные из GitHub"},
		},
	}); err != nil {
		log.Warn("set bot commands", "err", err)
	}

	return app, nil
}

// Run blocks until the context is cancelled.
func (a *App) Run(ctx context.Context) {
	a.tg.Start(ctx)
}

// guard drops old messages and chats outside the optional allowlist.
func (a *App) guard(next tgbot.HandlerFunc) tgbot.HandlerFunc {
	return func(ctx context.Context, b *tgbot.Bot, update *models.Update) {
		msg := update.Message
		if msg == nil {
			return
		}
		// Messages sent before the bot started (for example while it was being
		// restarted) must not be answered. This is a fallback: the getUpdates
		// offset normally skips the backlog without any clock comparison.
		if a.dropOldByDate && msg.Date > 0 && int64(msg.Date) < a.start.Unix()-60 {
			return
		}
		if len(a.cfg.AllowedChats) > 0 {
			if _, ok := a.cfg.AllowedChats[msg.Chat.ID]; !ok {
				return
			}
		}
		next(ctx, b, update)
	}
}

func (a *App) dispatch(ctx context.Context, _ *tgbot.Bot, update *models.Update) {
	msg := update.Message
	cmd, args := splitCommand(msg.Text)

	switch cmd {
	case "start", "help":
		a.handleStart(ctx, msg.Chat.ID)
	case "teacher":
		a.handleTeacher(ctx, msg.Chat.ID, args)
	case "subject":
		a.handleSubject(ctx, msg.Chat.ID, args)
	case "find":
		a.handleFind(ctx, msg.Chat.ID, args)
	case "sync":
		a.handleSync(ctx, msg.Chat.ID)
	}
}

// splitCommand extracts the command name and the rest of the message. It also
// strips the "@botname" suffix used in group chats.
func splitCommand(text string) (string, string) {
	text = strings.TrimSpace(text)
	if !strings.HasPrefix(text, "/") {
		return "", ""
	}

	field, rest := text, ""
	if i := strings.IndexAny(text, " \n\t"); i >= 0 {
		field, rest = text[:i], strings.TrimSpace(text[i+1:])
	}
	if i := strings.Index(field, "@"); i >= 0 {
		field = field[:i]
	}
	return strings.ToLower(strings.TrimPrefix(field, "/")), rest
}

// lastUpdateID asks Telegram for the newest pending update id. Passing it as
// the initial offset makes the library start from the next update.
func lastUpdateID(ctx context.Context, apiURL, token string) (int64, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	url := strings.TrimRight(apiURL, "/") + "/bot" + token + "/getUpdates?offset=-1&limit=1"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, fmt.Errorf("build getUpdates request: %s", redactToken(err.Error(), token))
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		// Do not leak the bot token from the request URL into logs.
		return 0, fmt.Errorf("getUpdates request failed: %s", redactToken(err.Error(), token))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, nil
	}

	var payload struct {
		OK     bool `json:"ok"`
		Result []struct {
			UpdateID int64 `json:"update_id"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return 0, err
	}
	if !payload.OK || len(payload.Result) == 0 {
		return 0, nil
	}
	return payload.Result[len(payload.Result)-1].UpdateID, nil
}

func redactToken(message, token string) string {
	if token == "" {
		return message
	}
	return strings.ReplaceAll(message, token, "***")
}
