import { ScheduleBoard } from '../components/schedule/ScheduleBoard';

export function SchedulePage() {
  return (
    <>
      <section className="page-hero">
        <div className="hero-sl speedlines" />
        <div className="wrap">
          <div className="eb">Расписание</div>
          <h1 className="d2">Тренировки недели</h1>
        </div>
      </section>
      <section className="sec">
        <div className="wrap">
          <ScheduleBoard />
        </div>
      </section>
    </>
  );
}
