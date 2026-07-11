import { useEffect, useState } from 'react';
import { createPortal } from 'react-dom';
import { useLocation, useNavigate } from 'react-router-dom';
import { useAuth } from '../../context/AuthContext';
import { CloseIcon } from '../icons';

export function SurveyNudgeModal() {
  const { user, surveyStatus, loading } = useAuth();
  const navigate = useNavigate();
  const location = useLocation();
  const [dismissed, setDismissed] = useState(false);

  useEffect(() => {
    if (!user) {
      setDismissed(false);
      return;
    }
    setDismissed(sessionStorage.getItem(`survey_nudge_dismissed_${user.id}`) === '1');
  }, [user]);

  const incomplete = surveyStatus === 'pending' || surveyStatus === 'in_progress';
  const onSurveyPage = location.pathname === '/survey';
  const open = !loading && !!user && incomplete && !onSurveyPage && !dismissed;
  if (!open) return null;

  function close() {
    if (user) sessionStorage.setItem(`survey_nudge_dismissed_${user.id}`, '1');
    setDismissed(true);
  }

  function goSurvey() {
    close();
    navigate('/survey');
  }

  const inProgress = surveyStatus === 'in_progress';

  return createPortal(
    <div
      className="modal-bg"
      onClick={(e) => {
        if (e.target === e.currentTarget) close();
      }}
    >
      <div className="modal">
        <button className="modal-close" onClick={close} aria-label="Закрыть">
          <CloseIcon className="i i-sm" />
        </button>
        <div className="modal-top">
          <div className="modal-sl speedlines" />
          <h3>{inProgress ? 'Закончи анкету бегуна' : 'Добро пожаловать в The Runish!'}</h3>
          <p>
            {inProgress
              ? 'Ты начал анкету бегуна, но не закончил. Заверши её — так мы подберём тренировки под тебя.'
              : 'Пройди короткую анкету бегуна — пара минут, и мы подберём тебе подходящие тренировки.'}
          </p>
        </div>
        <div className="modal-body">
          <button className="btn btn-primary btn-block" onClick={goSurvey}>
            {inProgress ? 'Закончить анкету' : 'Пройти анкету'}
          </button>
          <div className="modal-actions">
            <button className="btn btn-ghost btn-sm" onClick={close}>
              Позже
            </button>
          </div>
        </div>
      </div>
    </div>,
    document.body,
  );
}
