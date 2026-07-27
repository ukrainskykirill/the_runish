import { useState, type MouseEvent } from 'react';
import { Link } from 'react-router-dom';
import { api } from '../../api/client';
import type { TrainingOccurrence } from '../../api/types';
import { useAuth } from '../../context/AuthContext';
import { useAsync } from '../../lib/useAsync';
import { ErrorState } from '../LoadState';
import { occKey, resolveRegMode, useTrainingRegister } from './registerAction';

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

function ymd(d: Date): string {
  return `${d.getFullYear()}-${pad2(d.getMonth() + 1)}-${pad2(d.getDate())}`;
}

function parseYmd(s: string): Date {
  const [y, m, d] = s.split('-').map(Number);
  return new Date(y, m - 1, d);
}

function sameDay(a: Date, b: Date): boolean {
  return a.getFullYear() === b.getFullYear() && a.getMonth() === b.getMonth() && a.getDate() === b.getDate();
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
  occ: TrainingOccurrence;
  variant: 'week' | 'month';
  expanded: boolean;
  onToggle: () => void;
  busy: boolean;
  onRegister: () => void;
  onCancel: () => void;
}

function CalEventFoot({ occ, busy, onRegister, onCancel }: Omit<EventProps, 'variant' | 'expanded' | 'onToggle'>) {
  const { user, canBookFreeLesson } = useAuth();
  const mode = resolveRegMode(occ, user, canBookFreeLesson);
  const stop = (e: MouseEvent) => e.stopPropagation();

  switch (mode) {
    case 'registered':
      return (
        <div className="cal-ev-foot" onClick={stop}>
          <span className="cal-reg-tag">✓ Записан</span>
          <button className="cal-reg-btn ghost" disabled={busy} onClick={(e) => { stop(e); onCancel(); }}>
            Отменить
          </button>
        </div>
      );
    case 'cancelled':
      return <div className="cal-ev-foot"><span className="cal-reg-note">Отменено</span></div>;
    case 'full':
      return <div className="cal-ev-foot"><span className="cal-reg-note">Мест нет</span></div>;
    case 'needsub':
      return (
        <div className="cal-ev-foot" onClick={stop}>
          <Link className="cal-reg-btn ghost" to="/runners">Нужна подписка</Link>
        </div>
      );
    case 'free':
      return (
        <div className="cal-ev-foot" onClick={stop}>
          <button className="cal-reg-btn" disabled={busy} onClick={(e) => { stop(e); onRegister(); }}>
            Первая — бесплатно 🎉
          </button>
        </div>
      );
    default:
      return (
        <div className="cal-ev-foot" onClick={stop}>
          <button className="cal-reg-btn" disabled={busy} onClick={(e) => { stop(e); onRegister(); }}>
            Записаться
          </button>
        </div>
      );
  }
}

function ScheduleEvent({ occ, variant, expanded, onToggle, busy, onRegister, onCancel }: EventProps) {
  const cls = variant === 'week' ? 'cal-ev' : 'cal-mev';
  const tCls = variant === 'week' ? 'ev-t' : 'mev-t';
  const timeCls = variant === 'week' ? 'ev-time' : 'mev-time';
  const placeCls = variant === 'week' ? 'ev-place' : 'mev-place';
  const hasDesc = !!occ.description;

  const placeNode = occ.place_url ? (
    <span
      className="ev-place-link"
      onClick={(e) => {
        e.preventDefault();
        e.stopPropagation();
        window.open(occ.place_url!, '_blank', 'noopener,noreferrer');
      }}
    >
      {occ.place}
    </span>
  ) : (
    occ.place
  );

  return (
    <div
      className={cls + (hasDesc ? ' has-desc' : '') + (expanded ? ' open' : '')}
      role={hasDesc ? 'button' : undefined}
      tabIndex={hasDesc ? 0 : undefined}
      aria-expanded={hasDesc ? expanded : undefined}
      onClick={hasDesc ? onToggle : undefined}
      onKeyDown={
        hasDesc
          ? (e) => {
              if (e.key === 'Enter' || e.key === ' ') {
                e.preventDefault();
                onToggle();
              }
            }
          : undefined
      }
    >
      <div className={tCls}>
        {occ.kind === 'sunday_runish' ? <span className="cal-ev-badge">SR</span> : null}
        {occ.title}
        {hasDesc && (
          <span className={'cal-ev-caret' + (expanded ? ' open' : '')}>
            <ChevronIcon />
          </span>
        )}
      </div>
      <div className={timeCls}>
        <ClockIcon />
        {occ.start_time}
      </div>
      <div className={placeCls}>
        <PinIcon />
        {placeNode}
      </div>
      {hasDesc && expanded && <div className="cal-ev-desc">{occ.description}</div>}
      <CalEventFoot occ={occ} busy={busy} onRegister={onRegister} onCancel={onCancel} />
    </div>
  );
}

