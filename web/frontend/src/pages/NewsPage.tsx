import { useState } from 'react';
import { api } from '../api/client';
import { ErrorState, LoadingGrid } from '../components/LoadState';
import { NewsCard } from '../components/cards/NewsCard';
import { NewsModal } from '../components/news/NewsModal';
import { useAsync } from '../lib/useAsync';

export function NewsPage() {
  const { data, loading, error } = useAsync(() => api.news(), []);
  const news = data?.news ?? [];
  const [selectedNewsId, setSelectedNewsId] = useState<number | null>(null);

  return (
    <>
      <section className="page-hero">
        <div className="hero-sl speedlines" />
        <div className="wrap">
          <div className="eb">Новости</div>
          <h1 className="d2">Что в клубе</h1>
        </div>
      </section>
      <section className="sec">
        <div className="wrap">
          {loading ? (
            <LoadingGrid count={3} />
          ) : error ? (
            <ErrorState message="Не удалось загрузить новости" />
          ) : news.length === 0 ? (
            <div className="empty-state">Новостей пока нет</div>
          ) : (
            <div className="news-grid">
              {news.map((item, i) => (
                <NewsCard key={item.id} news={item} iconIndex={i} onReadMore={setSelectedNewsId} />
              ))}
            </div>
          )}
        </div>
      </section>
      <NewsModal newsId={selectedNewsId} onClose={() => setSelectedNewsId(null)} />
    </>
  );
}
