import { describe, expect, it } from 'vitest';

import { buildDashboardCsvContent, type DashboardExportOptions } from '../dashboardExporter';
import type { DashboardStats, TeacherAnalyticsData } from '@/modules/teacher/types/teacher';

const stats: DashboardStats = {
  total_students: 2,
  active_today: 50,
  avg_completion_rate: 90,
  pending_grading: 1,
};

const options: DashboardExportOptions = {
  format: 'csv',
  timeRangeLabel: '本周',
  sections: {
    overview: true,
    knowledgePoints: true,
    topStudents: true,
    weeklyActivity: true,
  },
};

describe('buildDashboardCsvContent', () => {
  it('escapes spreadsheet formula prefixes in text fields', () => {
    const analytics: TeacherAnalyticsData = {
      overview: {
        total_students: 2,
        avg_completion_rate: 90,
        avg_score: 88,
        avg_study_hours: 3,
      },
      knowledge_points: [
        { concept_id: 'kp-1', name: '=HYPERLINK("https://evil.test")', mastery: 80, student_count: 2 },
      ],
      top_students: [
        { rank: 1, student_id: 'student-1', name: '+Alice', avg_score: 99 },
      ],
      weekly_activity: [
        { date: '2026-07-01', day_label: ' @周三', active_rate: 70 },
      ],
    };

    const csv = buildDashboardCsvContent(stats, analytics, options);

    expect(csv).toContain('"\'=HYPERLINK(""https://evil.test"")",80,2');
    expect(csv).toContain("1,'+Alice,99");
    expect(csv).toContain("2026-07-01,' @周三,70");
  });

  it('keeps regular CSV escaping for commas and quotes', () => {
    const analytics: TeacherAnalyticsData = {
      overview: {
        total_students: 2,
        avg_completion_rate: 90,
        avg_score: 88,
        avg_study_hours: 3,
      },
      knowledge_points: [
        { concept_id: 'kp-1', name: '函数,极限', mastery: 80, student_count: 2 },
        { concept_id: 'kp-2', name: 'He said "ok"', mastery: 75, student_count: 1 },
      ],
      top_students: [],
      weekly_activity: [],
    };

    const csv = buildDashboardCsvContent(stats, analytics, options);

    expect(csv).toContain('"函数,极限",80,2');
    expect(csv).toContain('"He said ""ok""",75,1');
  });
});
