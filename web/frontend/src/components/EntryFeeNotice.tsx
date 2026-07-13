import { useAuth } from '../context/AuthContext';

// EntryFeeNotice — объяснение порядка покупки: сначала разовый вступительный взнос,
// потом подписка. Прячется, если у вошедшего пользователя взнос уже оплачен.
export function EntryFeeNotice() {
  const { user } = useAuth();
  if (user?.entry_fee_paid) return null;

  return (
    <div className="entry-notice">
      <div className="en-head">
        <span className="en-badge">Как начать</span>
        <h3>Сначала — вступление в клуб</h3>
      </div>
      <p className="en-lead">
        Вступительный взнос оплачивается <b>один раз</b> и открывает доступ ко всем подпискам клуба.
        Пока он не оплачен, оформить подписку нельзя — это обязательный шаг для всех участников.
      </p>
      <ol className="en-steps">
        <li>
          <span className="en-num">1</span>
          <div>
            <b>Оплати вступительный взнос</b>
            <span>Разово, при первом оформлении</span>
          </div>
        </li>
        <li>
          <span className="en-num">2</span>
          <div>
            <b>Оформи подписку</b>
            <span>После взноса она станет доступной</span>
          </div>
        </li>
        <li>
          <span className="en-num">3</span>
          <div>
            <b>Тренируйся</b>
            <span>Тренировки и сообщество</span>
          </div>
        </li>
      </ol>
      <p className="en-tip">
        💡 Хочешь начать сразу — возьми <b>«Старт в клуб»</b>: вступительный взнос и первая подписка
        одним пакетом, выгоднее, чем оплачивать по отдельности.
      </p>
    </div>
  );
}
