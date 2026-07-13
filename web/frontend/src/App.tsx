import { lazy, Suspense } from 'react';
import { BrowserRouter, Route, Routes } from 'react-router-dom';
import { Layout } from './components/layout/Layout';
import { HomePage } from './pages/HomePage';

const CatalogPage = lazy(() => import('./pages/CatalogPage').then((m) => ({ default: m.CatalogPage })));
const NewsPage = lazy(() => import('./pages/NewsPage').then((m) => ({ default: m.NewsPage })));
const SchedulePage = lazy(() => import('./pages/SchedulePage').then((m) => ({ default: m.SchedulePage })));
const MerchPage = lazy(() => import('./pages/MerchPage').then((m) => ({ default: m.MerchPage })));
const CartPage = lazy(() => import('./pages/CartPage').then((m) => ({ default: m.CartPage })));
const ProfilePage = lazy(() => import('./pages/ProfilePage').then((m) => ({ default: m.ProfilePage })));
const SurveyPage = lazy(() => import('./pages/SurveyPage').then((m) => ({ default: m.SurveyPage })));
const LoginPage = lazy(() => import('./pages/LoginPage').then((m) => ({ default: m.LoginPage })));
const LegalOfferPage = lazy(() => import('./pages/LegalOfferPage').then((m) => ({ default: m.LegalOfferPage })));
const LegalPrivacyPage = lazy(() => import('./pages/LegalPrivacyPage').then((m) => ({ default: m.LegalPrivacyPage })));
const NotFoundPage = lazy(() => import('./pages/NotFoundPage').then((m) => ({ default: m.NotFoundPage })));
const PaymentResultPage = lazy(() =>
  import('./pages/PaymentResultPage').then((m) => ({ default: m.PaymentResultPage })),
);
const MiniAppLayout = lazy(() =>
  import('./pages/app/MiniAppLayout').then((m) => ({ default: m.MiniAppLayout })),
);
const MiniHomePage = lazy(() => import('./pages/app/MiniHomePage').then((m) => ({ default: m.MiniHomePage })));
const MiniSchedulePage = lazy(() =>
  import('./pages/app/MiniSchedulePage').then((m) => ({ default: m.MiniSchedulePage })),
);
const MiniSubscriptionsPage = lazy(() =>
  import('./pages/app/MiniSubscriptionsPage').then((m) => ({ default: m.MiniSubscriptionsPage })),
);
const MiniProfilePage = lazy(() =>
  import('./pages/app/MiniProfilePage').then((m) => ({ default: m.MiniProfilePage })),
);

function App() {
  return (
    <BrowserRouter>
      <Suspense fallback={<div className="route-loading">Загрузка…</div>}>
        <Routes>
          <Route element={<Layout />}>
            <Route path="/" element={<HomePage />} />
            <Route path="/runners" element={<CatalogPage />} />
            <Route path="/news" element={<NewsPage />} />
            <Route path="/schedule" element={<SchedulePage />} />
            <Route path="/merch" element={<MerchPage />} />
            <Route path="/cart" element={<CartPage />} />
            <Route path="/me" element={<ProfilePage />} />
            <Route path="/survey" element={<SurveyPage />} />
            <Route path="/auth/telegram" element={<LoginPage />} />
            <Route path="/legal/offer" element={<LegalOfferPage />} />
            <Route path="/legal/privacy" element={<LegalPrivacyPage />} />
            <Route path="/payment/success" element={<PaymentResultPage status="success" />} />
            <Route path="/payment/fail" element={<PaymentResultPage status="fail" />} />
            <Route path="*" element={<NotFoundPage />} />
          </Route>
          <Route path="/app" element={<MiniAppLayout />}>
            <Route index element={<MiniHomePage />} />
            <Route path="schedule" element={<MiniSchedulePage />} />
            <Route path="subscriptions" element={<MiniSubscriptionsPage />} />
            <Route path="profile" element={<MiniProfilePage />} />
          </Route>
        </Routes>
      </Suspense>
    </BrowserRouter>
  );
}

export default App;
