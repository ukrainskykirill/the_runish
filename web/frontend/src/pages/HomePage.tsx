import { useState } from 'react';
import { Link } from 'react-router-dom';
import { api, formatPrice } from '../api/client';
import type { Training } from '../api/types';
import logoWordmark from '../assets/logo-wordmark.png';
import { HeroVideo } from '../components/HeroVideo';
import { EntryFeeNotice } from '../components/EntryFeeNotice';
import { FeatureCard } from '../components/cards/FeatureCard';
import { MerchCard } from '../components/cards/MerchCard';
import { NewsCard } from '../components/cards/NewsCard';
import { ServiceCard } from '../components/cards/ServiceCard';
import { TelegramCard } from '../components/cards/TelegramCard';
import { SubscribeBanner } from '../components/SubscribeBanner';
import { NewsModal } from '../components/news/NewsModal';
import { ScheduleCalendar } from '../components/schedule/ScheduleCalendar';
import { useAuth } from '../context/AuthContext';
import { useCart } from '../context/CartContext';
import { useUI } from '../context/UIContext';
import { BoltIcon, BoxIcon, CommunityIcon } from '../components/icons';
import { TELEGRAM_LINKS } from '../lib/constants';
import { useAsync } from '../lib/useAsync';

const EMPTY_TRAININGS: Training[] = [];

