import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { MathText } from '../MathText';

describe('MathText', () => {
  it('renders text HTML as plain text while keeping inline math rendering', () => {
    const { container } = render(
      <MathText>{'<img src=x onerror=alert(1)> 公式 $x^2$ <script>alert(1)</script>'}</MathText>
    );

    expect(container.querySelector('img')).not.toBeInTheDocument();
    expect(container.querySelector('script')).not.toBeInTheDocument();
    expect(screen.getByText(/<img src=x onerror=alert\(1\)> 公式/)).toBeInTheDocument();
    expect(screen.getByText(/<script>alert\(1\)<\/script>/)).toBeInTheDocument();
    expect(container.querySelector('.katex')).toBeInTheDocument();
  });

  it('renders block math in a dedicated block container', () => {
    const { container } = render(<MathText>{'推导：$$\\frac{1}{2}$$完成'}</MathText>);

    const mathBlock = container.querySelector('.math-block');
    expect(mathBlock).toBeInTheDocument();
    expect(mathBlock?.querySelector('.katex-display')).toBeInTheDocument();
    expect(screen.getByText(/推导：/)).toBeInTheDocument();
    expect(screen.getByText(/完成/)).toBeInTheDocument();
  });

  it('keeps malformed math text inert in KaTeX error output', () => {
    const { container } = render(<MathText>{'错误 $\\bad{<img src=x onerror=alert(1)>}$'}</MathText>);

    expect(container.querySelector('img')).not.toBeInTheDocument();
    expect(container.textContent).toContain('<img src=x onerror=alert(1)>');
  });
});
