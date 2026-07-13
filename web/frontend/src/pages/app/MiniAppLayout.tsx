import { NavLink, Outlet } from 'react-router-dom';
import '../../styles/miniapp.css';
import { useTelegram } from '../../hooks/useTelegram';
import { useAuth } from '../../context/AuthContext';
import { Toast } from '../../components/Toast';
import { HomeIcon, CalendarIcon, ListIcon, UserIcon } from '../../components/icons';
import { AuthGate } from './AuthGate';

const TABS = [
  { to: '/app', label: 'Главная', icon: HomeIcon, end: true },
  { to: '/app/schedule', label: 'Расписание', icon: CalendarIcon, end: false },
  { to: '/app/subscriptions', label: 'Абонементы', icon: ListIcon, end: false },
  { to: '/app/profile', label: 'Профиль', icon: UserIcon, end: false },
];

export function MiniAppLayout() {
  useTelegram();
  const { user, loading } = useAuth();

  if (loading) return null;

  if (!user) {
    return (
      <div className="tg-app">
        <AuthGate />
        <Toast />
      </div>
    );
  }

  return (
    <div className="tg-app">
      <div className="tg-body">
        <Outlet />
      </div>
      <nav className="tabbar">
        {TABS.map(({ to, label, icon: Icon, end }) => (
          <NavLink key={to} to={to} end={end} className={({ isActive }) => 'tab' + (isActive ? ' on' : '')}>
            <Icon className="i" />
            {label}
          </NavLink>
        ))}
      </nav>
      <Toast />
    </div>
  );
}
