package search

import (
	"testing"

	"infobot/internal/vault"
)

func testIndex() *vault.Index {
	return &vault.Index{
		Teachers: []*vault.Teacher{
			{Name: "Медведев Владислав Сергеевич"},
			{Name: "Кузнецова Марина Геннадьевна"},
			{Name: "Шихрагимов Шихрагим Темирланович"},
			{Name: "Волкова Юлия Александровна"},
		},
		Subjects: []*vault.Subject{
			{Name: "МДК.11.01 Технология разработки и защиты баз данных"},
			{Name: "МДК.01.02 Поддержка и тестирование программных модулей"},
			{Name: "ОП.07 Экономика отрасли"},
		},
		Notes: []*vault.Note{
			{Title: "Аттестация", Path: "Аттестация.md", Body: "Предметы в последнем столбце не влияют на стипендию"},
		},
	}
}

func TestTeachers(t *testing.T) {
	idx := testIndex()
	cases := []struct {
		query string
		want  string
		min   float64
	}{
		{"медведев", "Медведев Владислав Сергеевич", 0.8},
		{"МЕДВЕДЕВ ВЛАДИСЛАВ", "Медведев Владислав Сергеевич", 0.9},
		{"медведев в с", "Медведев Владислав Сергеевич", 0.9},
		{"кузнецов", "Кузнецова Марина Геннадьевна", 0.8},
		{"шихрагимова", "Шихрагимов Шихрагим Темирланович", 0.8},
		{"волкова", "Волкова Юлия Александровна", 0.8},
	}

	for _, tc := range cases {
		got := Teachers(idx, tc.query)
		if len(got) == 0 {
			t.Errorf("Teachers(%q) returned nothing", tc.query)
			continue
		}
		if got[0].Teacher.Name != tc.want {
			t.Errorf("Teachers(%q)[0] = %q, want %q (score %.2f)", tc.query, got[0].Teacher.Name, tc.want, got[0].Score)
		}
		if got[0].Score < tc.min {
			t.Errorf("Teachers(%q)[0].Score = %.2f, want >= %.2f", tc.query, got[0].Score, tc.min)
		}
	}
}

func TestTeachersNoMatch(t *testing.T) {
	got := Teachers(testIndex(), "петров")
	if len(got) == 0 {
		return
	}
	if got[0].Score >= 0.55 {
		t.Errorf("петров matched %q with score %.2f", got[0].Teacher.Name, got[0].Score)
	}
}

func TestSubjects(t *testing.T) {
	idx := testIndex()
	cases := []struct {
		query string
		want  string
	}{
		{"мдк 11.01", "МДК.11.01 Технология разработки и защиты баз данных"},
		{"мдк 1.2", "МДК.01.02 Поддержка и тестирование программных модулей"},
		{"оп 7", "ОП.07 Экономика отрасли"},
		{"экономика", "ОП.07 Экономика отрасли"},
	}

	for _, tc := range cases {
		got := Subjects(idx, tc.query)
		if len(got) == 0 || got[0].Subject.Name != tc.want {
			names := make([]string, 0, len(got))
			for _, m := range got {
				names = append(names, m.Subject.Name)
			}
			t.Errorf("Subjects(%q) = %v, want %q first", tc.query, names, tc.want)
		}
	}
}

func TestDocuments(t *testing.T) {
	idx := testIndex()

	got := Documents(idx, "аттестация")
	if len(got) == 0 || got[0].Title != "Аттестация" {
		t.Fatalf("Documents(аттестация) = %v", got)
	}

	got = Documents(idx, "стипендию")
	if len(got) != 1 || got[0].Title != "Аттестация" {
		t.Fatalf("content search = %v, want the Аттестация note", got)
	}
	if got[0].Score >= 0.55 {
		t.Errorf("content match score = %.2f, want < 0.55", got[0].Score)
	}
}

func TestUniqueBySubstring(t *testing.T) {
	idx := testIndex()

	subject := UniqueSubjectBySubstring(idx, "мдк.01")
	if subject == nil || subject.Name != "МДК.01.02 Поддержка и тестирование программных модулей" {
		t.Errorf("UniqueSubjectBySubstring(мдк.01) = %v", subject)
	}
	if got := UniqueSubjectBySubstring(idx, "мдк"); got != nil {
		t.Errorf("several subjects contain 'мдк', got %q", got.Name)
	}
	if got := UniqueSubjectBySubstring(idx, "экономика"); got == nil {
		t.Error("expected the economy subject")
	}
	if got := UniqueSubjectBySubstring(idx, "нет такого"); got != nil {
		t.Errorf("unexpected match: %q", got.Name)
	}

	teacher := UniqueTeacherBySubstring(idx, "шихрагим")
	if teacher == nil || teacher.Name != "Шихрагимов Шихрагим Темирланович" {
		t.Errorf("UniqueTeacherBySubstring(шихрагим) = %v", teacher)
	}
}

func TestNormalize(t *testing.T) {
	cases := map[string]string{
		"МДК.01.02":     "мдк 1 2",
		"ОП.07":         "оп 7",
		"Ёжик в тумане": "ежик в тумане",
		"  Медведев  ":  "медведев",
	}
	for in, want := range cases {
		if got := Normalize(in); got != want {
			t.Errorf("Normalize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLevenshtein(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"кот", "кот", 0},
		{"кот", "кит", 1},
		{"кот", "", 3},
		{"шихрагимов", "шихрагимова", 1},
	}
	for _, tc := range cases {
		if got := Levenshtein(tc.a, tc.b); got != tc.want {
			t.Errorf("Levenshtein(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}
