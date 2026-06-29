export interface Service {
  id: number;
  kind: string;
  title: string;
  description: string;
  price_kop: number;
  duration_days?: number;
  sort_order: number;
  is_active: boolean;
  price_with_sub_kop?: number;
  promo_price_kop?: number;
  // Вычисляется сервером под текущего юзера и акцию:
  effective_price_kop: number;
  locked: boolean; // подписка недоступна без оплаченного взноса
  owned: boolean; // взнос/бандл уже оплачен
  created_at: string;
  updated_at: string;
}

export interface CatalogResponse {
  services: Service[];
  promo_active: boolean;
}

export interface HomeResponse {
  services: Service[];
  promo_active: boolean;
  news: NewsItem[];
  merch: MerchItem[];
  trainings: Training[];
}

export interface NewsItem {
  id: number;
  title: string;
  content: string;
  sort_order: number;
  is_active: boolean;
  published_at?: string;
  created_at: string;
  updated_at: string;
}

export interface MerchItem {
  id: number;
  title: string;
  description: string;
  price_kop: number;
  sort_order: number;
  is_active: boolean;
  created_at: string;
  updated_at: string;
}

export interface Training {
  id: number;
  title: string;
  weekday: number; // ISO: 1=Пн … 7=Вс
  start_time: string; // "HH:MM"
  place: string;
  is_active: boolean;
  sort_order: number;
  created_at: string;
  updated_at: string;
}

export interface User {
  id: number;
  telegram_id: number;
  username: string;
  full_name: string;
  phone: string;
  bot_dialog_open: boolean;
  is_admin: boolean;
  entry_fee_paid: boolean;
  created_at: string;
}

export interface OrderItem {
  id: number;
  order_id: number;
  service_id: number;
  title: string;
  price_kop: number;
  qty: number;
}

export interface Order {
  id: number;
  user_id: number;
  amount_kop: number;
  status: string;
  contact_phone: string;
  contact_name: string;
  contact_tg: string;
  created_at: string;
  items?: OrderItem[];
}

export interface Subscription {
  id: number;
  user_id: number;
  service_id: number;
  service_title: string;
  payment_id?: number;
  status: string;
  started_at: string;
  expires_at: string;
  reminded_7d: boolean;
  reminded_3d: boolean;
  reminded_1d: boolean;
  created_at: string;
}

export interface CartLine {
  service_id: number;
  title: string;
  price_kop: number;
  qty: number;
  subtotal: number;
}

export interface CartResponse {
  lines: CartLine[];
  total: number;
  count: number;
}

export interface MeResponse {
  user: User | null;
  subscriptions: Subscription[];
  orders: Order[];
  can_book_free_lesson: boolean;
  can_choose_subscription: boolean;
  bot_username: string;
}

export interface CheckoutResponse {
  order_id: number;
  payment_url: string;
}

export interface TelegramStartResponse {
  bot_username: string;
  nonce: string;
}
