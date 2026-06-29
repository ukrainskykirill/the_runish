// Package survey описывает онбординг-анкету бегового клуба: вопросы, ветвление
// и порядок шагов. Пакет чистый — без Telegram и БД, чтобы логику можно было
// покрыть юнит-тестами. Воркер связывает движок с вводом/выводом бота.
package survey

import "strings"

// Kind — тип вопроса.
type Kind int

const (
	Single Kind = iota // один вариант (инлайн-кнопки)
	Multi              // несколько вариантов (тогглы + «Готово»)
	Text               // свободный ввод
)

// Step — шаг анкеты.
type Step struct {
	Key     string   // уникальный ключ (он же ключ в answers)
	Label   string   // короткая метка для админки
	Text    string   // полный текст вопроса для бота
	Kind    Kind     // тип ответа
	Options []string // варианты (для Single/Multi)
}

// Ветки по беговому опыту (вопрос experience).
const (
	BranchNovice  = "novice"  // «Я вообще не бегал(а)»
	BranchCasual  = "casual"  // «пару раз нерегулярно» / «иногда для себя»
	BranchRegular = "regular" // «регулярно» / «давно и участвую в забегах»
)

// Порядок шагов: общий префикс → шаги ветки → общий хвост.
var (
	prefix = []string{"name", "experience"}
	suffix = []string{"format", "time", "important", "notes"}

	branchSteps = map[string][]string{
		BranchNovice:  {"motivation", "health", "comfort_start", "freq"},
		BranchCasual:  {"last_run", "distance", "pace", "goal"},
		BranchRegular: {"freq_reg", "volume", "distances", "goals", "races"},
	}
)

// FirstStep — ключ самого первого шага анкеты.
func FirstStep() string { return prefix[0] }

// BranchFor возвращает ветку по индексу ответа на вопрос об опыте (0-based).
func BranchFor(experienceIdx int) string {
	switch experienceIdx {
	case 0:
		return BranchNovice
	case 1, 2:
		return BranchCasual
	case 3, 4:
		return BranchRegular
	default:
		return BranchCasual
	}
}

// Sequence — полный упорядоченный список ключей шагов для ветки.
// Пустая ветка ("") даёт префикс+хвост (до ответа на вопрос об опыте).
func Sequence(branch string) []string {
	seq := append([]string{}, prefix...)
	if bs, ok := branchSteps[branch]; ok {
		seq = append(seq, bs...)
	}
	seq = append(seq, suffix...)
	return seq
}

