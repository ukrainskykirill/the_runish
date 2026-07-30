import type { PlanGroup, PlanMaterial } from '../../api/types';

const WEEKDAYS = ['пн', 'вт', 'ср', 'чт', 'пт', 'сб', 'вс'];

/** "2026-07-20" → "20.07" */
function dayLabel(date: string): string {
  const [, m, d] = date.split('-');
  return `${d}.${m}`;
}

function todayISO(): string {
  const now = new Date();
  const m = String(now.getMonth() + 1).padStart(2, '0');
  const d = String(now.getDate()).padStart(2, '0');
  return `${now.getFullYear()}-${m}-${d}`;
}

interface WeekPlanViewProps {
  groups: PlanGroup[];
  materials?: PlanMaterial[];
}

export function WeekPlanView({ groups, materials = [] }: WeekPlanViewProps) {
  const today = todayISO();

  if (groups.length === 0) {
    return <div className="empty-state">План на эту неделю пока не опубликован</div>;
  }

  return (
    <>
      <div className="plan-groups">
        {groups.map((group, gi) => (
          <article className="plan-card" key={gi}>
            <h3 className="plan-card-title">{group.title}</h3>
            <ul className="plan-days">
              {group.days.map((day) => {
                const rest = !day.kind && !day.task;
                return (
                  <li
                    className={`plan-day${rest ? ' rest' : ''}${day.date === today ? ' today' : ''}`}
                    key={day.date}
                  >
                    <div className="plan-day-when">
                      <b>{WEEKDAYS[day.weekday - 1]}</b>
                      <span>{dayLabel(day.date)}</span>
                    </div>
                    <div className="plan-day-body">
                      <div className="plan-day-kind">{day.kind || 'Выходной'}</div>
                      {day.task ? <div className="plan-day-task">{day.task}</div> : null}
                      {day.link_url ? (
                        <a className="plan-day-link" href={day.link_url} target="_blank" rel="noreferrer">
                          {day.link_label || 'Подробнее'}
                        </a>
                      ) : null}
                    </div>
                  </li>
                );
              })}
            </ul>
          </article>
        ))}
      </div>

      {materials.length > 0 && (
        <div className="plan-materials">
          <span className="lab">Материалы</span>
          {materials.map((m) => (
            <a key={m.url} className="btn btn-ghost btn-sm" href={m.url} target="_blank" rel="noreferrer">
              {m.label}
            </a>
          ))}
        </div>
      )}
    </>
  );
}
