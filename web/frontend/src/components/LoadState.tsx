interface LoadingGridProps {
  count?: number;
}

export function LoadingGrid({ count = 4 }: LoadingGridProps) {
  return (
    <div className="skeleton-grid" aria-label="Загрузка">
      {Array.from({ length: count }, (_, i) => (
        <div className="skeleton-card" key={i}>
          <span className="sk sk-chip" />
          <span className="sk sk-title" />
          <span className="sk sk-line" />
          <span className="sk sk-line short" />
          <span className="sk sk-price" />
          <span className="sk sk-button" />
        </div>
      ))}
    </div>
  );
}

interface ErrorStateProps {
  message?: string;
}

export function ErrorState({ message = 'Не удалось загрузить данные' }: ErrorStateProps) {
  return <div className="empty-state">{message}</div>;
}
