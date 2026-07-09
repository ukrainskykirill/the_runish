package survey

import (
	"reflect"
	"testing"

	"therunish/internal/domain"
)

func testTemplate() *Template {
	return Build([]domain.SurveyQuestion{
		{Key: "name", Phase: domain.SurveyPhaseIntro, Kind: domain.SurveyKindText, Label: "Имя", Prompt: "Как тебя зовут?", Position: 0},
		{Key: "experience", Phase: domain.SurveyPhaseIntro, Kind: domain.SurveyKindSingle, Label: "Беговой опыт", Prompt: "Какой опыт?", Position: 1, IsSelector: true, Options: []domain.SurveyOption{
			{Label: "Не бегал", Branch: BranchNovice},
			{Label: "Иногда", Branch: BranchCasual},
			{Label: "Регулярно", Branch: BranchRegular},
		}},
		{Key: "motivation", Phase: domain.SurveyPhaseBranch, Branch: BranchNovice, Kind: domain.SurveyKindSingle, Label: "Мотивация", Prompt: "Почему?", Position: 0, Options: []domain.SurveyOption{{Label: "Двигаться"}}},
		{Key: "distance", Phase: domain.SurveyPhaseBranch, Branch: BranchCasual, Kind: domain.SurveyKindSingle, Label: "Дистанция", Prompt: "Какая?", Position: 0, Options: []domain.SurveyOption{{Label: "5 км"}}},
		{Key: "volume", Phase: domain.SurveyPhaseBranch, Branch: BranchRegular, Kind: domain.SurveyKindSingle, Label: "Объём", Prompt: "Сколько?", Position: 0, Options: []domain.SurveyOption{{Label: "40+ км"}}},
		{Key: "distances", Phase: domain.SurveyPhaseBranch, Branch: BranchRegular, Kind: domain.SurveyKindMulti, Label: "Интересные дистанции", Prompt: "Что интересно?", Position: 1, Options: []domain.SurveyOption{{Label: "5 км"}, {Label: "10 км"}}},
		{Key: "notes", Phase: domain.SurveyPhaseOutro, Kind: domain.SurveyKindText, Label: "Заметки", Prompt: "Ещё что-то?", Position: 0},
	})
}

func TestBranchForOption(t *testing.T) {
	tmpl := testTemplate()
	st, ok := tmpl.Get("experience")
	if !ok || !st.IsSelector {
		t.Fatal("experience must be a selector step")
	}
	cases := map[int]string{0: BranchNovice, 1: BranchCasual, 2: BranchRegular}
	for idx, want := range cases {
		if got := BranchForOption(st, idx); got != want {
			t.Errorf("BranchForOption(%d) = %q, want %q", idx, got, want)
		}
	}
	if got := BranchForOption(st, 99); got != "" {
		t.Errorf("BranchForOption(out of range) = %q, want empty", got)
	}
}

func TestSequence(t *testing.T) {
	tmpl := testTemplate()
	cases := map[string][]string{
		BranchNovice:  {"name", "experience", "motivation", "notes"},
		BranchCasual:  {"name", "experience", "distance", "notes"},
		BranchRegular: {"name", "experience", "volume", "distances", "notes"},
		"":            {"name", "experience", "notes"},
	}
	for branch, want := range cases {
		if got := tmpl.Sequence(branch); !reflect.DeepEqual(got, want) {
			t.Errorf("Sequence(%q) = %v, want %v", branch, got, want)
		}
	}
}

func TestNextWalksWholeBranch(t *testing.T) {
	tmpl := testTemplate()
	for _, branch := range []string{BranchNovice, BranchCasual, BranchRegular} {
		seq := tmpl.Sequence(branch)
		cur := tmpl.FirstStep()
		walked := []string{cur}
		for {
			next, done := tmpl.Next(branch, cur)
			if done {
				break
			}
			walked = append(walked, next)
			cur = next
		}
		if !reflect.DeepEqual(walked, seq) {
			t.Errorf("branch %q walk = %v, want %v", branch, walked, seq)
		}
		if _, done := tmpl.Next(branch, seq[len(seq)-1]); !done {
			t.Errorf("branch %q: Next on last step should be done", branch)
		}
	}
}

func TestRenderAnswers(t *testing.T) {
	tmpl := testTemplate()
	answers := map[string]any{
		"name":       "Кирилл",
		"experience": "Регулярно",
		"distances":  []any{"5 км", "10 км"},
	}
	got := tmpl.RenderAnswers(BranchRegular, answers)
	want := []QA{
		{Question: "Имя", Answer: "Кирилл"},
		{Question: "Беговой опыт", Answer: "Регулярно"},
		{Question: "Интересные дистанции", Answer: "5 км, 10 км"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("RenderAnswers = %v, want %v", got, want)
	}
}