export function HomePage() {
  const { add } = useCart();
  const { user, subscriptions, canBookFreeLesson, loading: authLoading, refresh } = useAuth();
  const { showToast } = useUI();
  const { data: homeData } = useAsync(() => api.home(), []);
  const [freeLessonBusy, setFreeLessonBusy] = useState(false);
  const [selectedNewsId, setSelectedNewsId] = useState<number | null>(null);

  const services = homeData?.services ?? [];
  const news = homeData?.news ?? [];
  const merch = homeData?.merch ?? [];
  const trainings = homeData?.trainings ?? EMPTY_TRAININGS;
  const subscription = services.find((s) => s.kind === 'subscription');
  const subPrice = subscription ? formatPrice(subscription.price_kop) : '';
  const hasActiveSub = subscriptions.length > 0;

  async function handleFreeLessonClick() {
    if (freeLessonBusy) return;
    setFreeLessonBusy(true);
    const botWindow = window.open('about:blank', '_blank');
    try {
      const start = await api.authTelegramStart();
      if (!start.bot_username) {
        botWindow?.close();
        showToast('Telegram-бот пока не настроен');
        return;
      }

      const botURL = `https://t.me/${start.bot_username}${start.nonce ? `?start=${start.nonce}` : ''}`;
      if (botWindow) {
        botWindow.location.href = botURL;
      } else {
        window.location.href = botURL;
      }

      if (!user && start.nonce) {
        const deadline = Date.now() + 10 * 60 * 1000;
        while (Date.now() < deadline) {
          await new Promise((resolve) => window.setTimeout(resolve, 2000));
          const { status } = await api.authTelegramStatus(start.nonce);
          if (status === 'confirmed') {
            await api.authTelegramComplete(start.nonce);
            await refresh();
            showToast('Вы вошли через Telegram');
            return;
          }
          if (status === 'expired') return;
        }
      }
    } catch {
      botWindow?.close();
      showToast('Не удалось открыть Telegram');
    } finally {
      setFreeLessonBusy(false);
    }
  }

  return (
    <>
      <section className="hero" id="top">
        <div className="hero-track" />
        <div className="wrap hero-in">
          <div className="hero-copy">
            <div className="hero-title">
              <img className="hero-mark" src={logoWordmark} alt="The Runish — Рожденные двигаться" />
              <h1 className="d1">Беговой клуб</h1>
            </div>
            <p className="lead">
              Все тренировки в одной подписке{subPrice ? ` за ${subPrice}/мес` : ''}
            </p>
            <div className="hero-actions">
              {!hasActiveSub ? (
                <a className="btn btn-on-red" href="#runners">
                  Купить подписку
                </a>
              ) : null}
              {!authLoading && canBookFreeLesson ? (
                <button
                  className="btn btn-outline-red"
                  type="button"
                  onClick={handleFreeLessonClick}
                  disabled={freeLessonBusy}
                >
                  Пробное занятие бесплатно
                </button>
              ) : null}
            </div>
          </div>
        </div>
        <div className="hero-video">
          {/* hero-петля с отказоустойчивой загрузкой (постер + авто-ретрай плея) */}
          <HeroVideo />
        </div>
      </section>

      <section className="sec alt" id="about">
        <div className="wrap">
          <div className="sec-head">
            <div className="eb">Почему мы</div>
            <h2 className="d2">Три причины начать</h2>
          </div>
          <div className="feat-grid">
            <FeatureCard icon={<BoltIcon />} title="Тренировки">
              Интервалы, длительный кросс и беговая ОФП по расписанию. Тренер ведёт — ты бежишь в
              своём темпе и растёшь.
            </FeatureCard>
            <FeatureCard icon={<CommunityIcon />} title="Сообщество">
              Чат клуба в Telegram, совместные старты и мероприятия. Здесь находят компанию для
              бега — и не только.
            </FeatureCard>
            <FeatureCard icon={<BoxIcon />} title="Подписка">
              Гибкие форматы: разовое занятие, безлимит на месяц или индивидуальное сопровождение.
              Оплата онлайн.
            </FeatureCard>
          </div>
        </div>
      </section>

      <ScheduleCalendar trainings={trainings} />

      <section className="sec alt" id="runners">
        <div className="wrap">
          <div className="sec-head between">
            <div>
              <div className="eb">Тренировки и подписки</div>
              <h2 className="d2">Подписка на бег</h2>
              <p>
                Подписка «The Runish Community» — это формат абонемента, по которому ты можешь
                посещать все тренировки клуба и заниматься по общему тренировочному плану.
              </p>
            </div>
            <Link className="btn btn-ghost btn-sm" to="/runners">
              Все тренировки
            </Link>
          </div>
          {!hasActiveSub ? (
            <SubscribeBanner
              price={subPrice}
              onBuy={() => {
                if (subscription) add(subscription.id);
              }}
            />
          ) : null}
          <EntryFeeNotice />
          <div className="cat-grid">
            {services.map((service) => (
              <ServiceCard key={service.id} service={service} onAdd={add} />
            ))}
          </div>
        </div>
      </section>

      <section className="sec" id="news">
        <div className="wrap">
          <div className="sec-head between">
            <div>
              <div className="eb">Новости</div>
              <h2 className="d2">Что в клубе</h2>
            </div>
            <Link className="btn btn-ghost btn-sm" to="/news">
              Все новости
            </Link>
          </div>
          <div className="news-grid">
            {news.map((item, i) => (
              <NewsCard key={item.id} news={item} iconIndex={i} onReadMore={setSelectedNewsId} />
            ))}
          </div>
        </div>
      </section>

      <section className="sec alt" id="merch">
        <div className="wrap">
          <div className="sec-head between">
            <div>
              <div className="eb">Мерч</div>
              <h2 className="d2">Носи клуб</h2>
            </div>
            <Link className="btn btn-ghost btn-sm" to="/merch">
              Весь магазин
            </Link>
          </div>
          <div className="merch-grid">
            {merch.map((item) => (
              <MerchCard key={item.id} item={item} />
            ))}
          </div>
        </div>
      </section>

      <section className="tg-band" id="contacts">
        <div className="wrap">
          <div className="sec-head">
            <div className="eb">Telegram</div>
            <h2 className="d2">Всегда на связи</h2>
          </div>
          <div className="tg-grid">
            <TelegramCard href={TELEGRAM_LINKS.channel} title="Канал" subtitle="Анонсы, фото и жизнь клуба" />
            <TelegramCard href={TELEGRAM_LINKS.chat} title="Чат" subtitle="Общение с участниками клуба" />
            <TelegramCard
              href={TELEGRAM_LINKS.support}
              title="Поддержка"
              subtitle="Вопросы по подписке и оплате"
            />
          </div>
        </div>
      </section>
      <NewsModal newsId={selectedNewsId} onClose={() => setSelectedNewsId(null)} />
    </>
  );
}
