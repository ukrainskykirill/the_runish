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

/** ISO-номер дня недели (1=Пн … 7=Вс) для JS Date. */
function isoWeekday(d: Date): number {
  return ((d.getDay() + 6) % 7) + 1;
}

function pad2(n: number): string {
  return n < 10 ? `0${n}` : `${n}`;
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

// ScheduleBoard — доска недельного расписания (без секции/заголовка).
// Переиспользуется и в секции на главной, и на отдельной странице /schedule.
interface ScheduleBoardProps {
  trainings?: Training[];
}

export function ScheduleBoard({ trainings: providedTrainings }: ScheduleBoardProps) {
  const shouldFetch = providedTrainings === undefined;
  const { data, loading, error } = useAsync(
    () => (shouldFetch ? api.schedule() : Promise.resolve({ trainings: providedTrainings })),
    [shouldFetch, providedTrainings],
  );
  const trainings = data?.trainings ?? providedTrainings ?? [];

  // «Сегодня» и текущая неделя (Пн–Вс) — по часам браузера.
  const today = new Date();
  const todayIso = isoWeekday(today);
  const monday = new Date(today);
  monday.setHours(0, 0, 0, 0);
  monday.setDate(today.getDate() - (todayIso - 1));

  // Тренировки по дню недели (внутри дня уже отсортированы по времени с бэка).
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
  const range =
    monday.getMonth() === sunday.getMonth()
      ? `${monday.getDate()} — ${sunday.getDate()} ${MONTHS_GEN[sunday.getMonth()]}`
      : `${monday.getDate()} ${MONTHS_GEN[monday.getMonth()]} — ${sunday.getDate()} ${MONTHS_GEN[sunday.getMonth()]}`;

  return (
    <div className="cal-board">
      <div className="cal-strip speedlines" />
      <div className="cal-board-head">
        <div className="wk">
          <span className="t">Эта неделя</span>
          <span className="r">{range}</span>
        </div>
        <div className="cal-legend">
          <span className="lg">
            <span className="sw red" />
            Тренировка
          </span>
        </div>
      </div>

      {shouldFetch && loading ? (
        <div className="cal-loading">
          {Array.from({ length: 7 }, (_, i) => (
            <span className="sk" key={i} />
          ))}
        </div>
      ) : shouldFetch && error ? (
        <ErrorState message="Не удалось загрузить расписание" />
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
                  <Link key={t.id} className="cal-ev" to="/runners">
                    <div className="ev-t">{t.title}</div>
                    <div className="ev-time">
                      <ClockIcon />
                      {t.start_time}
                    </div>
                    <div className="ev-place">
                      <PinIcon />
                      {t.place}
                    </div>
                  </Link>
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
