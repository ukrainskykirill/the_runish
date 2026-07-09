package survey

import (
	"sort"
	"strings"

	"therunish/internal/domain"
)

type Kind int

const (
	Single Kind = iota
	Multi
	Text
)

const (
	BranchNovice  = "novice"
	BranchCasual  = "casual"
	BranchRegular = "regular"
)

type Option struct {
	Label  string
	Branch string
}

type Step struct {
	Key        string
	Label      string
	Text       string
	Kind       Kind
	Options    []Option
	IsSelector bool
	Phase      string
	Branch     string
	Position   int
}

func (s Step) OptionLabels() []string {
	out := make([]string, len(s.Options))
	for i, o := range s.Options {
		out[i] = o.Label
	}
	return out
}

type Template struct {
	steps    []Step
	registry map[string]Step
}

func kindFromString(k string) Kind {
	switch k {
	case domain.SurveyKindMulti:
		return Multi
	case domain.SurveyKindText:
		return Text
	default:
		return Single
	}
}

func Build(questions []domain.SurveyQuestion) *Template {
	steps := make([]Step, 0, len(questions))
	for _, q := range questions {
		opts := make([]Option, len(q.Options))
		for i, o := range q.Options {
			opts[i] = Option{Label: o.Label, Branch: o.Branch}
		}
		steps = append(steps, Step{
			Key:        q.Key,
			Label:      q.Label,
			Text:       q.Prompt,
			Kind:       kindFromString(q.Kind),
			Options:    opts,
			IsSelector: q.IsSelector,
			Phase:      q.Phase,
			Branch:     q.Branch,
			Position:   q.Position,
		})
	}
	sort.SliceStable(steps, func(i, j int) bool {
		if pi, pj := phaseOrder(steps[i].Phase), phaseOrder(steps[j].Phase); pi != pj {
			return pi < pj
		}
		if steps[i].Branch != steps[j].Branch {
			return steps[i].Branch < steps[j].Branch
		}
		return steps[i].Position < steps[j].Position
	})

	reg := make(map[string]Step, len(steps))
	for _, s := range steps {
		reg[s.Key] = s
	}
	return &Template{steps: steps, registry: reg}
}

func phaseOrder(phase string) int {
	switch phase {
	case domain.SurveyPhaseIntro:
		return 0
	case domain.SurveyPhaseBranch:
		return 1
	case domain.SurveyPhaseOutro:
		return 2
	default:
		return 3
	}
}

func (t *Template) Get(key string) (Step, bool) {
	s, ok := t.registry[key]
	return s, ok
}

func (t *Template) intro() []Step { return t.phaseSteps(domain.SurveyPhaseIntro, "") }
func (t *Template) outro() []Step { return t.phaseSteps(domain.SurveyPhaseOutro, "") }

func (t *Template) phaseSteps(phase, branch string) []Step {
	var out []Step
	for _, s := range t.steps {
		if s.Phase != phase {
			continue
		}
		if phase == domain.SurveyPhaseBranch && s.Branch != branch {
			continue
		}
		out = append(out, s)
	}
	return out
}

func (t *Template) FirstStep() string {
	if intro := t.intro(); len(intro) > 0 {
		return intro[0].Key
	}
	if len(t.steps) > 0 {
		return t.steps[0].Key
	}
	return ""
}

func BranchForOption(st Step, idx int) string {
	if idx < 0 || idx >= len(st.Options) {
		return ""
	}
	return st.Options[idx].Branch
}

func (t *Template) Sequence(branch string) []string {
	ordered := make([]Step, 0, len(t.steps))
	ordered = append(ordered, t.intro()...)
	if branch != "" {
		ordered = append(ordered, t.phaseSteps(domain.SurveyPhaseBranch, branch)...)
	}
	ordered = append(ordered, t.outro()...)

	keys := make([]string, len(ordered))
	for i, s := range ordered {
		keys[i] = s.Key
	}
	return keys
}

func (t *Template) Next(branch, cur string) (next string, done bool) {
	seq := t.Sequence(branch)
	for i, k := range seq {
		if k == cur {
			if i+1 < len(seq) {
				return seq[i+1], false
			}
			return "", true
		}
	}
	return "", true
}

type QA struct {
	Question string
	Answer   string
}

func (t *Template) RenderAnswers(branch string, answers map[string]any) []QA {
	out := make([]QA, 0, len(answers))
	for _, key := range t.Sequence(branch) {
		st, ok := t.registry[key]
		if !ok {
			continue
		}
		v, ok := answers[key]
		if !ok {
			continue
		}
		out = append(out, QA{Question: st.Label, Answer: formatAnswer(v)})
	}
	return out
}

func formatAnswer(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case []any:
		parts := make([]string, 0, len(val))
		for _, x := range val {
			if s, ok := x.(string); ok {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, ", ")
	case []string:
		return strings.Join(val, ", ")
	default:
		return ""
	}
}

func Greeting() string {
	return "Привет! 👋\n" +
		"Рады видеть тебя в нашем беговом клубе.\n" +
		"Сейчас зададим несколько коротких вопросов, чтобы понять твой беговой опыт, цели и подобрать комфортный формат участия.\n" +
		"Обещаем: это не экзамен, нормативы сдавать не нужно — только познакомимся 🙂"
}

func Final() string {
	return "Спасибо! 🏃‍♂️🏃‍♀️\n" +
		"Мы сохранили твои ответы.\n\n" +
		"Теперь мы понимаем твой уровень и сможем предложить подходящий формат: от мягкого старта до тренировок под конкретную дистанцию.\n\n" +
		"Добро пожаловать в клуб — побежали знакомиться с бегом без героизма, но с прогрессом 🙂"
}
