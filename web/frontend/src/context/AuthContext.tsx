import { createContext, useCallback, useContext, useEffect, useState, type ReactNode } from 'react';
import { api } from '../api/client';
import type { Order, Subscription, SurveyStatus, TrainingRegistration, User } from '../api/types';

interface AuthContextValue {
  user: User | null;
  subscriptions: Subscription[];
  orders: Order[];
  trainingRegistrations: TrainingRegistration[];
  canBookFreeLesson: boolean;
  canChooseSubscription: boolean;
  botUsername: string;
  vkEnabled: boolean;
  surveyStatus: SurveyStatus;
  loading: boolean;
  refresh: () => Promise<void>;
  logout: () => Promise<void>;
}

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [subscriptions, setSubscriptions] = useState<Subscription[]>([]);
  const [orders, setOrders] = useState<Order[]>([]);
  const [trainingRegistrations, setTrainingRegistrations] = useState<TrainingRegistration[]>([]);
  const [canBookFreeLesson, setCanBookFreeLesson] = useState(true);
  const [canChooseSubscription, setCanChooseSubscription] = useState(true);
  const [botUsername, setBotUsername] = useState('');
  const [vkEnabled, setVkEnabled] = useState(false);
  const [surveyStatus, setSurveyStatus] = useState<SurveyStatus>('pending');
  const [loading, setLoading] = useState(true);

  const refresh = useCallback(async () => {
    try {
      const data = await api.me();
      setUser(data.user);
      setSubscriptions(data.subscriptions);
      setOrders(data.orders);
      setTrainingRegistrations(data.training_registrations ?? []);
      setCanBookFreeLesson(data.can_book_free_lesson);
      setCanChooseSubscription(data.can_choose_subscription);
      setBotUsername(data.bot_username);
      setVkEnabled(data.vk_enabled);
      setSurveyStatus(data.survey_status);
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
    setTrainingRegistrations([]);
    setCanBookFreeLesson(true);
    setCanChooseSubscription(true);
    setSurveyStatus('pending');
  }, []);

  return (
    <AuthContext.Provider
      value={{
        user,
        subscriptions,
        orders,
        trainingRegistrations,
        canBookFreeLesson,
        canChooseSubscription,
        botUsername,
        vkEnabled,
        surveyStatus,
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
