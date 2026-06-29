import { Link } from 'react-router-dom';
import type { Training } from '../../api/types';
import { ScheduleBoard } from './ScheduleBoard';

interface ScheduleCalendarProps {
  trainings?: Training[];
}

// ScheduleCalendar — секция «Расписание» на главной (тизер недельной доски).
export function ScheduleCalendar({ trainings }: ScheduleCalendarProps) {
  return (
    <section className="sec" id="schedule">
      <div className="wrap">
        <div className="sec-head between">
          <div>
            <div className="eb">Расписание</div>
            <h2 className="d2">Эта неделя</h2>
          </div>
          <Link className="btn btn-ghost btn-sm" to="/schedule">
            Всё расписание
          </Link>
        </div>
        <ScheduleBoard trainings={trainings} />
      </div>
    </section>
  );
}
