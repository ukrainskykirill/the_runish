import { useEffect, useReducer, useRef } from 'react';
import { api, formatDate } from '../../api/client';
import type { NewsItem } from '../../api/types';
import { CloseIcon } from '../icons';

interface NewsModalProps {
  newsId: number | null;
  onClose: () => void;
}

export function NewsModal({ newsId, onClose }: NewsModalProps) {
  const [state, dispatch] = useReducer(newsModalReducer, { item: null, loading: false });
  const requestSeq = useRef(0);

  useEffect(() => {
    if (newsId == null) {
      dispatch({ type: 'reset' });
      return;
    }
    const seq = ++requestSeq.current;
    dispatch({ type: 'start' });
    api
      .newsById(newsId)
      .then((data) => {
        if (requestSeq.current === seq) dispatch({ type: 'success', item: data });
      })
      .catch(() => {
        if (requestSeq.current === seq) dispatch({ type: 'error' });
      });
  }, [newsId]);

  useEffect(() => {
    if (newsId == null) return;
    document.body.style.overflow = 'hidden';
    function onKey(e: KeyboardEvent) {
      if (e.key === 'Escape') onClose();
    }
    document.addEventListener('keydown', onKey);
    return () => {
      document.body.style.overflow = '';
      document.removeEventListener('keydown', onKey);
    };
  }, [newsId, onClose]);

  if (newsId == null) return null;

  const date = state.item ? (state.item.published_at ?? state.item.created_at) : null;

  return (
    <div
      className="modal-bg"
      onClick={(e) => {
        if (e.target === e.currentTarget) onClose();
      }}
    >
      <div className="modal news-modal">
        <button className="modal-close" onClick={onClose} aria-label="Закрыть">
          <CloseIcon className="i i-sm" />
        </button>
        {state.loading || !state.item ? (
          <div className="modal-body" style={{ textAlign: 'center', padding: '48px 24px', color: 'var(--text-faint)' }}>
            Загрузка…
          </div>
        ) : (
          <>
            <div className="modal-top">
              <span className="modal-sl speedlines" />
              {date && (
                <div style={{ position: 'relative', color: 'rgba(244,236,221,0.85)', fontSize: 12, textTransform: 'uppercase', letterSpacing: '0.08em', fontWeight: 600, marginBottom: 10 }}>
                  {formatDate(date)}
                </div>
              )}
              <h3>{state.item.title}</h3>
            </div>
            <div className="modal-body">
              <p style={{ whiteSpace: 'pre-line', color: 'var(--text-body)', lineHeight: 1.65, margin: 0 }}>{state.item.content}</p>
            </div>
          </>
        )}
      </div>
    </div>
  );
}

interface NewsModalState {
  item: NewsItem | null;
  loading: boolean;
}

type NewsModalAction =
  | { type: 'reset' }
  | { type: 'start' }
  | { type: 'success'; item: NewsItem }
  | { type: 'error' };

function newsModalReducer(_state: NewsModalState, action: NewsModalAction): NewsModalState {
  switch (action.type) {
    case 'reset':
      return { item: null, loading: false };
    case 'start':
      return { item: null, loading: true };
    case 'success':
      return { item: action.item, loading: false };
    case 'error':
      return { item: null, loading: false };
  }
}
