import { api } from '../api/client';
import { EntryFeeNotice } from '../components/EntryFeeNotice';
import { ErrorState, LoadingGrid } from '../components/LoadState';
import { ServiceCard } from '../components/cards/ServiceCard';
import { useCart } from '../context/CartContext';
import { useAsync } from '../lib/useAsync';

export function CatalogPage() {
  const { add } = useCart();
  const { data, loading, error } = useAsync(() => api.catalog(), []);
  const services = data?.services ?? [];

  return (
    <>
      <section className="page-hero">
        <div className="hero-sl speedlines" />
        <div className="wrap">
          <div className="eb">Тренировки и подписки</div>
          <h1 className="d2">Выбери формат</h1>
        </div>
      </section>
      <section className="sec">
        <div className="wrap">
          {loading ? (
            <LoadingGrid />
          ) : error ? (
            <ErrorState message="Не удалось загрузить каталог" />
          ) : services.length === 0 ? (
            <div className="empty-state">Каталог пока пуст</div>
          ) : (
            <>
              <EntryFeeNotice />
              <div className="cat-grid">
                {services.map((service) => (
                  <ServiceCard key={service.id} service={service} onAdd={add} />
                ))}
              </div>
            </>
          )}
        </div>
      </section>
    </>
  );
}
