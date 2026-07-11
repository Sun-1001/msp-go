import React, { useEffect, useId, useLayoutEffect, useRef, useState } from 'react';
import { Link, useLocation } from 'react-router-dom';
import { motion } from 'framer-motion';
import { MoreHorizontal } from 'lucide-react';
import { cn } from '@/libs/utils/cn';
import { animationCombos, navIndicatorVariants } from '@/libs/animations';
import type { NavItem } from '@/modules/auth/constants/navigationConfig';

const NAV_ITEM_GAP = 4;
const NAV_ITEM_CLASSES = 'relative flex shrink-0 items-center whitespace-nowrap rounded-md px-3 py-2 text-sm font-medium';

interface ResponsiveNavigationProps {
  items: NavItem[];
  isTeacher: boolean;
}

interface NavigationItemLinkProps {
  item: NavItem;
  isActive: boolean;
  isTeacher: boolean;
  measurement?: boolean;
  onClick?: () => void;
}

interface OverflowNavigationMenuProps {
  items: NavItem[];
  pathname: string;
  isTeacher: boolean;
}

interface OverflowTriggerProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  isActive: boolean;
  isTeacher: boolean;
  measurement?: boolean;
}

function getVisibleItemCount(
  availableWidth: number,
  itemWidths: number[],
  overflowButtonWidth: number
): number {
  if (itemWidths.length === 0) return 0;
  if (itemWidths.some((width) => width <= 0) || overflowButtonWidth <= 0) {
    return itemWidths.length;
  }

  const fullWidth = itemWidths.reduce((total, width) => total + width, 0)
    + NAV_ITEM_GAP * (itemWidths.length - 1);
  if (fullWidth <= availableWidth + 0.5) return itemWidths.length;
  if (availableWidth < overflowButtonWidth) return 0;

  let visibleCount = 0;
  let usedWidth = overflowButtonWidth;

  for (const itemWidth of itemWidths) {
    const nextWidth = usedWidth + NAV_ITEM_GAP + itemWidth;
    if (nextWidth > availableWidth + 0.5) break;

    visibleCount += 1;
    usedWidth = nextWidth;
  }

  return visibleCount;
}

function isNavigationItemActive(pathname: string, href: string): boolean {
  return pathname.startsWith(href);
}

function NavigationItemLink({
  item,
  isActive,
  isTeacher,
  measurement = false,
  onClick,
}: NavigationItemLinkProps) {
  const Icon = item.icon;

  return (
    <Link
      to={item.href}
      aria-current={!measurement && isActive ? 'page' : undefined}
      aria-hidden={measurement ? true : undefined}
      tabIndex={measurement ? -1 : undefined}
      data-nav-item-measure={measurement ? '' : undefined}
      onClick={onClick}
      className={cn(
        NAV_ITEM_CLASSES,
        animationCombos.navItemHover,
        isActive
          ? isTeacher
            ? 'bg-emerald-50 text-emerald-600 dark:bg-emerald-950/50 dark:text-emerald-400'
            : 'bg-primary-50 text-primary-600 dark:bg-primary-950/50 dark:text-primary-400'
          : 'text-surface-600 hover:bg-surface-50 hover:text-primary-600 dark:text-surface-400 dark:hover:bg-surface-800 dark:hover:text-primary-400'
      )}
    >
      <Icon className={cn('mr-2 h-4 w-4 transition-transform duration-200', isActive && 'scale-110')} />
      {item.label}
      {!measurement && (
        <motion.span
          className={cn(
            'absolute inset-x-0 bottom-0 h-0.5 rounded-full',
            isTeacher
              ? 'bg-emerald-600 dark:bg-emerald-400'
              : 'bg-primary-600 dark:bg-primary-400'
          )}
          initial="inactive"
          animate={isActive ? 'active' : 'inactive'}
          variants={navIndicatorVariants}
        />
      )}
    </Link>
  );
}

const OverflowTrigger = React.forwardRef<HTMLButtonElement, OverflowTriggerProps>(
  function OverflowTrigger({
    isActive,
    isTeacher,
    measurement = false,
    ...buttonProps
  }, ref) {
    return (
      <button
        {...buttonProps}
        ref={ref}
        type="button"
        tabIndex={measurement ? -1 : buttonProps.tabIndex}
        aria-hidden={measurement ? true : buttonProps['aria-hidden']}
        data-nav-overflow-measure={measurement ? '' : undefined}
        className={cn(
          'relative flex h-9 w-9 shrink-0 items-center justify-center rounded-md',
          animationCombos.navItemHover,
          isActive
            ? isTeacher
              ? 'bg-emerald-50 text-emerald-600 dark:bg-emerald-950/50 dark:text-emerald-400'
              : 'bg-primary-50 text-primary-600 dark:bg-primary-950/50 dark:text-primary-400'
            : 'text-surface-600 hover:bg-surface-50 hover:text-primary-600 dark:text-surface-400 dark:hover:bg-surface-800 dark:hover:text-primary-400'
        )}
      >
        <MoreHorizontal className="h-5 w-5" />
        {!measurement && isActive && (
          <span
            className={cn(
              'absolute inset-x-0 bottom-0 h-0.5 rounded-full',
              isTeacher
                ? 'bg-emerald-600 dark:bg-emerald-400'
                : 'bg-primary-600 dark:bg-primary-400'
            )}
          />
        )}
      </button>
    );
  }
);

