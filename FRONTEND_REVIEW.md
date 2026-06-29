# Frontend Review

Дата: 2026-06-27

Цель: зафиксировать проблемы и точки ускорения фронтенда. Большая часть найденных пунктов уже исправлена в последующих правках.

## Короткий итог

Фронтенд собирается (`npm run build`) и проходит lint (`npm run lint`). Основные найденные проблемы закрыты: добавлены loading/error состояния, route-level lazy loading, кэш для публичных read-only запросов, защита от гонок в модалке новостей и одноразовая обработка Telegram polling.

## Находки

### 1. Hero-видео может задерживать первый видимый экран

Файл: `web/frontend/src/pages/HomePage.tsx:116`

Статус: частично исправлено. Добавлен лёгкий poster `web/frontend/public/hero-run-poster.jpg`; видео оставлено с `preload="auto"`, чтобы autoplay не зависал на poster. WebM-пробник оказался тяжелее MP4, поэтому не подключался.

Сейчас:

```tsx
<video src="/hero-run.mp4" poster="/hero-run-poster.jpg" preload="auto" autoPlay muted loop playsInline />
```

Что было не так:

- Сам файл `web/frontend/public/hero-run.mp4` весит около `2.3M`, в `dist` сейчас около `3.1M`.
- Видео находится в первом экране, поэтому оно конкурирует с JS/CSS/шрифтами за сеть и декодирование.

Что ещё можно обсудить позднее:

- Подготовить более лёгкие версии: mobile/desktop, возможно WebM + MP4 fallback.
- Проверить, нужен ли autoplay на мобильных сразу или можно начинать после `canplay`.

### 2. Главная делает 4 независимых запроса при первом открытии

Файлы:

- `web/frontend/src/pages/HomePage.tsx:24`
- `web/frontend/src/pages/HomePage.tsx:25`
- `web/frontend/src/pages/HomePage.tsx:26`
- `web/frontend/src/components/schedule/ScheduleBoard.tsx:41`

Статус: исправлено для главной. Добавлен агрегирующий `/api/home`, главная теперь берёт `catalog/news/merch/schedule` одним запросом. Для отдельных read-only страниц добавлен небольшой session cache.

На главной одновременно грузятся:

- `/api/catalog`
- `/api/news`
- `/api/merch`
- `/api/schedule`

Плюс глобальные контексты при старте приложения грузят `/api/me` и `/api/cart`.

Что не так:

- На первом экране это 5-6 запросов сразу.
- Данные не кэшируются между страницами. Например, главная грузит schedule, потом `/schedule` снова грузит schedule.
- При медленном туннеле/мобильной сети секции могут быть пустыми до завершения запросов, без skeleton/error-state.

Что было сделано:

- Добавлен `/api/home`.
- Добавлен кэш для `news`, `newsById`, `merch`, `schedule`.
- Добавлены явные loading/error состояния.

### 3. `useAsync` скрывает ошибки и провоцирует повторяющийся шаблон загрузки

Файл: `web/frontend/src/lib/useAsync.ts:15`

Статус: исправлено.

Что было не так:

- Ошибка сохраняется в `error`, но большинство страниц её не показывают.
- Нет `AbortController`, только флаг `active`; запрос продолжает идти в фоне.
- Линтер ругается на синхронные `setLoading/setError` внутри effect.
- Из-за `// eslint-disable-next-line react-hooks/exhaustive-deps` легко случайно поймать устаревший callback.

Что было сделано:

- `useAsync` переведён на reducer.
- Добавлен `AbortController`.
- Страницы начали показывать loading/error states.

### 4. В расписании есть обычный `<a href="/runners">`, который перезагружает SPA

Файл: `web/frontend/src/components/schedule/ScheduleBoard.tsx:120`

Статус: исправлено. Карточка тренировки теперь использует `Link` из `react-router-dom`.

Что не так:

- Это полный переход браузера, а не SPA-навигация.
- Теряются in-memory состояния: открытые dropdown, возможный кэш, текущие React-состояния.
- На туннеле полный reload заметнее, чем `Link`.

Что было сделано:

- Полный reload заменён на SPA-переход.
- Текущий путь оставлен `/runners`.

### 5. `NewsModal` может показать устаревшую новость при быстром переключении

