import { useState } from 'react';
import { Link } from 'react-router-dom';
import { api, formatDate } from '../../api/client';
import { useAuth } from '../../context/AuthContext';
import { useAsync } from '../../lib/useAsync';
import { TELEGRAM_LINKS } from '../../lib/constants';
import monogramCream from '../../assets/monogram-cream.png';
import { BoxIcon, CalendarIcon, ListIcon, TelegramIcon } from '../../components/icons';

export function MiniHomePage() {
  const [renderedAt] = useState(() => Date.now());
  const { subscriptions } = useAuth();
  const { data } = useAsync(() => api.home(), []);
  const news = data?.news ?? [];

  const sub = subscriptions[0];
  let progressPct = 0;
  if (sub) {
    const start = new Date(sub.started_at).getTime();
    const end = new Date(sub.expires_at).getTime();
    progressPct = end > start ? Math.min(100, Math.max(0, ((renderedAt - start) / (end - start)) * 100)) : 0;
  }

  return (
    <div className="pad stack">
      <div className="m-hero">
        <span className="hsl speedlines" />
        <img className="hmono" src={monogramCream} alt="" />
        <div className="heb">Беговой клуб · Москва</div>
        <h2>
          Рождённые
          <br />
          двигаться
        </h2>
      </div>

      {sub ? (
        <div className="sub-status">
          <div className="top">
            <div>
              <div className="lab">Мой абонемент</div>
              <div className="name">{sub.service_title}</div>
            </div>
            <span className="chip chip-track">Активен</span>
          </div>
          <div className="bar">
            <i style={{ width: `${progressPct}%` }} />
          </div>
          <div className="until">
            Действует до <b>{formatDate(sub.expires_at)}</b>
          </div>
        </div>
      ) : (
        <div className="sub-status">
          <div className="top">
            <div>
              <div className="lab">Абонемент</div>
              <div className="name">Не оформлен</div>
            </div>
          </div>
          <p className="until">
            <Link className="btn btn-primary btn-sm" to="/app/subscriptions">
              Выбрать абонемент
            </Link>
          </p>
        </div>
      )}

      <div className="tiles">
        <Link className="tile" to="/app/schedule">
          <span className="ti">
            <CalendarIcon className="i i-sm" />
          </span>
          <div>
            <div className="tt">Расписание</div>
            <div className="td">Запись в один тап</div>
          </div>
        </Link>
        <Link className="tile" to="/app/subscriptions">
          <span className="ti">
            <ListIcon className="i i-sm" />
          </span>
          <div>
            <div className="tt">Абонементы</div>
            <div className="td">Оплата картой</div>
          </div>
        </Link>
        <a className="tile ink" href={TELEGRAM_LINKS.channel} target="_blank" rel="noopener noreferrer">
          <span className="ti">
            <TelegramIcon className="i i-sm" />
          </span>
          <div>
            <div className="tt">Канал клуба</div>
            <div className="td">Анонсы</div>
          </div>
        </a>
        <Link className="tile ink" to="/app/profile">
          <span className="ti">
            <BoxIcon className="i i-sm" />
          </span>
          <div>
            <div className="tt">Профиль</div>
            <div className="td">Мои записи</div>
          </div>
        </Link>
      </div>

      {news.length ? (
        <div>
          <div className="sec-top">
            <h3 className="m-h">Что в клубе</h3>
          </div>
          <div className="hscroll">
            {news.slice(0, 6).map((n) => (
              <div className="ncard" key={n.id}>
                <div className="nh">
                  <span className="nsl speedlines" />
                </div>
                <div className="nb">
                  <div className="date">{n.published_at ? formatDate(n.published_at) : ''}</div>
                  <h4>{n.title}</h4>
                </div>
              </div>
            ))}
          </div>
        </div>
      ) : null}
    </div>
  );
}
