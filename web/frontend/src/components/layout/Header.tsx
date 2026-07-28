import { useEffect, useRef, useState } from 'react';
import { Link, NavLink } from 'react-router-dom';
import monogramRed from '../../assets/monogram-red.png';
import { useAuth } from '../../context/AuthContext';
import { useCart } from '../../context/CartContext';
import { useUI } from '../../context/UIContext';
import { getInitials } from '../../lib/format';
import { AccountDropdown } from '../account/AccountDropdown';
import { CartDropdown } from '../cart/CartDropdown';
import { CartIcon, ChevronDownIcon, CloseIcon, MenuIcon, TelegramIcon } from '../icons';

const navLinkClass = ({ isActive }: { isActive: boolean }) => (isActive ? 'active' : undefined);

export function Header() {
  const { user } = useAuth();
  const { count } = useCart();
  const { openLoginModal } = useUI();
  const [cartOpen, setCartOpen] = useState(false);
  const [acctOpen, setAcctOpen] = useState(false);
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    function onClick(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) {
        setCartOpen(false);
        setAcctOpen(false);
      }
    }
    document.addEventListener('click', onClick);
    return () => document.removeEventListener('click', onClick);
  }, []);

  return (
    <header className="hdr">
      <div className="wrap hdr-in">
        <Link className="brand" to="/">
          <img src={monogramRed} alt="The Runish" />
          <b>The Runish</b>
        </Link>
        <nav className="nav">
          <NavLink to="/" end className={navLinkClass}>
            Главная
          </NavLink>
          <NavLink to="/news" className={navLinkClass}>
            Анонсы
          </NavLink>
          <NavLink to="/schedule" className={navLinkClass}>
            Расписание
          </NavLink>
          <NavLink to="/runners" className={navLinkClass}>
            Тренировки
          </NavLink>
          <NavLink to="/merch" className={navLinkClass}>
            Мерч
          </NavLink>
        </nav>
        <div className="hdr-right" ref={ref}>
          <button
            className="icon-btn"
            aria-label="Корзина"
            onClick={() => {
              setCartOpen((o) => !o);
              setAcctOpen(false);
            }}
          >
            <CartIcon />
            {count > 0 ? <span className="badge">{count}</span> : null}
          </button>

          {user ? (
            <button
              className="acct"
              onClick={() => {
                setAcctOpen((o) => !o);
                setCartOpen(false);
              }}
            >
              <span className="av">{getInitials(user.full_name || user.username)}</span>
              <span className="nm">{user.full_name || user.username}</span>
              <ChevronDownIcon className="i i-sm" style={{ color: 'var(--text-faint)' }} />
            </button>
          ) : (
            <button className="btn btn-primary btn-sm" onClick={() => openLoginModal()}>
              <TelegramIcon className="i tg-i" />
              Войти
            </button>
          )}

          <button
            className="hamburger"
            aria-label="Меню"
            onClick={() => setMobileMenuOpen((o) => !o)}
          >
            {mobileMenuOpen ? <CloseIcon className="i" /> : <MenuIcon className="i" />}
          </button>

          <CartDropdown open={cartOpen} onClose={() => setCartOpen(false)} />
          <AccountDropdown open={acctOpen} onClose={() => setAcctOpen(false)} />
        </div>
      </div>

      {mobileMenuOpen && (
        <nav className="mob-menu">
          <NavLink to="/" end className={navLinkClass} onClick={() => setMobileMenuOpen(false)}>
            Главная
          </NavLink>
          <NavLink to="/news" className={navLinkClass} onClick={() => setMobileMenuOpen(false)}>
            Анонсы
          </NavLink>
          <NavLink to="/schedule" className={navLinkClass} onClick={() => setMobileMenuOpen(false)}>
            Расписание
          </NavLink>
          <NavLink to="/runners" className={navLinkClass} onClick={() => setMobileMenuOpen(false)}>
            Тренировки
          </NavLink>
          <NavLink to="/merch" className={navLinkClass} onClick={() => setMobileMenuOpen(false)}>
            Мерч
          </NavLink>
        </nav>
      )}
    </header>
  );
}
