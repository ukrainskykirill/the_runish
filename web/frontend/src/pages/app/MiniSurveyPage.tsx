import { useEffect, useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import { api } from '../../api/client';
import type { SurveyAnswers, SurveyQuestion, SurveyResponse } from '../../api/types';
import { CheckIcon, ChevronRightIcon, FlagIcon } from '../../components/icons';
import { ErrorState } from '../../components/LoadState';
import { useAuth } from '../../context/AuthContext';
import { useUI } from '../../context/UIContext';
import { useAsync } from '../../lib/useAsync';

const byPosition = (a: SurveyQuestion, b: SurveyQuestion) => a.position - b.position;

function branchFromAnswers(questions: SurveyQuestion[], answers: SurveyAnswers): string {
  const selector = questions.find((question) => question.is_selector);
  const value = selector ? answers[selector.key] : undefined;
  if (!selector || typeof value !== 'string') return '';
  return selector.options.find((option) => option.label === value)?.branch ?? '';
}

function buildSequence(questions: SurveyQuestion[], answers: SurveyAnswers): SurveyQuestion[] {
  const branch = branchFromAnswers(questions, answers);
  return [
    ...questions.filter((question) => question.phase === 'intro').sort(byPosition),
    ...questions
      .filter((question) => question.phase === 'branch' && question.branch === branch)
      .sort(byPosition),
    ...questions.filter((question) => question.phase === 'outro').sort(byPosition),
  ];
}

function isAnswered(question: SurveyQuestion, value: string | string[] | undefined): boolean {
  return question.kind === 'multi'
    ? Array.isArray(value) && value.length > 0
    : typeof value === 'string' && value.trim().length > 0;
}

function firstUnanswered(questions: SurveyQuestion[], answers: SurveyAnswers): number {
  const sequence = buildSequence(questions, answers);
  const index = sequence.findIndex((question) => !isAnswered(question, answers[question.key]));
  return index < 0 ? Math.max(0, sequence.length - 1) : index;
}

function answerText(value: string | string[] | undefined): string {
  return Array.isArray(value) ? value.join(', ') : value || '—';
}

type SurveyPhase = 'welcome' | 'questions' | 'done';

export function MiniSurveyPage() {
  const state = useAsync<SurveyResponse>((signal) => api.survey(signal), []);

  if (state.loading) {
    return (
      <div className="pad mini-survey-page">
        <div className="mini-survey-skeleton" />
      </div>
    );
  }

  if (state.error || !state.data) {
    return (
      <div className="pad mini-survey-page">
        <ErrorState message="Не удалось загрузить анкету" />
      </div>
    );
  }

  return <MiniSurveyFlow data={state.data} />;
}

function MiniSurveyFlow({ data }: { data: SurveyResponse }) {
  const { refresh } = useAuth();
  const { showToast } = useUI();
  const initialAnswers = data.answers ?? {};
  const [answers, setAnswers] = useState<SurveyAnswers>(initialAnswers);
  const [phase, setPhase] = useState<SurveyPhase>(
    data.status === 'completed' ? 'done' : data.status === 'in_progress' ? 'questions' : 'welcome',
  );
  const [step, setStep] = useState(() =>
    data.status === 'in_progress' ? firstUnanswered(data.questions, initialAnswers) : 0,
  );
  const [submitting, setSubmitting] = useState(false);
  const sequence = useMemo(() => buildSequence(data.questions, answers), [data.questions, answers]);
  const current = sequence[step];

  useEffect(() => {
    if (phase !== 'questions' || Object.keys(answers).length === 0) return;
    const timer = window.setTimeout(() => void api.surveyProgress(answers).catch(() => {}), 500);
    return () => window.clearTimeout(timer);
  }, [answers, phase]);

  async function submit(nextAnswers = answers) {
    setSubmitting(true);
    try {
      await api.surveySubmit(nextAnswers);
      setAnswers(nextAnswers);
      setPhase('done');
      await refresh();
      showToast('Анкета сохранена');
    } catch {
      showToast('Не удалось сохранить анкету');
    } finally {
      setSubmitting(false);
    }
  }

  function goNext(nextAnswers = answers) {
    const nextSequence = buildSequence(data.questions, nextAnswers);
    if (step + 1 < nextSequence.length) setStep((value) => value + 1);
    else void submit(nextAnswers);
  }

  function selectSingle(question: SurveyQuestion, value: string) {
    const nextAnswers = { ...answers, [question.key]: value };
    setAnswers(nextAnswers);
    window.setTimeout(() => goNext(nextAnswers), 180);
  }

  if (phase === 'welcome') {
    return (
      <div className="pad mini-survey-page">
        <section className="mini-survey-cover">
          <span className="mini-survey-lines speedlines" />
          <span className="mini-survey-mark"><FlagIcon className="i" /></span>
          <div className="mini-survey-eyebrow">Анкета бегуна</div>
          <h1>Бежим<br />вместе</h1>
          <div className="mini-survey-greeting">
            {data.greeting.split('\n').filter(Boolean).map((line) => <p key={line}>{line}</p>)}
          </div>
          <button className="mini-survey-primary" onClick={() => { setStep(0); setPhase('questions'); }}>
            Начать <ChevronRightIcon className="i i-sm" />
          </button>
        </section>
        <p className="mini-survey-note">Ответы видит только команда клуба — они помогут подобрать комфортный формат тренировок.</p>
      </div>
    );
  }

  if (phase === 'done') {
    return (
      <div className="pad mini-survey-page">
        <section className="mini-survey-finish">
          <span className="mini-survey-check"><CheckIcon className="i" /></span>
          <div className="mini-survey-eyebrow">Готово</div>
          <h1>Спасибо!</h1>
          {data.final.split('\n').filter(Boolean).map((line) => <p key={line}>{line}</p>)}
        </section>
        <section className="mini-survey-summary">
          <div className="mini-survey-summary-head">
            <h2>Ваши ответы</h2>
            <button onClick={() => { setStep(0); setPhase('questions'); }}>Изменить</button>
          </div>
          <dl>
            {buildSequence(data.questions, answers).map((question) => (
              <div key={question.key}>
                <dt>{question.label}</dt>
                <dd>{answerText(answers[question.key])}</dd>
              </div>
            ))}
          </dl>
        </section>
        <Link className="mini-survey-back" to="/app/profile">Вернуться в профиль</Link>
      </div>
    );
  }

  if (!current) {
    return <div className="pad"><ErrorState message="В анкете пока нет вопросов" /></div>;
  }

  const progress = sequence.length ? Math.round(((step + 1) / sequence.length) * 100) : 0;
  const value = answers[current.key];

  return (
    <div className="pad mini-survey-page">
      <div className="mini-survey-progress-copy">
        <span>Вопрос {step + 1}</span>
        <b>{progress}%</b>
      </div>
      <div className="mini-survey-progress"><i style={{ width: `${progress}%` }} /></div>
      <section className="mini-survey-question">
        <div className="mini-survey-eyebrow">{current.label}</div>
        <h1>{current.prompt}</h1>
        {current.kind === 'text' ? (
          <textarea
            value={typeof value === 'string' ? value : ''}
            rows={5}
            placeholder="Ваш ответ…"
            onChange={(event) => setAnswers((prev) => ({ ...prev, [current.key]: event.target.value }))}
            autoFocus
          />
        ) : (
          <div className="mini-survey-options">
            {current.options.map((option, index) => {
              const selected = current.kind === 'multi'
                ? Array.isArray(value) && value.includes(option.label)
                : value === option.label;
              return (
                <button
                  key={option.label}
                  className={selected ? 'selected' : ''}
                  onClick={() => {
                    if (current.kind === 'single') return selectSingle(current, option.label);
                    const selectedValues = Array.isArray(value) ? value : [];
                    setAnswers((prev) => ({
                      ...prev,
                      [current.key]: selected
                        ? selectedValues.filter((item) => item !== option.label)
                        : [...selectedValues, option.label],
                    }));
                  }}
                >
                  <span className="mini-survey-option-index">{String(index + 1).padStart(2, '0')}</span>
                  <span>{option.label}</span>
                  <i>{selected ? '✓' : ''}</i>
                </button>
              );
            })}
          </div>
        )}
      </section>
      <div className="mini-survey-nav">
        <button className="back" onClick={() => step > 0 ? setStep((value) => value - 1) : setPhase('welcome')}>
          Назад
        </button>
        {current.kind !== 'single' ? (
          <button className="next" disabled={!isAnswered(current, value) || submitting} onClick={() => goNext()}>
            {step + 1 === sequence.length ? (submitting ? 'Сохраняем…' : 'Завершить') : 'Далее'}
          </button>
        ) : null}
      </div>
    </div>
  );
}
