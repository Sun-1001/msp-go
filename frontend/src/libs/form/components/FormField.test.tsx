import { createRef } from 'react';
import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { FormField } from './FormField';

describe('FormField', () => {
  it('preserves input refs and events when a trailing action is present', () => {
    const inputRef = createRef<HTMLInputElement>();
    const onChange = vi.fn();
    const onAction = vi.fn();

    render(
      <FormField
        ref={inputRef}
        label="密码"
        type="password"
        onChange={onChange}
        trailingAction={(
          <button type="button" onClick={onAction} aria-label="显示密码">
            显示
          </button>
        )}
      />
    );

    const input = screen.getByLabelText('密码');
    expect(inputRef.current).toBe(input);
    expect(input).toHaveClass('pr-11');

    fireEvent.change(input, { target: { value: 'secret' } });
    fireEvent.click(screen.getByRole('button', { name: '显示密码' }));

    expect(onChange).toHaveBeenCalledOnce();
    expect(onAction).toHaveBeenCalledOnce();
  });
});
