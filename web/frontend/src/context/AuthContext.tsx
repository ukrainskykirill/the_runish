import { createContext, useCallback, useContext, useEffect, useState, type ReactNode } from 'react';
import { api } from '../api/client';
import type { Order, Subscription, User } from '../api/types';

interface AuthContextValue {
  user: User | null;
  subscriptions: Subscription[];
  orders: Order[];
  canBookFreeLesson: boolean;
  canChooseSubscription: boolean;
  botUsername: string;
  loading: boolean;
  refresh: () => Promise<void>;
  logout: () => Promise<void>;
}

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [subscriptions, setSubscriptions] = useState<Subscription[]>([]);
  const [orders, setOrders] = useState<Order[]>([]);
  const [canBookFreeLesson, setCanBookFreeLesson] = useState(true);
  const [canChooseSubscription, setCanChooseSubscription] = useState(true);
  const [botUsername, setBotUsername] = useState('');
  const [loading, setLoading] = useState(true);

  const refresh = useCallback(async () => {
    try {
      const data = await api.me();
      setUser(data.user);
      setSubscriptions(data.subscriptions);
      setOrders(data.orders);
      setCanBookFreeLesson(data.can_book_free_lesson);
      setCanChooseSubscription(data.can_choose_subscription);
      setBotUsername(data.bot_username);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    refresh();
  }, [refresh]);

  const logout = useCallback(async () => {
    await api.logout();
    setUser(null);
    setSubscriptions([]);
    setOrders([]);
    setCanBookFreeLesson(true);
    setCanChooseSubscription(true);
  }, []);

  return (
    <AuthContext.Provider
      value={{
        user,
        subscriptions,
        orders,
        canBookFreeLesson,
        canChooseSubscription,
        botUsername,
        loading,
        refresh,
        logout,
      }}
    >
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error('useAuth must be used within AuthProvider');
  return ctx;
}
