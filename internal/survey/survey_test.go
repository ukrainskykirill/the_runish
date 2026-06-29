package survey

import (
	"reflect"
	"testing"
)

func TestBranchFor(t *testing.T) {
	cases := map[int]string{
		0: BranchNovice,
		1: BranchCasual,
		2: BranchCasual,
		3: BranchRegular,
		4: BranchRegular,
	}
	for idx, want := range cases {
		if got := BranchFor(idx); got != want {
			t.Errorf("BranchFor(%d) = %q, want %q", idx, got, want)
		}
	}
}

func TestSequence(t *testing.T) {
	cases := map[string][]string{
		BranchNovice:  {"name", "experience", "motivation", "health", "comfort_start", "freq", "format", "time", "important", "notes"},
		BranchCasual:  {"name", "experience", "last_run", "distance", "pace", "goal", "format", "time", "important", "notes"},
		BranchRegular: {"name", "experience", "freq_reg", "volume", "distances", "goals", "races", "format", "time", "important", "notes"},
		// до выбора ветки — только префикс+хвост
		"": {"name", "experience", "format", "time", "important", "notes"},
	}
	for branch, want := range cases {
		if got := Sequence(branch); !reflect.DeepEqual(got, want) {
			t.Errorf("Sequence(%q) = %v, want %v", branch, got, want)
		}
	}
}

func TestNextWalksWholeBranch(t *testing.T) {
	for _, branch := range []string{BranchNovice, BranchCasual, BranchRegular} {
		seq := Sequence(branch)
		cur := FirstStep()
		walked := []string{cur}
		for {
			next, done := Next(branch, cur)
			if done {
				break
			}
			walked = append(walked, next)
			cur = next
		}
		if !reflect.DeepEqual(walked, seq) {
			t.Errorf("branch %q walk = %v, want %v", branch, walked, seq)
		}
		// последний шаг должен давать done
		if _, done := Next(branch, seq[len(seq)-1]); !done {
			t.Errorf("branch %q: Next on last step should be done", branch)
		}
	}
}

func TestEveryStepHasDefinition(t *testing.T) {
	for _, branch := range []string{BranchNovice, BranchCasual, BranchRegular} {
		for _, key := range Sequence(branch) {
			st, ok := Get(key)
			if !ok {
				t.Fatalf("no definition for step %q", key)
			}
			if st.Text == "" || st.Label == "" {
				t.Errorf("step %q missing text/label", key)
			}
			if (st.Kind == Single || st.Kind == Multi) && len(st.Options) == 0 {
				t.Errorf("step %q is choice but has no options", key)
			}
		}
	}
}

func TestRenderAnswers(t *testing.T) {
	answers := map[string]any{
		"name":       "Кирилл",
		"experience": "Бегаю регулярно",
		"distances":  []any{"5 км", "10 км"},
	}
	got := RenderAnswers(BranchRegular, answers)
	want := []QA{
		{Question: "Имя", Answer: "Кирилл"},
		{Question: "Беговой опыт", Answer: "Бегаю регулярно"},
		{Question: "Интересные дистанции", Answer: "5 км, 10 км"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("RenderAnswers = %v, want %v", got, want)
	}
}