function OverflowNavigationMenu({ items, pathname, isTeacher }: OverflowNavigationMenuProps) {
  const [isOpen, setIsOpen] = useState(false);
  const containerRef = useRef<HTMLDivElement | null>(null);
  const triggerRef = useRef<HTMLButtonElement | null>(null);
  const menuId = useId();
  const activeItem = items.find((item) => isNavigationItemActive(pathname, item.href));

  useEffect(() => {
    if (!isOpen) return;

    const handlePointerDown = (event: PointerEvent) => {
      if (!containerRef.current?.contains(event.target as Node)) {
        setIsOpen(false);
      }
    };
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key !== 'Escape') return;

      setIsOpen(false);
      triggerRef.current?.focus();
    };

    document.addEventListener('pointerdown', handlePointerDown);
    document.addEventListener('keydown', handleKeyDown);
    return () => {
      document.removeEventListener('pointerdown', handlePointerDown);
      document.removeEventListener('keydown', handleKeyDown);
    };
  }, [isOpen]);

  return (
    <div ref={containerRef} className="relative shrink-0">
      <OverflowTrigger
        ref={triggerRef}
        isActive={activeItem !== undefined}
        isTeacher={isTeacher}
        title="更多导航"
        aria-label={activeItem ? `更多导航，当前页面：${activeItem.label}` : '更多导航'}
        aria-controls={isOpen ? menuId : undefined}
        aria-expanded={isOpen}
        onClick={() => setIsOpen((open) => !open)}
      />

      {isOpen && (
        <div
          id={menuId}
          role="group"
          aria-label="更多导航"
          className="absolute right-0 top-full z-50 mt-2 w-44 overflow-hidden rounded-md border border-surface-200 bg-white py-1 shadow-lg dark:border-surface-700 dark:bg-surface-800"
        >
          {items.map((item) => {
            const isActive = isNavigationItemActive(pathname, item.href);
            const Icon = item.icon;

            return (
              <Link
                key={item.href}
                to={item.href}
                aria-current={isActive ? 'page' : undefined}
                onClick={() => setIsOpen(false)}
                className={cn(
                  'flex w-full items-center whitespace-nowrap px-3 py-2.5 text-sm font-medium transition-colors',
                  isActive
                    ? isTeacher
                      ? 'bg-emerald-50 text-emerald-700 dark:bg-emerald-950/50 dark:text-emerald-300'
                      : 'bg-primary-50 text-primary-700 dark:bg-primary-950/50 dark:text-primary-300'
                    : 'text-surface-700 hover:bg-surface-50 hover:text-primary-600 dark:text-surface-300 dark:hover:bg-surface-700 dark:hover:text-primary-400'
                )}
              >
                <Icon className={cn('mr-2 h-4 w-4', isActive && 'scale-110')} />
                {item.label}
              </Link>
            );
          })}
        </div>
      )}
    </div>
  );
}

export const ResponsiveNavigation: React.FC<ResponsiveNavigationProps> = ({ items, isTeacher }) => {
  const { pathname } = useLocation();
  const containerRef = useRef<HTMLDivElement | null>(null);
  const measurementRef = useRef<HTMLDivElement | null>(null);
  const [visibleItemCount, setVisibleItemCount] = useState(items.length);
  const itemSignature = items.map((item) => item.href).join('|');
  const itemCount = items.length;

  useLayoutEffect(() => {
    const container = containerRef.current;
    const measurement = measurementRef.current;
    if (!container || !measurement) return;

    const measure = () => {
      const measuredItems = Array.from(
        measurement.querySelectorAll<HTMLElement>('[data-nav-item-measure]')
      );
      const overflowButton = measurement.querySelector<HTMLElement>('[data-nav-overflow-measure]');
      if (measuredItems.length !== itemCount || !overflowButton) return;

      const itemWidths = measuredItems.map((item) => item.getBoundingClientRect().width);
      const nextVisibleItemCount = getVisibleItemCount(
        container.clientWidth,
        itemWidths,
        overflowButton.getBoundingClientRect().width
      );

      setVisibleItemCount((currentCount) => (
        currentCount === nextVisibleItemCount ? currentCount : nextVisibleItemCount
      ));
    };

    measure();

    if (typeof ResizeObserver === 'undefined') {
      window.addEventListener('resize', measure);
      return () => window.removeEventListener('resize', measure);
    }

    const resizeObserver = new ResizeObserver(measure);
    resizeObserver.observe(container);
    resizeObserver.observe(measurement);
    return () => resizeObserver.disconnect();
  }, [itemCount, itemSignature]);

  const clampedVisibleItemCount = Math.min(visibleItemCount, items.length);
  const visibleItems = items.slice(0, clampedVisibleItemCount);
  const overflowItems = items.slice(clampedVisibleItemCount);

  return (
    <div ref={containerRef} className="relative hidden min-w-0 flex-1 items-center justify-end md:flex">
      <nav aria-label="主导航" className="flex min-w-0 items-center gap-1">
        {visibleItems.map((item) => (
          <NavigationItemLink
            key={item.href}
            item={item}
            isActive={isNavigationItemActive(pathname, item.href)}
            isTeacher={isTeacher}
          />
        ))}
        {overflowItems.length > 0 && (
          <OverflowNavigationMenu
            items={overflowItems}
            pathname={pathname}
            isTeacher={isTeacher}
          />
        )}
      </nav>

      <div
        ref={measurementRef}
        aria-hidden="true"
        className="invisible pointer-events-none absolute left-0 top-0 flex w-max items-center gap-1"
      >
        {items.map((item) => (
          <NavigationItemLink
            key={item.href}
            item={item}
            isActive={false}
            isTeacher={isTeacher}
            measurement
          />
        ))}
        <OverflowTrigger isActive={false} isTeacher={isTeacher} measurement />
      </div>
    </div>
  );
};
