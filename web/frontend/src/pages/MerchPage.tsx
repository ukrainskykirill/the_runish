import { api } from '../api/client';
import { ErrorState, LoadingGrid } from '../components/LoadState';
import { MerchCard } from '../components/cards/MerchCard';
import { useAsync } from '../lib/useAsync';

export function MerchPage() {
  const { data, loading, error } = useAsync(() => api.merch(), []);
  const merch = data?.merch ?? [];

  return (
    <>
      <section className="page-hero">
        <div className="hero-sl speedlines" />
        <div className="wrap">
          <div className="eb">Мерч</div>
          <h1 className="d2">Носи клуб</h1>
        </div>
      </section>
      <section className="sec">
        <div className="wrap">
          {loading ? (
            <LoadingGrid count={3} />
          ) : error ? (
            <ErrorState message="Не удалось загрузить мерч" />
          ) : merch.length === 0 ? (
            <div className="empty-state">Мерч скоро появится</div>
          ) : (
            <div className="merch-grid">
              {merch.map((item) => (
                <MerchCard key={item.id} item={item} />
              ))}
            </div>
          )}
        </div>
      </section>
    </>
  );
}
