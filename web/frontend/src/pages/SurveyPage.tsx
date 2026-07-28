import { useEffect, useMemo, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { api } from '../api/client';
import type { SurveyAnswers, SurveyQuestion, SurveyResponse } from '../api/types';
import { useAuth } from '../context/AuthContext';
import { useUI } from '../context/UIContext';
import { useAsync } from '../lib/useAsync';
import { LoadingGrid, ErrorState } from '../components/LoadState';

const byPosition = (a: SurveyQuestion, b: SurveyQuestion) => a.position - b.position;

function branchFromAnswers(questions: SurveyQuestion[], answers: SurveyAnswers): string {
  const selector = questions.find((q) => q.is_selector);
  if (!selector) return '';
  const chosen = answers[selector.key];
  const opt = selector.options.find((o) => o.label === chosen);
  return opt?.branch ?? '';
}

function buildSequence(questions: SurveyQuestion[], answers: SurveyAnswers): SurveyQuestion[] {
  const intro = questions.filter((q) => q.phase === 'intro').sort(byPosition);
  const outro = questions.filter((q) => q.phase === 'outro').sort(byPosition);
  const branch = branchFromAnswers(questions, answers);
  const branchQs = branch
    ? questions.filter((q) => q.phase === 'branch' && q.branch === branch).sort(byPosition)
    : [];
  return [...intro, ...branchQs, ...outro];
}

function isAnswered(q: SurveyQuestion, value: string | string[] | undefined): boolean {
  if (q.kind === 'multi') return Array.isArray(value) && value.length > 0;
  return typeof value === 'string' && value.trim() !== '';
}

function firstUnansweredIndex(questions: SurveyQuestion[], answers: SurveyAnswers): number {
  const seq = buildSequence(questions, answers);
  for (let i = 0; i < seq.length; i++) {
    if (!isAnswered(seq[i], answers[seq[i].key])) return i;
  }
  return seq.length > 0 ? seq.length - 1 : 0;
}

function formatValue(value: string | string[] | undefined): string {
  if (Array.isArray(value)) return value.join(', ');
  return value ?? '—';
}

type Phase = 'welcome' | 'questions' | 'done';

export function SurveyPage() {
  const { user, loading: authLoading } = useAuth();
  const navigate = useNavigate();
  const state = useAsync<SurveyResponse>((signal) => api.survey(signal), []);

  useEffect(() => {
    if (!authLoading && !user) navigate('/', { replace: true });
  }, [authLoading, user, navigate]);

  if (authLoading || !user) return null;
  if (state.loading) {
    return (
      <SurveyShell>
        <LoadingGrid count={1} />
      </SurveyShell>
    );
  }
  if (state.error || !state.data) {
    return (
      <SurveyShell>
        <ErrorState message="Не удалось загрузить анкету" />
      </SurveyShell>
    );
  }

  return <SurveyFlow data={state.data} />;
}

function SurveyShell({ children }: { children: React.ReactNode }) {
  return (
    <>
      <section className="page-hero">
        <div className="hero-sl speedlines" />
        <div className="wrap">
          <div className="eb">Анкета</div>
          <h1 className="d2">Анкета бегуна</h1>
        </div>
      </section>
      <section className="sec">
        <div className="wrap">{children}</div>
      </section>
    </>
  );
}

function SurveyFlow({ data }: { data: SurveyResponse }) {
  const { showToast } = useUI();
  const { refresh: refreshAuth } = useAuth();
  const initialCompleted = data.status === 'completed';
  const initialAnswers = data.answers ?? {};
  const [answers, setAnswers] = useState<SurveyAnswers>(initialAnswers);
  const [phase, setPhase] = useState<Phase>(
    initialCompleted ? 'done' : data.status === 'in_progress' ? 'questions' : 'welcome',
  );
  const [stepIndex, setStepIndex] = useState(() =>
    data.status === 'in_progress' ? firstUnansweredIndex(data.questions, initialAnswers) : 0,
  );
  const [submitting, setSubmitting] = useState(false);

  const sequence = useMemo(() => buildSequence(data.questions, answers), [data.questions, answers]);
  const current = sequence[stepIndex];
  const total = sequence.length;

  useEffect(() => {
    if (initialCompleted || phase !== 'questions') return;
    if (Object.keys(answers).length === 0) return;
    const t = setTimeout(() => {
      void api.surveyProgress(answers).catch(() => {});
    }, 400);
    return () => clearTimeout(t);
  }, [answers, phase, initialCompleted]);

  function setAnswer(key: string, value: string | string[]) {
    setAnswers((prev) => ({ ...prev, [key]: value }));
  }

  function goNext() {
    if (stepIndex + 1 < sequence.length) {
      setStepIndex((i) => i + 1);
    } else {
      void submit();
    }
  }

  function goBack() {
    if (stepIndex > 0) setStepIndex((i) => i - 1);
    else setPhase('welcome');
  }

  async function submit() {
    setSubmitting(true);
    try {
      await api.surveySubmit(answers);
      setPhase('done');
      void refreshAuth();
      showToast('Спасибо! Анкета сохранена');
    } catch {
      showToast('Не удалось сохранить анкету');
    } finally {
      setSubmitting(false);
    }
  }

  if (phase === 'welcome') {
    return (
      <SurveyShell>
        <div className="survey-card survey-intro">
          {data.greeting.split('\n').map((line, i) => (
            <p key={i}>{line}</p>
          ))}
          <button
            className="btn btn-primary"
            style={{ marginTop: 24 }}
            onClick={() => {
              setStepIndex(0);
              setPhase('questions');
            }}
          >
            {data.status === 'completed' ? 'Пройти заново' : 'Начать'}
          </button>
        </div>
      </SurveyShell>
    );
  }

  if (phase === 'done') {
    const branch = branchFromAnswers(data.questions, answers);
    const summarySeq = buildSequence(data.questions, answers);
    return (
      <SurveyShell>
        <div className="survey-card survey-done">
          {data.final.split('\n').map((line, i) => (
            <p key={i}>{line}</p>
          ))}
        </div>
        <div className="survey-summary">
          <div className="sec-head" style={{ marginTop: 8 }}>
            <div className="eb">Ваши ответы</div>
          </div>
          <dl className="survey-answers">
            {summarySeq.map((q) => (
              <div className="survey-answer-row" key={q.key}>
                <dt>{q.label}</dt>
                <dd>{formatValue(answers[q.key])}</dd>
              </div>
            ))}
          </dl>
          <div style={{ display: 'flex', gap: 12, marginTop: 24, flexWrap: 'wrap' }}>
            <Link className="btn btn-primary" to="/runners">
              Купить тренировку
            </Link>
            <button
              className="btn btn-ghost"
              onClick={() => {
                setStepIndex(0);
                setPhase('questions');
              }}
            >
              Изменить ответы
            </button>
          </div>
          {branch && <input type="hidden" value={branch} readOnly />}
        </div>
      </SurveyShell>
    );
  }

  if (!current) {
    return (
      <SurveyShell>
        <ErrorState message="В анкете пока нет вопросов" />
      </SurveyShell>
    );
  }

  const answered = isAnswered(current, answers[current.key]);
  const progress = total > 0 ? Math.round(((stepIndex + 1) / total) * 100) : 0;

  return (
    <SurveyShell>
      <div className="survey-progress">
        <div className="survey-progress-bar" style={{ width: `${progress}%` }} />
      </div>
      <div className="survey-step-count">
        Вопрос {stepIndex + 1} из {total}
      </div>

      <div className="survey-card">
        <h2 className="survey-prompt">{current.prompt}</h2>

        <QuestionInput
          question={current}
          value={answers[current.key]}
          onChange={(v) => setAnswer(current.key, v)}
          onAutoAdvance={goNext}
        />

        <div className="survey-nav">
          <button className="btn btn-ghost" onClick={goBack} disabled={submitting}>
            Назад
          </button>
          {current.kind !== 'single' && (
            <button className="btn btn-primary" onClick={goNext} disabled={!answered || submitting}>
              {stepIndex + 1 === total ? (submitting ? 'Сохраняем…' : 'Завершить') : 'Далее'}
            </button>
          )}
        </div>
      </div>
    </SurveyShell>
  );
}

interface QuestionInputProps {
  question: SurveyQuestion;
  value: string | string[] | undefined;
  onChange: (value: string | string[]) => void;
  onAutoAdvance: () => void;
}

function QuestionInput({ question, value, onChange, onAutoAdvance }: QuestionInputProps) {
  if (question.kind === 'text') {
    return (
      <textarea
        className="survey-textarea"
        value={typeof value === 'string' ? value : ''}
        placeholder="Ваш ответ…"
        rows={4}
        onChange={(e) => onChange(e.target.value)}
        autoFocus
      />
    );
  }

  if (question.kind === 'multi') {
    const selected = Array.isArray(value) ? value : [];
    const toggle = (label: string) => {
      onChange(
        selected.includes(label) ? selected.filter((x) => x !== label) : [...selected, label],
      );
    };
    return (
      <div className="survey-options">
        {question.options.map((opt) => {
          const on = selected.includes(opt.label);
          return (
            <button
              key={opt.label}
              type="button"
              className={`survey-option${on ? ' is-selected' : ''}`}
              onClick={() => toggle(opt.label)}
            >
              <span className="survey-option-check">{on ? '✓' : ''}</span>
              <span>{opt.label}</span>
            </button>
          );
        })}
      </div>
    );
  }

  return (
    <div className="survey-options">
      {question.options.map((opt) => {
        const on = value === opt.label;
        return (
          <button
            key={opt.label}
            type="button"
            className={`survey-option${on ? ' is-selected' : ''}`}
            onClick={() => {
              onChange(opt.label);
              setTimeout(onAutoAdvance, 180);
            }}
          >
            <span className="survey-option-radio">{on ? '●' : ''}</span>
            <span>{opt.label}</span>
          </button>
        );
      })}
    </div>
  );
}
