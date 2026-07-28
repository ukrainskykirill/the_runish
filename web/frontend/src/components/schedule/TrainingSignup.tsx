import { useState } from 'react';
import type { TrainingOccurrence } from '../../api/types';
import { api } from '../../api/client';
import { useAuth } from '../../context/AuthContext';
import { useAsync } from '../../lib/useAsync';
import { ErrorState } from '../LoadState';
import { RegisterAction, occKey, useTrainingRegister } from './registerAction';

const DOW = ['Пн', 'Вт', 'Ср', 'Чт', 'Пт', 'Сб', 'Вс'];
const MONTHS_GEN = [
  'января', 'февраля', 'марта', 'апреля', 'мая', 'июня',
  'июля', 'августа', 'сентября', 'октября', 'ноября', 'декабря',
];

/** "2026-07-02" + weekday → "Чт, 2 июля". */
function formatSessionDate(date: string, weekday: number): string {
  const [, m, d] = date.split('-').map(Number);
  return `${DOW[weekday - 1]}, ${d} ${MONTHS_GEN[m - 1]}`;
}

export function TrainingSignup() {
  const { user, refresh } = useAuth();
  const [reloadKey, setReloadKey] = useState(0);

  const { data, loading, error } = useAsync(() => api.trainingsUpcoming(), [reloadKey, user?.id]);
  const occurrences = (data?.occurrences ?? []).filter((o) => !o.past);

  async function reload() {
    setReloadKey((k) => k + 1);
    await refresh();
  }
  const { busyKey, handleRegister, handleCancel } = useTrainingRegister(reload);

  if (loading) {
    return (
      <div className="ts-loading">
        {Array.from({ length: 4 }, (_, i) => (
          <span className="sk" key={i} />
        ))}
      </div>
    );
  }
  if (error) return <ErrorState message="Не удалось загрузить занятия для записи" />;

  if (occurrences.length === 0) {
    return <div className="empty-state">Ближайших тренировок для записи пока нет</div>;
  }

  const groups: { date: string; weekday: number; items: TrainingOccurrence[] }[] = [];
  for (const o of occurrences) {
    const last = groups[groups.length - 1];
    if (last && last.date === o.session_date) last.items.push(o);
    else groups.push({ date: o.session_date, weekday: o.weekday, items: [o] });
  }

  return (
    <div className="ts">
      {groups.map((g) => (
        <div className="ts-group" key={g.date}>
          <div className="ts-date">{formatSessionDate(g.date, g.weekday)}</div>
          <div className="ts-cards">
            {g.items.map((o) => (
              <TrainingCard
                key={occKey(o)}
                occ={o}
                busy={busyKey === occKey(o)}
                onRegister={() => handleRegister(o)}
                onCancel={() => handleCancel(o)}
              />
            ))}
          </div>
        </div>
      ))}
    </div>
  );
}

interface CardProps {
  occ: TrainingOccurrence;
  busy: boolean;
  onRegister: () => void;
  onCancel: () => void;
}

function TrainingCard({ occ, busy, onRegister, onCancel }: CardProps) {
  const { user } = useAuth();
  const spotsLeft =
    occ.capacity != null ? Math.max(occ.capacity - occ.registered_count, 0) : null;

  return (
    <div className={'tcard' + (occ.registered ? ' is-reg' : '') + (occ.cancelled ? ' is-cancelled' : '')}>
      <div className="tcard-main">
        <div className="tcard-title">
          {occ.title}
          {occ.kind === 'sunday_runish' ? <span className="chip chip-track">Sunday Runish</span> : null}
        </div>
        <div className="tcard-meta">
          <span className="tcard-time">{occ.start_time}</span>
          {occ.place_url ? (
            <a className="tcard-place tcard-place-link" href={occ.place_url} target="_blank" rel="noopener noreferrer">{occ.place}</a>
          ) : (
            <span className="tcard-place">{occ.place}</span>
          )}
        </div>
        <div className="tcard-info">
          {occ.cancelled ? (
            <span className="tcard-note red">Занятие отменено</span>
          ) : (
            <>
              {spotsLeft != null && (
                <span className={'tcard-note' + (spotsLeft === 0 ? ' red' : '')}>
                  {spotsLeft === 0 ? 'Мест нет' : `Осталось мест: ${spotsLeft}`}
                </span>
              )}
              {user && occ.registered && occ.quota_left != null && (
                <span className="tcard-note">По абонементу осталось: {occ.quota_left}</span>
              )}
            </>
          )}
        </div>
      </div>
      <div className="tcard-action">
        <RegisterAction occ={occ} busy={busy} onRegister={onRegister} onCancel={onCancel} />
      </div>
    </div>
  );
}