export function ScheduleBoard() {
  const { user, refresh } = useAuth();
  const [view, setView] = useState<'week' | 'month'>('week');
  const [weekOffset, setWeekOffset] = useState(0);
  const [monthOffset, setMonthOffset] = useState(0);
  const [expandedKey, setExpandedKey] = useState<string | null>(null);
  const [reloadKey, setReloadKey] = useState(0);

  const { data, loading, error } = useAsync(() => api.trainingsUpcoming(), [reloadKey, user?.id]);
  const occurrences = data?.occurrences ?? [];

  async function reload() {
    setReloadKey((k) => k + 1);
    await refresh();
  }
  const { busyKey, handleRegister, handleCancel } = useTrainingRegister(reload);
  const toggle = (key: string) => setExpandedKey((cur) => (cur === key ? null : key));

  const byDate = new Map<string, TrainingOccurrence[]>();
  for (const o of occurrences) {
    const arr = byDate.get(o.session_date);
    if (arr) arr.push(o);
    else byDate.set(o.session_date, [o]);
  }

  const today = new Date();
  today.setHours(0, 0, 0, 0);
  let lastStr = ymd(today);
  for (const o of occurrences) {
    if (o.session_date > lastStr) lastStr = o.session_date;
  }
  const lastDate = parseYmd(lastStr);

  const curMonday = new Date(today);
  curMonday.setDate(today.getDate() - (isoWeekday(today) - 1));
  const monday = new Date(curMonday);
  monday.setDate(curMonday.getDate() + weekOffset * 7);
  const weekCells = Array.from({ length: 7 }, (_, i) => {
    const date = new Date(monday);
    date.setDate(monday.getDate() + i);
    return {
      date,
      isToday: sameDay(date, today),
      isWeekend: isoWeekday(date) >= 6,
      items: byDate.get(ymd(date)) ?? [],
    };
  });
  const sunday = weekCells[6].date;
  const weekRange =
    monday.getMonth() === sunday.getMonth()
      ? `${monday.getDate()} — ${sunday.getDate()} ${MONTHS_GEN[sunday.getMonth()]}`
      : `${monday.getDate()} ${MONTHS_GEN[monday.getMonth()]} — ${sunday.getDate()} ${MONTHS_GEN[sunday.getMonth()]}`;
  const lastMonday = new Date(lastDate);
  lastMonday.setDate(lastDate.getDate() - (isoWeekday(lastDate) - 1));
  const maxWeekOffset = Math.max(0, Math.round((lastMonday.getTime() - curMonday.getTime()) / (7 * 86400000)));

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
    const inMonth = date.getMonth() === mMonth;
    return {
      date,
      inMonth,
      isToday: sameDay(date, today),
      items: inMonth ? byDate.get(ymd(date)) ?? [] : [],
    };
  });
  const maxMonthOffset = Math.max(
    0,
    (lastDate.getFullYear() - today.getFullYear()) * 12 + (lastDate.getMonth() - today.getMonth()),
  );

  const isWeek = view === 'week';
  const headTitle = isWeek ? (weekOffset === 0 ? 'Эта неделя' : 'Неделя') : `${MONTHS_NOM[mMonth]} ${mYear}`;
  const canPrev = isWeek ? weekOffset > 0 : monthOffset > 0;
  const canNext = isWeek ? weekOffset < maxWeekOffset : monthOffset < maxMonthOffset;
  const goPrev = () => (isWeek ? setWeekOffset((w) => Math.max(0, w - 1)) : setMonthOffset((m) => Math.max(0, m - 1)));
  const goNext = () =>
    isWeek ? setWeekOffset((w) => Math.min(maxWeekOffset, w + 1)) : setMonthOffset((m) => Math.min(maxMonthOffset, m + 1));

  return (
    <div className="cal-board">
      <div className="cal-strip speedlines" />
      <div className="cal-board-head">
        <div className="wk">
          <div className="cal-monthnav">
            <button type="button" className="cal-navbtn" aria-label="Назад" disabled={!canPrev} onClick={goPrev}>
              ‹
            </button>
            <span className="t">{headTitle}</span>
            <button type="button" className="cal-navbtn" aria-label="Вперёд" disabled={!canNext} onClick={goNext}>
              ›
            </button>
          </div>
          {isWeek ? <span className="r">{weekRange}</span> : null}
        </div>
        <div className="cal-views" role="tablist" aria-label="Вид календаря">
          <button type="button" className={'cal-view-btn' + (isWeek ? ' active' : '')} onClick={() => setView('week')}>
            Неделя
          </button>
          <button type="button" className={'cal-view-btn' + (!isWeek ? ' active' : '')} onClick={() => setView('month')}>
            Месяц
          </button>
        </div>
      </div>

      {loading ? (
        <div className="cal-loading">
          {Array.from({ length: 7 }, (_, i) => (
            <span className="sk" key={i} />
          ))}
        </div>
      ) : error ? (
        <ErrorState message="Не удалось загрузить расписание" />
      ) : isWeek ? (
        <div className="cal">
          {weekCells.map((d) => (
            <div
              key={ymd(d.date)}
              className={'cal-day' + (d.isToday ? ' today' : '') + (d.isWeekend ? ' weekend' : '') + (d.items.length ? ' has' : '')}
            >
              <div className="cal-colhead">
                <div className="dow">{DOW[isoWeekday(d.date) - 1]}</div>
                <div className="dnum">{pad2(d.date.getDate())}.{pad2(d.date.getMonth() + 1)}</div>
                {d.isToday && <span className="cal-today-tag">Сегодня</span>}
              </div>
              <div className="events">
                {d.items.length === 0 ? (
                  <div className="cal-rest">
                    <span className="dash" />
                    <span className="lbl">Отдых</span>
                  </div>
                ) : (
                  d.items.map((o) => (
                    <ScheduleEvent
                      key={occKey(o)}
                      occ={o}
                      variant="week"
                      expanded={expandedKey === occKey(o)}
                      onToggle={() => toggle(occKey(o))}
                      busy={busyKey === occKey(o)}
                      onRegister={() => handleRegister(o)}
                      onCancel={() => handleCancel(o)}
                    />
                  ))
                )}
              </div>
            </div>
          ))}
        </div>
      ) : (
        <div className="cal-month">
          {DOW.map((d, i) => (
            <div key={`h${i}`} className={'cal-mhead' + (i >= 5 ? ' weekend' : '')}>
              {d}
            </div>
          ))}
          {monthCells.map((c, i) => (
            <div
              key={i}
              className={'cal-mday' + (c.inMonth ? '' : ' out') + (c.isToday ? ' today' : '') + (c.items.length ? ' has' : '')}
            >
              <div className="cal-mnum">{c.date.getDate()}</div>
              <div className="cal-mevents">
                {c.items.map((o) => (
                  <ScheduleEvent
                    key={occKey(o)}
                    occ={o}
                    variant="month"
                    expanded={expandedKey === occKey(o)}
                    onToggle={() => toggle(occKey(o))}
                    busy={busyKey === occKey(o)}
                    onRegister={() => handleRegister(o)}
                    onCancel={() => handleCancel(o)}
                  />
                ))}
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
