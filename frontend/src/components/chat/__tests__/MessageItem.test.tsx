import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { MessageItem } from '../MessageItem';

describe('MessageItem attachments', () => {
  it('renders only safe uploaded image attachments', () => {
    render(
      <MessageItem
        id="message-1"
        role="student"
        content="带图片"
        modeName="chat"
        attachments={[
          '/uploads/images/file.png',
          'https://example.com/image.png',
          '/uploads/documents/file.pdf',
          '/uploads/images/../secret.png',
        ]}
      />
    );

    const image = screen.getByRole('img', { name: '附件 1' });
    expect(image).toHaveAttribute('src', 'http://localhost:3000/uploads/images/file.png');

    const link = screen.getByRole('link');
    expect(link).toHaveAttribute('href', 'http://localhost:3000/uploads/images/file.png');
    expect(link).toHaveAttribute('rel', 'noopener noreferrer');
    expect(screen.queryByAltText('附件 2')).not.toBeInTheDocument();
  });
});
