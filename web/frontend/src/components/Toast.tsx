import { CheckIcon } from './icons';
import { useUI } from '../context/UIContext';

export function Toast() {
  const { toastMessage, toastVisible } = useUI();

  return (
    <div className={`toast ${toastVisible ? 'show' : ''}`}>
      <CheckIcon className="i i-sm" />
      <span>{toastMessage}</span>
    </div>
  );
}
