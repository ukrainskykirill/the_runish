import { Outlet } from 'react-router-dom';
import { LoginModal } from '../auth/LoginModal';
import { SurveyNudgeModal } from '../survey/SurveyNudgeModal';
import { Toast } from '../Toast';
import { Footer } from './Footer';
import { Header } from './Header';
import { ScrollToHash } from './ScrollToHash';

export function Layout() {
  return (
    <>
      <ScrollToHash />
      <Header />
      <main>
        <Outlet />
      </main>
      <Footer />
      <Toast />
      <LoginModal />
      <SurveyNudgeModal />
    </>
  );
}
