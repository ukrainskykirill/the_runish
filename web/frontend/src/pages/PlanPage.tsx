import { useEffect, useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { api } from '../api/client';
import type { PlanResponse } from '../api/types';
import { WeekPlanView } from '../components/plan/WeekPlanView';
import { ErrorState } from '../components/LoadState';
import { useAuth } from '../context/AuthContext';
import { useAsync } from '../lib/useAsync';

export function PlanPage() {
  const { user, subscriptions, loading: authLoading } = useAuth();
  const navigate = useNavigate();
  const [params, setParams] = useSearchParams();
  const week = params.get('week') ?? undefined;
  const [hasAccess, setHasAccess] = useState(true);

  const hasSub = subscriptions.length > 0;

  useEffect(() => {
    if (authLoading) return;
    if (!user) navigate('/', { replace: true });
    else if (!hasSub) navigate('/runners', { replace: true });
  }, [authLoading, user, hasSub, navigate]);

  const plan = useAsync<PlanResponse>(
    (signal) =>
      api.plan(week, signal).catch((err) => {
        if (err instanceof Error && 'code' in err && err.code === 'subscription_required') {
          setHasAccess(false);
        }
        throw err;
      }),
    [week],
  );

  if (authLoading || !user || !hasSub) return null;

  const weeks = plan.data?.weeks ?? [];
  const current = plan.data?.week_start ?? '';
  const idx = weeks.indexOf(current);
  // weeks отсортированы от новой к старой, поэтому «предыдущая» — следующий индекс.
  const prevWeek = idx >= 0 && idx + 1 < weeks.length ? weeks[idx + 1] : null;
  const nextWeek = idx > 0 ? weeks[idx - 1] : null;

  return (
    <>
      <section className="page-hero">
        <div className="hero-sl speedlines" />
        <div className="wrap">
          <div className="eb">Подписка</div>
          <h1 className="d2">План тренировок</h1>
        </div>
      </section>

      <section className="sec">
        <div className="wrap">
          {!hasAccess ? (
            <ErrorState message="План доступен только с активной подпиской" />
          ) : plan.loading ? (
            <div className="empty-state">Загрузка…</div>
          ) : plan.error ? (
            <ErrorState />
          ) : !plan.data || plan.data.groups.length === 0 ? (
            <div className="empty-state">План пока не опубликован</div>
          ) : (
            <>
              <div className="plan-weeks">
                <button
                  className="btn btn-ghost btn-sm"
                  disabled={!prevWeek}
                  onClick={() => prevWeek && setParams({ week: prevWeek })}
                >
                  ← Раньше
                </button>
                <span className="nm">{plan.data.label}</span>
                <button
                  className="btn btn-ghost btn-sm"
                  disabled={!nextWeek}
                  onClick={() => nextWeek && setParams({ week: nextWeek })}
                >
                  Позже →
                </button>
              </div>
              <WeekPlanView groups={plan.data.groups} materials={plan.data.materials} />
            </>
          )}
        </div>
      </section>
    </>
  );
}
