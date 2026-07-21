import { useState } from 'react';
import { Link } from 'react-router-dom';
import { api } from '../../api/client';
import type { Training } from '../../api/types';
import { ErrorState } from '../LoadState';
import { useAsync } from '../../lib/useAsync';

const DOW = ['Пн', 'Вт', 'Ср', 'Чт', 'Пт', 'Сб', 'Вс'];
const MONTHS_GEN = [
  'января', 'февраля', 'марта', 'апреля', 'мая', 'июня',
  'июля', 'августа', 'сентября', 'октября', 'ноября', 'декабря',
];
const MONTHS_NOM = [
  'Январь', 'Февраль', 'Март', 'Апрель', 'Май', 'Июнь',
  'Июль', 'Август', 'Сентябрь', 'Октябрь', 'Ноябрь', 'Декабрь',
];

function isoWeekday(d: Date): number {
  return ((d.getDay() + 6) % 7) + 1;
}

function pad2(n: number): string {
  return n < 10 ? `0${n}` : `${n}`;
}

function sameDay(a: Date, b: Date): boolean {
  return (
    a.getFullYear() === b.getFullYear() &&
    a.getMonth() === b.getMonth() &&
    a.getDate() === b.getDate()
  );
}

function ClockIcon() {
  return (
    <svg className="i i-xs" viewBox="0 0 24 24" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round">
      <circle cx="12" cy="12" r="9" />
      <path d="M12 8v4l3 2" />
    </svg>
  );
}

function PinIcon() {
  return (
    <svg className="i i-xs" viewBox="0 0 24 24" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round">
      <path d="M12 21s-7-5.7-7-11a7 7 0 0 1 14 0c0 5.3-7 11-7 11z" />
      <circle cx="12" cy="10" r="2.2" />
    </svg>
  );
}

function ChevronIcon() {
  return (
    <svg className="i i-xs" viewBox="0 0 24 24" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round">
      <path d="M6 9l6 6 6-6" />
    </svg>
  );
}

interface EventProps {
  t: Training;
  variant: 'week' | 'month';
  expanded: boolean;
  onToggle: () => void;
}

// Одно событие в расписании. Если у тренировки есть описание — клик раскрывает его,
// иначе ведёт в каталог (как раньше).
function ScheduleEvent({ t, variant, expanded, onToggle }: EventProps) {
  const cls = variant === 'week' ? 'cal-ev' : 'cal-mev';
  const tCls = variant === 'week' ? 'ev-t' : 'mev-t';
  const timeCls = variant === 'week' ? 'ev-time' : 'mev-time';
  const placeCls = variant === 'week' ? 'ev-place' : 'mev-place';
  const hasDesc = !!t.description;

  const placeNode = t.place_url ? (
    <span
      className="ev-place-link"
      onClick={(e) => {
        e.preventDefault();
        e.stopPropagation();
        window.open(t.place_url!, '_blank', 'noopener,noreferrer');
      }}
    >
      {t.place}
    </span>
  ) : (
    t.place
  );

  const inner = (
    <>
      <div className={tCls}>
        {t.title}
        {hasDesc && (
          <span className={'cal-ev-caret' + (expanded ? ' open' : '')}>
            <ChevronIcon />
          </span>
        )}
      </div>
      <div className={timeCls}>
        <ClockIcon />
        {t.start_time}
      </div>
      <div className={placeCls}>
        <PinIcon />
        {placeNode}
      </div>
      {hasDesc && expanded && <div className="cal-ev-desc">{t.description}</div>}
    </>
  );

  if (hasDesc) {
    return (
      <div
        className={cls + ' has-desc' + (expanded ? ' open' : '')}
        role="button"
        tabIndex={0}
        aria-expanded={expanded}
        onClick={onToggle}
        onKeyDown={(e) => {
          if (e.key === 'Enter' || e.key === ' ') {
            e.preventDefault();
            onToggle();
          }
        }}
      >
        {inner}
      </div>
    );
  }
  return (
    <Link className={cls} to="/runners">
      {inner}
    </Link>
  );
}

type CalView = 'week' | 'month';

interface ScheduleBoardProps {
  trainings?: Training[];
  enableViews?: boolean;
}

