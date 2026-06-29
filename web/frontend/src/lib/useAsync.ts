import { useEffect, useReducer } from 'react';

interface AsyncState<T> {
  data: T | null;
  loading: boolean;
  error: boolean;
}

type AsyncAction<T> =
  | { type: 'start' }
  | { type: 'success'; data: T }
  | { type: 'error' };

function asyncReducer<T>(state: AsyncState<T>, action: AsyncAction<T>): AsyncState<T> {
  switch (action.type) {
    case 'start':
      return { ...state, loading: true, error: false };
    case 'success':
      return { data: action.data, loading: false, error: false };
    case 'error':
      return { ...state, loading: false, error: true };
  }
}

/** Загружает данные через fn() при монтировании / изменении deps. */
export function useAsync<T>(fn: (signal: AbortSignal) => Promise<T>, deps: unknown[] = []): AsyncState<T> {
  const [state, dispatch] = useReducer(asyncReducer<T>, {
    data: null,
    loading: true,
    error: false,
  });

  useEffect(() => {
    let active = true;
    const controller = new AbortController();
    dispatch({ type: 'start' });
    fn(controller.signal)
      .then((d) => {
        if (active) {
          dispatch({ type: 'success', data: d });
        }
      })
      .catch((err) => {
        if (err instanceof DOMException && err.name === 'AbortError') return;
        if (active) {
          dispatch({ type: 'error' });
        }
      });
    return () => {
      active = false;
      controller.abort();
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, deps);

  return state;
}
