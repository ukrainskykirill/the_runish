import { ScheduleBoard } from '../components/schedule/ScheduleBoard';
import { TrainingSignup } from '../components/schedule/TrainingSignup';

export function SchedulePage() {
  return (
    <>
      <section className="page-hero">
        <div className="hero-sl speedlines" />
        <div className="wrap">
          <div className="eb">Расписание</div>
          <h1 className="d2">Клубные тренировки</h1>
        </div>
      </section>
      <section className="sec">
        <div className="wrap">
          <ScheduleBoard />
        </div>
      </section>
      <section className="sec alt">
        <div className="wrap">
          <div className="sec-head">
            <div className="eb">Запись</div>
            <h2 className="d2">Записаться на тренировку</h2>
            <p className="sec-lead">
              Выбери занятие на ближайшие недели. Запись доступна при оплаченном взносе и активной
              подписке или разовой тренировке. Напомним в Telegram за сутки до старта.
            </p>
          </div>
          <TrainingSignup />
        </div>
      </section>
    </>
  );
}
