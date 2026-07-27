import { Link } from 'react-router-dom';
import { ScheduleBoard } from './ScheduleBoard';

export function ScheduleCalendar() {
  return (
    <section className="sec" id="schedule">
      <div className="wrap">
        <div className="sec-head between">
          <div>
            <div className="eb">Расписание</div>
            <h2 className="d2">Тренировки</h2>
          </div>
          <Link className="btn btn-ghost btn-sm" to="/schedule">
            Всё расписание
          </Link>
        </div>
        <ScheduleBoard />
      </div>
    </section>
  );
}
