package bot

import (
	"strings"
	"testing"

	"infobot/internal/vault"
)

func testApp() *App {
	return &App{repo: newRepoLink("https://github.com/TeamKait/KAIT20-INFO.git", "master")}
}

func TestTeacherCard(t *testing.T) {
	card := testApp().teacherCard(&vault.Teacher{
		Name:             "Медведев Владислав Сергеевич",
		Email:            "vmedvedev@kait20.ru",
		EmailUnconfirmed: true,
		Subjects:         []string{"МДК.11.01 Технология разработки и защиты баз данных"},
		Path:             "Преподаватели/Медведев Владислав Сергеевич.md",
	})

	for _, want := range []string{
		"Медведев Владислав Сергеевич",
		"vmedvedev@kait20.ru",
		"не подтверждена",
		"МДК.11.01",
		"https://github.com/TeamKait/KAIT20-INFO/blob/master/",
	} {
		if !strings.Contains(card, want) {
			t.Errorf("card does not contain %q:\n%s", want, card)
		}
	}
}

func TestTeacherCardWithoutEmail(t *testing.T) {
	card := testApp().teacherCard(&vault.Teacher{Name: "Без Почты"})
	if !strings.Contains(card, "почта не указана") {
		t.Errorf("card = %q", card)
	}
}

func TestTeacherEmailText(t *testing.T) {
	app := testApp()

	text := app.teacherEmailText(&vault.Teacher{Name: "Волкова", Email: "uvolkova@kait20.ru", EmailUnconfirmed: true})
	if !strings.Contains(text, "uvolkova@kait20.ru") || !strings.Contains(text, "Не подтверждена") {
		t.Errorf("email text = %q", text)
	}

	text = app.teacherEmailText(&vault.Teacher{Name: "Без Почты"})
	if !strings.Contains(text, "почта не указана") {
		t.Errorf("email text = %q", text)
	}
}

func TestTeachersList(t *testing.T) {
	text := teachersList([]*vault.Teacher{
		{Name: "Волкова Юлия Александровна"},
		{Name: "Медведев Владислав Сергеевич"},
	})

	for _, want := range []string{"Преподаватели", "— 2", "Волкова", "Медведев", "/email &lt;имя&gt;"} {
		if !strings.Contains(text, want) {
			t.Errorf("list does not contain %q:\n%s", want, text)
		}
	}
}

func TestSubjectsList(t *testing.T) {
	text := subjectsList([]*vault.Subject{
		{Name: "МДК.11.01 Базы данных"},
		{Name: "УП.14 Учебная практика"},
	})

	for _, want := range []string{"Предметы", "— 2", "МДК.11.01", "УП.14", "/subject &lt;название&gt;"} {
		if !strings.Contains(text, want) {
			t.Errorf("list does not contain %q:\n%s", want, text)
		}
	}
}
