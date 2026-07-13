import { Link } from 'react-router-dom';
import monogramRed from '../../assets/monogram-red.png';
import { TELEGRAM_LINKS } from '../../lib/constants';

export function Footer() {
  return (
    <footer>
      <div className="wrap">
        <div className="foot-in">
          <div className="foot-brand">
            <img src={monogramRed} alt="The Runish" />
            <p>Беговой клуб The Runish. Сообщество людей, живущих в движении.</p>
          </div>
          <div className="foot-nav">
            <div className="foot-col">
              <h4>Клуб</h4>
              <Link to="/runners">Тренировки</Link>
              <Link to="/news">Новости</Link>
              <Link to="/merch">Мерч</Link>
            </div>
            <div className="foot-col">
              <h4>Telegram</h4>
              <a href={TELEGRAM_LINKS.channel} target="_blank" rel="noopener noreferrer">
                Канал
              </a>
              <a href={TELEGRAM_LINKS.chat} target="_blank" rel="noopener noreferrer">
                Чат
              </a>
              <a href={TELEGRAM_LINKS.support} target="_blank" rel="noopener noreferrer">
                Поддержка
              </a>
            </div>
          </div>
        </div>
        <div className="foot-legal">
          <span>© {new Date().getFullYear()} The Runish</span>
          <span className="lr">
            <Link to="/legal/offer">Публичная оферта</Link>
            <Link to="/legal/privacy">Обработка персональных данных</Link>
          </span>
        </div>
      </div>
    </footer>
  );
}
