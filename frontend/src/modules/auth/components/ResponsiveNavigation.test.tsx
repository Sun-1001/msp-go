import { act, fireEvent, render, screen, within } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { NavItem } from '@/modules/auth/constants/navigationConfig';
import { ResponsiveNavigation } from './ResponsiveNavigation';

const TestIcon = ({ className }: { className?: string }) => (
  <svg aria-hidden="true" className={className} />
);

const navItems: NavItem[] = [
  { label: '我的班级', href: '/my-class', icon: TestIcon },
  { label: '智能刷题', href: '/exercise', icon: TestIcon },
  { label: '资源中心', href: '/resources', icon: TestIcon },
];

let resizeObserverCallback: ResizeObserverCallback | undefined;

class ResizeObserverMock {
  constructor(callback: ResizeObserverCallback) {
    resizeObserverCallback = callback;
  }

  observe = vi.fn();
  unobserve = vi.fn();
  disconnect = vi.fn();
}

function createRect(width: number): DOMRect {
  return {
    x: 0,
    y: 0,
    width,
    height: 36,
    top: 0,
    right: width,
    bottom: 36,
    left: 0,
    toJSON: () => ({}),
  };
}

function configureMeasurements(itemWidths: number[], overflowButtonWidth: number) {
  const navigation = screen.getByRole('navigation', { name: '主导航' });
  const container = navigation.parentElement as HTMLDivElement;
  const measuredItems = Array.from(
    document.querySelectorAll<HTMLElement>('[data-nav-item-measure]')
  );
  const measuredOverflowButton = document.querySelector<HTMLElement>(
    '[data-nav-overflow-measure]'
  ) as HTMLElement;
  let availableWidth = 0;

  Object.defineProperty(container, 'clientWidth', {
    configurable: true,
    get: () => availableWidth,
  });
  measuredItems.forEach((item, index) => {
    Object.defineProperty(item, 'getBoundingClientRect', {
      configurable: true,
      value: () => createRect(itemWidths[index] ?? 0),
    });
  });
  Object.defineProperty(measuredOverflowButton, 'getBoundingClientRect', {
    configurable: true,
    value: () => createRect(overflowButtonWidth),
  });

  return (width: number) => {
    availableWidth = width;
    act(() => {
      resizeObserverCallback?.([], {} as ResizeObserver);
    });
  };
}

function renderNavigation(initialPath = '/my-class') {
  render(
    <MemoryRouter initialEntries={[initialPath]}>
      <ResponsiveNavigation items={navItems} isTeacher={false} />
    </MemoryRouter>
  );

  return configureMeasurements([80, 80, 80], 36);
}

describe('ResponsiveNavigation', () => {
  beforeEach(() => {
    resizeObserverCallback = undefined;
    vi.stubGlobal('ResizeObserver', ResizeObserverMock);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it('centers the desktop navigation within the available header space', () => {
    renderNavigation();

    expect(screen.getByRole('navigation', { name: '主导航' }).parentElement)
      .toHaveClass('justify-center');
  });

  it('keeps all items visible when they fit and moves trailing items into overflow when narrowed', () => {
    const resize = renderNavigation('/resources');

    resize(248);
    expect(within(screen.getByRole('navigation', { name: '主导航' })).getAllByRole('link'))
      .toHaveLength(3);
    expect(screen.queryByRole('button', { name: /更多导航/ })).not.toBeInTheDocument();

    resize(247);
    const overflowButton = screen.getByRole('button', { name: '更多导航，当前页面：资源中心' });
    expect(within(screen.getByRole('navigation', { name: '主导航' })).getAllByRole('link'))
      .toHaveLength(2);
    expect(overflowButton).toHaveAttribute('aria-expanded', 'false');

    fireEvent.click(overflowButton);
    const overflowGroup = screen.getByRole('group', { name: '更多导航' });
    expect(within(overflowGroup).getByRole('link', { name: '资源中心' }))
      .toHaveAttribute('aria-current', 'page');
  });

  it('preserves item order at narrow boundaries and restores every item after expansion', () => {
    const resize = renderNavigation();

    resize(120);
    expect(
      within(screen.getByRole('navigation', { name: '主导航' }))
        .getAllByRole('link')
        .map((link) => link.textContent)
    ).toEqual(['我的班级']);

    fireEvent.click(screen.getByRole('button', { name: '更多导航' }));
    expect(
      within(screen.getByRole('group', { name: '更多导航' }))
        .getAllByRole('link')
        .map((link) => link.textContent)
    ).toEqual(['智能刷题', '资源中心']);

    fireEvent.click(screen.getByRole('button', { name: '更多导航' }));
    resize(119);
    expect(within(screen.getByRole('navigation', { name: '主导航' })).queryAllByRole('link'))
      .toHaveLength(0);

    resize(248);
    expect(
      within(screen.getByRole('navigation', { name: '主导航' }))
        .getAllByRole('link')
        .map((link) => link.textContent)
    ).toEqual(['我的班级', '智能刷题', '资源中心']);
    expect(screen.queryByRole('button', { name: /更多导航/ })).not.toBeInTheDocument();
  });

  it('closes the overflow menu on Escape, outside interaction, and item selection', () => {
    const resize = renderNavigation('/resources');
    resize(204);

    const overflowButton = screen.getByRole('button', { name: /更多导航/ });
    fireEvent.click(overflowButton);
    fireEvent.keyDown(document, { key: 'Escape' });
    expect(overflowButton).toHaveAttribute('aria-expanded', 'false');
    expect(overflowButton).toHaveFocus();

    fireEvent.click(overflowButton);
    fireEvent.pointerDown(document.body);
    expect(overflowButton).toHaveAttribute('aria-expanded', 'false');

    fireEvent.click(overflowButton);
    fireEvent.click(within(screen.getByRole('group', { name: '更多导航' })).getByRole('link'));
    expect(screen.queryByRole('group', { name: '更多导航' })).not.toBeInTheDocument();
  });
});
