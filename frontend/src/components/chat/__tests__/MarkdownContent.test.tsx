import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { MarkdownContent } from '../MarkdownContent';

describe('MarkdownContent links', () => {
  it('renders safe external links with tabnabbing protection', () => {
    render(<MarkdownContent content="[课程](example.com/course)" />);

    const link = screen.getByRole('link', { name: '课程' });
    expect(link).toHaveAttribute('href', 'https://example.com/course');
    expect(link).toHaveAttribute('target', '_blank');
    expect(link).toHaveAttribute('rel', 'noopener noreferrer');
  });

  it('does not render unsafe markdown links as clickable anchors', () => {
    render(<MarkdownContent content="[坏链接](javascript:alert(1))" />);

    expect(screen.queryByRole('link', { name: '坏链接' })).not.toBeInTheDocument();
    expect(screen.getByText('坏链接')).toBeInTheDocument();
  });
});
