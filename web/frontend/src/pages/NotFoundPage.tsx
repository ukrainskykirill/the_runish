import { Link } from 'react-router-dom';

export function NotFoundPage() {
  return (
    <section className="sec">
      <div className="wrap result-page">
        <div className="eb">404</div>
        <h1 className="d2">Страница не найдена</h1>
        <p>Возможно, ссылка устарела или была введена с ошибкой.</p>
        <div className="hero-actions">
          <Link className="btn btn-primary" to="/">
            На главную
          </Link>
        </div>
      </div>
    </section>
  );
}