export function ScheduleBoard({ trainings: providedTrainings, enableViews = false }: ScheduleBoardProps) {
  const [view, setView] = useState<CalView>('week');
  const [monthOffset, setMonthOffset] = useState(0);
  const [expandedId, setExpandedId] = useState<number | null>(null);
  const shouldFetch = providedTrainings === undefined;
  const { data, loading, error } = useAsync(
    () => (shouldFetch ? api.schedule() : Promise.resolve({ trainings: providedTrainings })),
    [shouldFetch, providedTrainings],
  );
  const trainings = data?.trainings ?? providedTrainings ?? [];

  const toggle = (id: number) => setExpandedId((cur) => (cur === id ? null : id));

  const today = new Date();
  const todayIso = isoWeekday(today);
  const monday = new Date(today);
  monday.setHours(0, 0, 0, 0);
  monday.setDate(today.getDate() - (todayIso - 1));

  const byDay = new Map<number, Training[]>();
  for (const t of trainings) {
    const arr = byDay.get(t.weekday);
    if (arr) arr.push(t);
    else byDay.set(t.weekday, [t]);
  }

  const days = Array.from({ length: 7 }, (_, i) => {
    const wd = i + 1;
    const date = new Date(monday);
    date.setDate(monday.getDate() + i);
    return {
      wd,
      date,
      isToday: wd === todayIso,
      isWeekend: wd >= 6,
      items: byDay.get(wd) ?? [],
    };
  });

  const sunday = days[6].date;
  const weekRange =
    monday.getMonth() === sunday.getMonth()
      ? `${monday.getDate()} — ${sunday.getDate()} ${MONTHS_GEN[sunday.getMonth()]}`
      : `${monday.getDate()} ${MONTHS_GEN[monday.getMonth()]} — ${sunday.getDate()} ${MONTHS_GEN[sunday.getMonth()]}`;

  // Отображаемый месяц (0 = текущий), с проекцией недельного шаблона на все недели.
  const monthFirst = new Date(today.getFullYear(), today.getMonth() + monthOffset, 1);
  const mYear = monthFirst.getFullYear();
  const mMonth = monthFirst.getMonth();
  const gridStart = new Date(monthFirst);
  gridStart.setDate(monthFirst.getDate() - (isoWeekday(monthFirst) - 1));
  const daysInMonth = new Date(mYear, mMonth + 1, 0).getDate();
  const cellsCount = Math.ceil((isoWeekday(monthFirst) - 1 + daysInMonth) / 7) * 7;
  const monthCells = Array.from({ length: cellsCount }, (_, i) => {
    const date = new Date(gridStart);
    date.setDate(gridStart.getDate() + i);
    const wd = isoWeekday(date);
    const inMonth = date.getMonth() === mMonth;
    return {
      date,
      wd,
      inMonth,
      isToday: sameDay(date, today),
      items: inMonth ? byDay.get(wd) ?? [] : [],
    };
  });

  const minMonthOffset = -today.getMonth();
  const maxMonthOffset = 11 - today.getMonth();

  const headTitle = view === 'month' ? `${MONTHS_NOM[mMonth]} ${mYear}` : 'Эта неделя';
  const headRange = view === 'month' ? '' : weekRange;

  return (
    <div className="cal-board">
      <div className="cal-strip speedlines" />
      <div className="cal-board-head">
        <div className="wk">
          {view === 'month' && enableViews ? (
            <div className="cal-monthnav">
              <button
                type="button"
                className="cal-navbtn"
                aria-label="Предыдущий месяц"
                disabled={monthOffset <= minMonthOffset}
                onClick={() => setMonthOffset((m) => Math.max(m - 1, minMonthOffset))}
              >
                ‹
              </button>
              <span className="t">{headTitle}</span>
              <button
                type="button"
                className="cal-navbtn"
                aria-label="Следующий месяц"
                disabled={monthOffset >= maxMonthOffset}
                onClick={() => setMonthOffset((m) => Math.min(m + 1, maxMonthOffset))}
              >
                ›
              </button>
            </div>
          ) : (
            <>
              <span className="t">{headTitle}</span>
              {headRange ? <span className="r">{headRange}</span> : null}
            </>
          )}
        </div>
        {enableViews ? (
          <div className="cal-views" role="tablist" aria-label="Вид календаря">
            <button
              type="button"
              className={'cal-view-btn' + (view === 'week' ? ' active' : '')}
              onClick={() => setView('week')}
            >
              Неделя
            </button>
            <button
              type="button"
              className={'cal-view-btn' + (view === 'month' ? ' active' : '')}
              onClick={() => setView('month')}
            >
              Месяц
            </button>
          </div>
        ) : (
          <div className="cal-legend">
            <span className="lg">
              <span className="sw red" />
              Тренировка
            </span>
          </div>
        )}
      </div>

      {shouldFetch && loading ? (
        <div className="cal-loading">
          {Array.from({ length: 7 }, (_, i) => (
            <span className="sk" key={i} />
          ))}
        </div>
      ) : shouldFetch && error ? (
        <ErrorState message="Не удалось загрузить расписание" />
      ) : view === 'month' ? (
        <div className="cal-month">
          {DOW.map((d, i) => (
            <div key={`h${i}`} className={'cal-mhead' + (i >= 5 ? ' weekend' : '')}>
              {d}
            </div>
          ))}
          {monthCells.map((c, i) => (
            <div
              key={i}
              className={
                'cal-mday' +
                (c.inMonth ? '' : ' out') +
                (c.isToday ? ' today' : '') +
                (c.items.length ? ' has' : '')
              }
            >
              <div className="cal-mnum">{c.date.getDate()}</div>
              <div className="cal-mevents">
                {c.items.map((t) => (
                  <ScheduleEvent
                    key={t.id}
                    t={t}
                    variant="month"
                    expanded={expandedId === t.id}
                    onToggle={() => toggle(t.id)}
                  />
                ))}
              </div>
            </div>
          ))}
        </div>
      ) : (
        <div className="cal">
        {days.map((d) => (
          <div
            key={d.wd}
            className={
              'cal-day' +
              (d.isToday ? ' today' : '') +
              (d.isWeekend ? ' weekend' : '') +
              (d.items.length ? ' has' : '')
            }
          >
            <div className="cal-colhead">
              <div className="dow">{DOW[d.wd - 1]}</div>
              <div className="dnum">
                {pad2(d.date.getDate())}.{pad2(d.date.getMonth() + 1)}
              </div>
              {d.isToday && <span className="cal-today-tag">Сегодня</span>}
            </div>
            <div className="events">
              {d.items.length === 0 ? (
                <div className="cal-rest">
                  <span className="dash" />
                  <span className="lbl">Отдых</span>
                </div>
              ) : (
                d.items.map((t) => (
                  <ScheduleEvent
                    key={t.id}
                    t={t}
                    variant="week"
                    expanded={expandedId === t.id}
                    onToggle={() => toggle(t.id)}
                  />
                ))
              )}
            </div>
          </div>
        ))}
        </div>
      )}
    </div>
  );
}
