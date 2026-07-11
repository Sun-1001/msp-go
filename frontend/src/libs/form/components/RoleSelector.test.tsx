import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { BookOpen, GraduationCap } from 'lucide-react';
import { describe, expect, it, vi } from 'vitest';
import { RoleSelector, type RoleOption } from './RoleSelector';

type TestRole = 'student' | 'teacher';

const options: RoleOption<TestRole>[] = [
  {
    value: 'student',
    label: '学生',
    description: '进入学习中心',
    icon: GraduationCap,
    gradient: 'from-primary-500 to-secondary-500',
    bgGradient: 'from-primary-50 to-secondary-50',
    borderColor: 'border-primary-500',
    textColor: 'text-primary-600',
  },
  {
    value: 'teacher',
    label: '教师',
    description: '进入教学管理',
    icon: BookOpen,
    gradient: 'from-emerald-500 to-teal-500',
    bgGradient: 'from-emerald-50 to-teal-50',
    borderColor: 'border-emerald-500',
    textColor: 'text-emerald-600',
  },
];

describe('RoleSelector', () => {
  it('renders the compact variant and reports role changes', async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();

    render(
      <RoleSelector
        options={options}
        value="student"
        onChange={onChange}
        label="选择身份"
        variant="compact"
      />
    );

    const studentButton = screen.getByRole('button', { name: '学生 进入学习中心' });
    expect(studentButton).toHaveClass('min-h-16');
    expect(studentButton).toHaveAttribute('aria-pressed', 'true');
    expect(screen.getByRole('group', { name: '选择身份' })).toBeInTheDocument();

    const teacherButton = screen.getByRole('button', { name: '教师 进入教学管理' });
    expect(teacherButton).toHaveAttribute('aria-pressed', 'false');

    await user.click(teacherButton);
    expect(onChange).toHaveBeenCalledWith('teacher');
  });

  it('does not report changes while disabled', async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();

    render(
      <RoleSelector options={options} value="student" onChange={onChange} disabled />
    );

    await user.click(screen.getByRole('button', { name: '教师 进入教学管理' }));
    expect(onChange).not.toHaveBeenCalled();
  });
});