Файл: `web/frontend/src/components/news/NewsModal.tsx:15`

Статус: исправлено.

Что было не так:

- Запрос новости не отменяется.
- Если быстро открыть одну новость, закрыть и открыть другую, первый медленный ответ может перезаписать `item`.
- Линтер также ругается на `setItem(null)` внутри effect.

Что было сделано:

- Добавлен request sequence guard.
- `newsById` теперь использует кэш.
- Состояние модалки переведено на reducer.

### 6. Telegram Login polling может вызвать `onSuccess` несколько раз

Файл: `web/frontend/src/components/auth/LoginPanel.tsx:35`

Статус: исправлено.

Что было не так:

- Интервал продолжает тикать, пока компонент не размонтирован.
- При статусе `confirmed` вызывается `authTelegramComplete`, затем `onSuccess`, но нет локального флага “уже завершаем”.
- Если complete/onSuccess задержатся, следующий тик теоретически может повторить завершение.

Что было сделано:

- Добавлен `completingRef`.
- Повторная обработка `confirmed` и dev-login больше не запускается параллельно.

### 7. Линтер сейчас не проходит

Команда: `npm run lint`

Статус: исправлено. `npm run lint` проходит.

Изначальный результат: 6 ошибок.

Основные категории:

- `react-hooks/set-state-in-effect`:
  - `web/frontend/src/components/layout/Header.tsx:26`
  - `web/frontend/src/components/news/NewsModal.tsx:17`
  - `web/frontend/src/lib/useAsync.ts:17`
- `react-refresh/only-export-components`:
  - `web/frontend/src/context/AuthContext.tsx`
  - `web/frontend/src/context/CartContext.tsx`
  - `web/frontend/src/context/UIContext.tsx`

Что было сделано:

- Убран лишний sync state update из `Header`.
- Для context-файлов добавлен scoped override `react-refresh/only-export-components`, потому что они закономерно экспортируют provider и hook.
- `useAsync` и `NewsModal` переведены с набора `setState` на reducers.

### 8. Один JS-бандл для всех страниц

Сборка сейчас даёт один основной bundle:

Статус: частично исправлено. Добавлен route-level lazy loading.

Что было не так:

- Пользователь главной сразу получает код личного кабинета, юридических страниц, корзины, админ-не связанного UI и т.п.
- Пока размер терпимый, но рост страниц быстро сделает первый экран тяжелее.

Что было сделано:

- Страницы кроме главной теперь грузятся через `React.lazy`.
- После правки сборка выделяет отдельные чанки страниц, а основной JS уменьшился примерно с `283K` до `263K`.

### 9. Нет явной стратегии для состояния после оплаты

Файл: `web/frontend/src/pages/PaymentResultPage.tsx`

Статус: частично исправлено. Страница успеха несколько раз обновляет auth/cart после возврата с оплаты, чтобы поймать подписку при небольшой задержке webhook.

Что ещё можно обсудить позднее:

- На странице успеха можно показывать “обновляем статус заказа” и polling заказа/платежа, если webhook может прийти с задержкой.
- Сейчас пользователь может попасть в личный кабинет раньше, чем T-Bank webhook активирует подписку.

### 10. Loading states выглядят как пустые секции

Файлы:

- `web/frontend/src/pages/CatalogPage.tsx`
- `web/frontend/src/pages/NewsPage.tsx`
- `web/frontend/src/pages/MerchPage.tsx`
- `web/frontend/src/components/schedule/ScheduleBoard.tsx`

Статус: исправлено.

Что было не так:

- Пока `loading=true`, сетки просто пустые.
- Если запрос медленный, пользователь видит пустую область без понимания, что данные ещё грузятся.
- Если запрос упал, часто состояние визуально похоже на пустой каталог.

Что было сделано:

- Добавлены `LoadingGrid` и `ErrorState`.
- Каталог, новости, мерч и расписание теперь различают loading/error/empty.

## Быстрые потенциальные улучшения без большой архитектуры

1. Подготовить более лёгкие версии hero-видео: mobile/desktop.
2. При необходимости вынести Telegram/LoginModal flow в отдельный lazy chunk.
3. Добавить более точный polling конкретного заказа после оплаты, если backend даст endpoint статуса заказа.

## Проверки

- `npm run build` — проходит.
- `npm run lint` — проходит.
