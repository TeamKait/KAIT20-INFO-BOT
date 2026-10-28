package bot

import (
	"context"
	"testing"
	"time"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"infobot/internal/config"
)

func updateAt(chatID int64, at time.Time) *models.Update {
	return &models.Update{Message: &models.Message{
		Date: int(at.Unix()),
		Chat: models.Chat{ID: chatID},
	}}
}

func TestGuardDropsOldMessagesWithoutOffset(t *testing.T) {
	app := &App{start: time.Now(), cfg: &config.Config{}, dropOldByDate: true}
	calls := 0
	handler := app.guard(func(context.Context, *tgbot.Bot, *models.Update) { calls++ })

	handler(context.Background(), nil, updateAt(1, time.Now().Add(-time.Hour)))
	if calls != 0 {
		t.Fatal("old message was handled")
	}

	handler(context.Background(), nil, updateAt(1, time.Now()))
	if calls != 1 {
		t.Fatal("fresh message was not handled")
	}
}

func TestGuardKeepsMessagesWithoutDateFallback(t *testing.T) {
	// When the getUpdates offset worked, the backlog is already skipped and the
	// date must not be used: a skewed clock would drop fresh messages.
	app := &App{start: time.Now(), cfg: &config.Config{}}
	calls := 0
	handler := app.guard(func(context.Context, *tgbot.Bot, *models.Update) { calls++ })

	handler(context.Background(), nil, updateAt(1, time.Now().Add(-time.Hour)))
	if calls != 1 {
		t.Fatal("date filter must be disabled when the offset is known")
	}
}

func TestGuardAllowedChats(t *testing.T) {
	app := &App{
		start: time.Now(),
		cfg:   &config.Config{AllowedChats: map[int64]struct{}{5: {}}},
	}
	handled := make([]int64, 0, 2)
	handler := app.guard(func(_ context.Context, _ *tgbot.Bot, u *models.Update) {
		handled = append(handled, u.Message.Chat.ID)
	})

	handler(context.Background(), nil, updateAt(6, time.Now()))
	handler(context.Background(), nil, updateAt(5, time.Now()))

	if len(handled) != 1 || handled[0] != 5 {
		t.Fatalf("handled chats = %v, want [5]", handled)
	}
}
