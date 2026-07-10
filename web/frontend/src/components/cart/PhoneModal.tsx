import { useState } from 'react';
import { createPortal } from 'react-dom';
import { api, ApiError } from '../../api/client';
import { CloseIcon, InfoIcon } from '../icons';

interface PhoneModalProps {
  onSuccess: () => void;
  onClose: () => void;
}

export function PhoneModal({ onSuccess, onClose }: PhoneModalProps) {
  const [phone, setPhone] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (saving) return;
    setError(null);
    setSaving(true);
    try {
      await api.setPhone(phone);
      onSuccess();
    } catch (err) {
      setSaving(false);
      if (err instanceof ApiError && err.code === 'invalid_phone') {
        setError('Проверьте номер — нужен российский, например +7 999 123-45-67.');
      } else {
        setError('Не удалось сохранить номер. Попробуйте ещё раз.');
      }
    }
  }

  return createPortal(
    <div
      className="modal-bg"
      onClick={(e) => {
        if (e.target === e.currentTarget) onClose();
      }}
    >
      <div className="modal">
        <button className="modal-close" onClick={onClose} aria-label="Закрыть">
          <CloseIcon className="i i-sm" />
        </button>
        <div className="modal-top">
          <div className="modal-sl speedlines" />
          <h3>Номер телефона</h3>
          <p>Он нужен для чека по 54-ФЗ и попадёт только в чек — рекламы не будет.</p>
        </div>
        <form className="modal-body" onSubmit={handleSubmit}>
          <input
            className="modal-input"
            type="tel"
            inputMode="tel"
            autoFocus
            placeholder="+7 999 123-45-67"
            value={phone}
            onChange={(e) => setPhone(e.target.value)}
          />
          {error ? (
            <div className="modal-note">
              <InfoIcon className="i i-sm" />
              {error}
            </div>
          ) : null}
          <button className="btn btn-primary btn-block" type="submit" disabled={saving}>
            {saving ? 'Сохраняем…' : 'Сохранить и оплатить'}
          </button>
        </form>
      </div>
    </div>,
    document.body,
  );
}
