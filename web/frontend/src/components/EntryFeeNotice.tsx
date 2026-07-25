import { Link } from 'react-router-dom';
import { useAuth } from '../context/AuthContext';

export function EntryFeeNotice() {
  const { user } = useAuth();
  if (user?.entry_fee_paid) return null;

  return (
    <div className="entry-notice">
      <div className="en-head">
        <span className="en-badge">Как начать</span>
        <h3>Вступление в клуб</h3>
      </div>
      <p className="en-lead">
        Вступительный взнос оплачивается <b>один раз</b> и открывает доступ ко всем подпискам клуба.
        Пока он не оплачен, оформить подписку нельзя — это обязательный шаг для всех участников.
      </p>
      <ol className="en-steps">
        <li>
          <span className="en-num">1</span>
          <div>
            <b>Авторизуйся через Telegram</b>
            <Link to="/auth/telegram" className="en-reg-link">
              Регистрация
            </Link>
          </div>
        </li>
        <li>
          <span className="en-num">2</span>
          <div>
            <b>Оплати вступительный взнос</b>
            <span>Единоразовый платеж при вступлении в клуб</span>
          </div>
        </li>
        <li>
          <span className="en-num">3</span>
          <div>
            <b>Оформи подписку</b>
            <span>Она станет доступной после вступления</span>
          </div>
        </li>
        <li>
          <span className="en-num">4</span>
          <div>
            <b>Клубные тренировки и онлайн-план</b>
          </div>
        </li>
      </ol>
    </div>
  );
}