// Next возвращает ключ следующего шага после cur в рамках ветки.
// done=true означает, что cur — последний шаг (анкета завершена).
func Next(branch, cur string) (next string, done bool) {
	seq := Sequence(branch)
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

// Get возвращает шаг по ключу.
func Get(key string) (Step, bool) {
	s, ok := registry[key]
	return s, ok
}

// Greeting — приветственное сообщение перед первым вопросом.
func Greeting() string {
	return "Привет! 👋\n" +
		"Рады видеть тебя в нашем беговом клубе.\n" +
		"Сейчас зададим несколько коротких вопросов, чтобы понять твой беговой опыт, цели и подобрать комфортный формат участия.\n" +
		"Обещаем: это не экзамен, нормативы сдавать не нужно — только познакомимся 🙂"
}

// Final — финальное сообщение после прохождения анкеты.
func Final() string {
	return "Спасибо! 🏃‍♂️🏃‍♀️\n" +
		"Мы сохранили твои ответы.\n\n" +
		"Теперь мы понимаем твой уровень и сможем предложить подходящий формат: от мягкого старта до тренировок под конкретную дистанцию.\n\n" +
		"Добро пожаловать в клуб — побежали знакомиться с бегом без героизма, но с прогрессом 🙂"
}

// QA — пара «вопрос → ответ» для отображения в админке.
type QA struct {
	Question string
	Answer   string
}

// RenderAnswers разворачивает сохранённые ответы в упорядоченный список QA
// (в порядке шагов ветки). answers — это map из JSONB: значения либо string,
// либо []any строк (мультивыбор).
func RenderAnswers(branch string, answers map[string]any) []QA {
	out := make([]QA, 0, len(answers))
	for _, key := range Sequence(branch) {
		st, ok := registry[key]
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

// steps — определения всех шагов анкеты (тексты строго по ТЗ).
var steps = []Step{
	{
		Key: "name", Label: "Имя", Kind: Text,
		Text: "Как тебя зовут?",
	},
	{
		Key: "experience", Label: "Беговой опыт", Kind: Single,
		Text: "Расскажи, пожалуйста, какой вариант ближе всего к тебе?",
		Options: []string{
			"Я вообще не бегал / не бегала",
			"Бегал(а) пару раз, но нерегулярно",
			"Бегаю иногда для себя",
			"Бегаю регулярно",
			"Бегаю давно и участвую в забегах",
		},
	},

	// --- Ветка novice ---
	{
		Key: "motivation", Label: "Почему интересен бег", Kind: Single,
		Text: "Почему тебе стало интересно попробовать бег?",
		Options: []string{
			"Хочу начать двигаться",
			"Хочу похудеть / улучшить форму",
			"Хочу больше энергии",
			"Хочу найти комьюнити",
			"Хочу подготовиться к первому забегу",
			"Другое",
		},
	},
	{
		Key: "health", Label: "Ограничения по здоровью", Kind: Single,
		Text: "Есть ли травмы, боли, хронические состояния или рекомендации врача, которые нам важно учитывать?",
		Options: []string{
			"Нет, всё ок",
			"Есть небольшие ограничения",
			"Есть серьёзные ограничения",
			"Не уверен(а), хочу начать аккуратно",
		},
	},
	{
		Key: "comfort_start", Label: "Комфортный старт", Kind: Single,
		Text: "Какой старт кажется тебе комфортным?",
		Options: []string{
			"Ходьба + лёгкий бег",
			"Очень лёгкие пробежки",
			"Групповая тренировка для новичков",
			"Пока хочу просто прийти и посмотреть",
		},
	},
	{
		Key: "freq", Label: "Готовность (раз/нед)", Kind: Single,
		Text: "Сколько раз в неделю ты готов(а) начинать?",
		Options: []string{
			"1 раз в неделю",
			"2 раза в неделю",
			"3 раза в неделю",
			"Пока не знаю",
		},
	},

	// --- Ветка casual ---
	{
		Key: "last_run", Label: "Последний раз бегал(а)", Kind: Single,
		Text: "Когда ты бегал(а) в последний раз?",
		Options: []string{
			"На этой неделе",
			"В этом месяце",
			"Несколько месяцев назад",
			"Больше полугода назад",
		},
	},
	{
		Key: "distance", Label: "Комфортная дистанция", Kind: Single,
		Text: "Какая дистанция сейчас для тебя комфортна?",
		Options: []string{
			"До 1 км",
			"1–3 км",
			"3–5 км",
			"5–10 км",
			"Пока не знаю",
		},
	},
	{
		Key: "pace", Label: "Темп", Kind: Single,
		Text: "Какой темп тебе ближе?",
		Options: []string{
			"Очень спокойно, главное — красиво",
			"Лёгкий разговорный бег",
			"Средний темп",
			"Хочу научиться бегать быстрее",
			"Темп не знаю",
		},
	},
	{
		Key: "goal", Label: "Главная цель", Kind: Single,
		Text: "Какая цель сейчас главная?",
		Options: []string{
			"Войти в режим",
			"Улучшить форму",
			"Научиться бегать без одышки",
			"Подготовиться к 5 км",
			"Подготовиться к 10 км",
			"Подготовиться к полумарафону/марафону",
			"Просто бегать в приятной компании",
		},
	},

	// --- Ветка regular ---
	{
		Key: "freq_reg", Label: "Как часто бегает", Kind: Single,
		Text: "Как часто ты сейчас бегаешь?",
		Options: []string{
			"1 раз в неделю",
			"2–3 раза в неделю",
			"4–5 раз в неделю",
			"Почти каждый день",
			"Сейчас перерыв, но хочу вернуться",
		},
	},
	{
		Key: "volume", Label: "Недельный объём", Kind: Single,
		Text: "Какой у тебя примерный недельный объём?",
		Options: []string{
			"До 10 км",
			"10–20 км",
			"20–40 км",
			"40+ км",
			"Не считаю",
		},
	},
	{
		Key: "distances", Label: "Интересные дистанции", Kind: Multi,
		Text: "Какие дистанции тебе сейчас интересны? (можно выбрать несколько)",
		Options: []string{
			"5 км",
			"10 км",
			"21,1 км",
			"42,2 км",
			"Трейл",
			"Просто регулярные тренировки без гонки за медалями",
		},
	},
	{
		Key: "goals", Label: "Цели на ближайшие месяцы", Kind: Multi,
		Text: "Есть ли у тебя беговая цель на ближайшие месяцы? (можно выбрать несколько)",
		Options: []string{
			"Подготовиться к забегу",
			"Улучшить темп",
			"Увеличить дистанцию",
			"Вернуться после перерыва",
			"Бегать стабильно и без травм",
			"Пока без конкретной цели",
		},
	},
	{
		Key: "races", Label: "Официальные забеги", Kind: Single,
		Text: "Участвовал(а) ли ты в официальных забегах?",
		Options: []string{
			"Нет",
			"Да, 5 км",
			"Да, 10 км",
			"Да, полумарафон",
			"Да, марафон",
			"Да, трейлы / другие старты",
		},
	},

	// --- Общий хвост ---
	{
		Key: "format", Label: "Формат тренировок", Kind: Single,
		Text: "В каком формате тебе комфортнее тренироваться?",
		Options: []string{
			"В группе",
			"Индивидуально",
			"Онлайн",
			"Группа + самостоятельные тренировки",
			"Пока хочу попробовать",
		},
	},
	{
		Key: "time", Label: "Удобное время", Kind: Single,
		Text: "В какое время тебе удобнее бегать?",
		Options: []string{
			"Утром",
			"Днём",
			"Вечером",
			"В будни",
			"В выходные",
			"Гибко, зависит от расписания",
		},
	},
	{
		Key: "important", Label: "Что важнее в клубе", Kind: Multi,
		Text: "Что для тебя важнее в клубе? (можно выбрать несколько)",
		Options: []string{
			"Атмосфера и люди",
			"Регулярность и дисциплина",
			"Прогресс в беге",
			"Подготовка к стартам",
			"Поддержка тренера",
			"Просто не бросить через неделю 🙂",
		},
	},
	{
		Key: "notes", Label: "Доп. инфо перед тренировкой", Kind: Text,
		Text: "Есть ли что-то, что нам важно знать о тебе перед первой тренировкой?\n" +
			"Например: травмы, страхи, прошлый опыт, ограничения, цель, пожелания по нагрузке.",
	},
}

// registry — быстрый доступ к шагам по ключу.
var registry = func() map[string]Step {
	m := make(map[string]Step, len(steps))
	for _, s := range steps {
		m[s.Key] = s
	}
	return m
}()
