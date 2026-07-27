import { useState } from 'react';
import { Link } from 'react-router-dom';
import { api, ApiError } from '../../api/client';
import type { TrainingOccurrence } from '../../api/types';
import { useAuth } from '../../context/AuthContext';
import { useUI } from '../../context/UIContext';

export const ERR_MESSAGES: Record<string, string> = {
  entry_fee_required: 'Сначала оплатите вступительный взнос',
  access_required: 'Нужна активная подписка',
  session_full: 'Свободных мест больше нет',
  already_registered: 'Вы уже записаны на это занятие',
  session_cancelled: 'Занятие отменено',
  past_session: 'Запись на это занятие закрыта',
  not_found: 'Тренировка не найдена',
};

export function occKey(o: TrainingOccurrence): string {
  return `${o.training_id}|${o.session_date}`;
}

export type RegMode = 'registered' | 'cancelled' | 'login' | 'full' | 'register' | 'free' | 'needsub';

export function resolveRegMode(
  occ: TrainingOccurrence,
  user: ReturnType<typeof useAuth>['user'],
  canBookFreeLesson: boolean,
): RegMode {
  if (occ.registered) return 'registered';
  if (occ.cancelled) return 'cancelled';
  if (!user) return 'login';
  const spotsLeft = occ.capacity != null ? Math.max(occ.capacity - occ.registered_count, 0) : null;
  if (spotsLeft === 0) return 'full';
  if (occ.kind === 'sunday_runish' || occ.in_my_access) return 'register';
  if (canBookFreeLesson) return 'free';
  return 'needsub';
}

export function useTrainingRegister(reload: () => Promise<void>) {
  const { user } = useAuth();
  const { openLoginModal, showToast } = useUI();
  const [busyKey, setBusyKey] = useState<string | null>(null);

  async function handleRegister(o: TrainingOccurrence) {
    if (!user) {
      openLoginModal({ reason: 'training' });
      return;
    }
    setBusyKey(occKey(o));
    try {
      await api.trainingRegister(o.training_id, o.session_date);
      showToast('Вы записаны на тренировку');
      await reload();
    } catch (e) {
      const code = e instanceof ApiError ? e.code : 'unknown_error';
      if (code === 'login_required') openLoginModal({ reason: 'training' });
      else showToast(ERR_MESSAGES[code] ?? 'Не удалось записаться');
    } finally {
      setBusyKey(null);
    }
  }

  async function handleCancel(o: TrainingOccurrence) {
    if (!o.my_registration_id) return;
    setBusyKey(occKey(o));
    try {
      await api.trainingCancel(o.my_registration_id);
      showToast('Запись отменена');
      await reload();
    } catch {
      showToast('Не удалось отменить запись');
    } finally {
      setBusyKey(null);
    }
  }

  return { busyKey, handleRegister, handleCancel };
}

interface RegisterActionProps {
  occ: TrainingOccurrence;
  busy: boolean;
  onRegister: () => void;
  onCancel: () => void;
}

export function RegisterAction({ occ, busy, onRegister, onCancel }: RegisterActionProps) {
  const { user, canBookFreeLesson } = useAuth();
  const mode = resolveRegMode(occ, user, canBookFreeLesson);

  switch (mode) {
    case 'registered':
      return (
        <div className="tcard-reg">
          <span className="chip chip-track">Вы записаны</span>
          <button className="btn btn-ghost btn-sm" disabled={busy} onClick={onCancel}>
            Отменить
          </button>
        </div>
      );
    case 'cancelled':
      return (
        <button className="btn btn-ghost btn-sm" disabled>
          Отменено
        </button>
      );
    case 'full':
      return (
        <button className="btn btn-ghost btn-sm" disabled>
          Мест нет
        </button>
      );
    case 'needsub':
      return (
        <Link className="btn btn-ghost btn-sm" to="/runners">
          Нужна подписка
        </Link>
      );
    case 'free':
      return (
        <button className="btn btn-primary btn-sm" disabled={busy} onClick={onRegister}>
          Записаться на первую бесплатную 🎉
        </button>
      );
    default:
      return (
        <button className="btn btn-primary btn-sm" disabled={busy} onClick={onRegister}>
          Записаться
        </button>
      );
  }
}
