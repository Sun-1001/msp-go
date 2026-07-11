import { act, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

const motionValueState = vi.hoisted(() => ({
  reduceMotion: false,
  values: [] as Array<import('framer-motion').MotionValue<number>>,
}));

vi.mock('framer-motion', async (importOriginal) => {
  const actual = await importOriginal<typeof import('framer-motion')>();
  const React = await import('react');

  return {
    ...actual,
    useMotionValue: (initial: number) => {
      const valueRef = React.useRef<import('framer-motion').MotionValue<number> | null>(null);

      if (valueRef.current === null) {
        valueRef.current = actual.motionValue(initial);
        motionValueState.values.push(valueRef.current);
      }

      return valueRef.current;
    },
    useSpring: (source: import('framer-motion').MotionValue<number>) => source,
    useReducedMotion: () => motionValueState.reduceMotion,
  };
});

import { AnimatedLoginCharacters } from './AnimatedLoginCharacters';
import {
  MASCOT_MOTION_PROFILES,
  calculateMascotPose,
} from './animatedLoginCharactersMotion';

let animationFrameCallbacks = new Map<number, FrameRequestCallback>();
let nextAnimationFrameId = 1;

function mockBounds(element: Element, left: number, top: number, width = 40, height = 20) {
  vi.spyOn(element, 'getBoundingClientRect').mockReturnValue({
    x: left,
    y: top,
    left,
    top,
    right: left + width,
    bottom: top + height,
    width,
    height,
    toJSON: () => ({}),
  });
}

function mockEyePair(
  group: Element,
  firstCenter: { x: number; y: number },
  secondCenter: { x: number; y: number }
) {
  const eyes = group.querySelectorAll('[data-mascot-eye]');
  mockBounds(eyes[0], firstCenter.x - 10, firstCenter.y - 10, 20, 20);
  mockBounds(eyes[1], secondCenter.x - 10, secondCenter.y - 10, 20, 20);
}

function flushAnimationFrame() {
  act(() => {
    const callbacks = Array.from(animationFrameCallbacks.values());
    animationFrameCallbacks.clear();
    callbacks.forEach((callback) => callback(performance.now()));
  });
}

function movePointer(clientX: number, clientY: number) {
  fireEvent.pointerMove(window, { clientX, clientY });
  flushAnimationFrame();
}

function rawGazeValues() {
  return [
    { x: motionValueState.values[0], y: motionValueState.values[1] },
    { x: motionValueState.values[2], y: motionValueState.values[3] },
    { x: motionValueState.values[4], y: motionValueState.values[5] },
    { x: motionValueState.values[6], y: motionValueState.values[7] },
  ];
}

function rawPoseValues() {
  return Array.from({ length: 4 }, (_, index) => {
    const offset = 8 + index * 6;
    return {
      bodyX: motionValueState.values[offset],
      bodyRotate: motionValueState.values[offset + 1],
      neckScaleY: motionValueState.values[offset + 2],
      headX: motionValueState.values[offset + 3],
      headY: motionValueState.values[offset + 4],
      headRotate: motionValueState.values[offset + 5],
    };
  });
}

describe('AnimatedLoginCharacters', () => {
  beforeEach(() => {
    motionValueState.reduceMotion = false;
    motionValueState.values.length = 0;
    animationFrameCallbacks = new Map();
    nextAnimationFrameId = 1;
    vi.spyOn(window, 'requestAnimationFrame').mockImplementation((callback) => {
      const frameId = nextAnimationFrameId++;
      animationFrameCallbacks.set(frameId, callback);
      return frameId;
    });
    vi.spyOn(window, 'cancelAnimationFrame').mockImplementation((frameId) => {
      animationFrameCallbacks.delete(frameId);
    });
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  it('exposes the gaze mode without adding accessible noise or role badges', () => {
    const { rerender } = render(<AnimatedLoginCharacters avertGaze={false} />);
    const scene = screen.getByTestId('animated-login-characters');

    expect(scene).toHaveAttribute('aria-hidden', 'true');
    expect(scene).toHaveAttribute('data-gaze', 'tracking');
    expect(scene.querySelectorAll('[data-mascot-eyes]')).toHaveLength(4);
    expect(scene.querySelectorAll('[data-mascot-body]')).toHaveLength(4);
    expect(scene.querySelectorAll('[data-mascot-base]')).toHaveLength(4);
    expect(scene.querySelectorAll('[data-mascot-anchor]')).toHaveLength(4);
    expect(scene.querySelectorAll('[data-mascot-blink]')).toHaveLength(4);
    expect(
      scene.querySelector('.lucide-brain-circuit, .lucide-network, .lucide-target, .lucide-trending-up')
    ).not.toBeInTheDocument();

    rerender(<AnimatedLoginCharacters avertGaze />);
    expect(scene).toHaveAttribute('data-gaze', 'averted');
  });

  it('focuses each mascot gaze independently on the same window pointer', () => {
    render(<AnimatedLoginCharacters avertGaze={false} />);
    const eyeGroups = screen.getByTestId('animated-login-characters').querySelectorAll('[data-mascot-eyes]');
    mockBounds(eyeGroups[0], 80, 80);
    mockBounds(eyeGroups[1], 460, 80);
    mockBounds(eyeGroups[2], 80, 460);
    mockBounds(eyeGroups[3], 460, 460);

    movePointer(300, 300);

    const [primary, secondary, emerald, amber] = rawGazeValues();
    expect(primary.x.get()).toBeGreaterThan(0);
    expect(primary.y.get()).toBeGreaterThan(0);
    expect(secondary.x.get()).toBeLessThan(0);
    expect(secondary.y.get()).toBeGreaterThan(0);
    expect(emerald.x.get()).toBeGreaterThan(0);
    expect(emerald.y.get()).toBeLessThan(0);
    expect(amber.x.get()).toBeLessThan(0);
    expect(amber.y.get()).toBeLessThan(0);
  });

  it('increases neck extension and head turn as the pointer gets farther away', () => {
    const bounds = { left: 100, top: 100, width: 40, height: 20 };
    const profile = {
      bodyHeight: 200,
      nearDistance: 80,
      farDistance: 600,
      maxNeckStretch: 0.18,
      maxBodyX: 12,
      maxBodyRotate: 7,
      maxHeadX: 15,
      maxHeadY: 12,
      maxHeadRotate: 8,
      initialBlinkDelay: 1000,
      blinkDelayRange: [3000, 6000] as const,
    };

    const near = calculateMascotPose(bounds, { clientX: 180, clientY: 110 }, profile);
    const medium = calculateMascotPose(bounds, { clientX: 420, clientY: 110 }, profile);
    const far = calculateMascotPose(bounds, { clientX: 900, clientY: 110 }, profile);
    const farLeft = calculateMascotPose(bounds, { clientX: -700, clientY: 110 }, profile);

    expect(near.neckScaleY).toBe(1);
    expect(near.bodyRotate).toBe(0);
    expect(near.headRotate).toBeGreaterThan(0);
    expect(medium.neckScaleY).toBeGreaterThan(near.neckScaleY);
    expect(far.neckScaleY).toBeGreaterThan(medium.neckScaleY);
    expect(far.headX).toBeGreaterThanOrEqual(14);
    expect(far.headRotate).toBeGreaterThanOrEqual(7);
    expect(farLeft.headX).toBeLessThanOrEqual(-14);
    expect(farLeft.headRotate).toBeLessThanOrEqual(-7);
  });

  it('calculates a different body pose for each mascot from the same pointer', () => {
    render(<AnimatedLoginCharacters avertGaze={false} />);
    const scene = screen.getByTestId('animated-login-characters');
    const anchors = scene.querySelectorAll('[data-mascot-anchor]');
    mockBounds(anchors[0], 80, 80);
    mockBounds(anchors[1], 460, 80);
    mockBounds(anchors[2], 80, 460);
    mockBounds(anchors[3], 460, 460);

    movePointer(300, 300);

    const [primary, secondary, emerald, amber] = rawPoseValues();
    expect(primary.bodyX.get()).toBeGreaterThan(0);
    expect(secondary.bodyX.get()).toBeLessThan(0);
    expect(emerald.bodyX.get()).toBeGreaterThan(0);
    expect(amber.bodyX.get()).toBeLessThan(0);
    expect(primary.headY.get()).toBeGreaterThan(0);
    expect(secondary.headY.get()).toBeGreaterThan(0);
    expect(emerald.headY.get()).toBeLessThan(0);
    expect(amber.headY.get()).toBeLessThan(0);
    expect(new Set(rawPoseValues().map(({ neckScaleY }) => neckScaleY.get().toFixed(4))).size).toBeGreaterThan(1);
  });

  it('reaches upward but looks downward without stretching in the opposite direction', () => {
    const bounds = { left: 100, top: 100, width: 40, height: 20 };
    const profile = MASCOT_MOTION_PROFILES.primary;
    const above = calculateMascotPose(bounds, { clientX: 120, clientY: -900 }, profile);
    const below = calculateMascotPose(bounds, { clientX: 120, clientY: 1100 }, profile);
    const belowRight = calculateMascotPose(bounds, { clientX: 820, clientY: 810 }, profile);

    expect(above.neckScaleY).toBeCloseTo(1 + profile.maxNeckStretch, 5);
    expect(above.headY).toBeLessThan(0);
    expect(below.neckScaleY).toBe(1);
    expect(below.headY).toBeGreaterThan(0);
    expect(belowRight.headY).toBeGreaterThan(0);
  });

  it('keeps every production mascot profile visibly distance-driven', () => {
    const bounds = { left: 100, top: 100, width: 40, height: 20 };
    const poses = Object.values(MASCOT_MOTION_PROFILES).map((profile) => {
      const horizontal = calculateMascotPose(bounds, { clientX: 1200, clientY: 110 }, profile);
      const above = calculateMascotPose(bounds, { clientX: 120, clientY: -1000 }, profile);

      expect(Math.abs(horizontal.headRotate)).toBeGreaterThanOrEqual(profile.maxHeadRotate * 0.99);
      expect((above.neckScaleY - 1) * profile.bodyHeight).toBeGreaterThan(28);
      return horizontal;
    });

    expect(new Set(poses.map(({ bodyX }) => bodyX.toFixed(3))).size).toBeGreaterThan(1);
    expect(new Set(poses.map(({ headRotate }) => headRotate.toFixed(3))).size).toBeGreaterThan(1);
  });

  it('uses a fixed gaze radius instead of pointer distance', () => {
    render(<AnimatedLoginCharacters avertGaze={false} />);
    const eyeGroups = screen.getByTestId('animated-login-characters').querySelectorAll('[data-mascot-eyes]');
    mockBounds(eyeGroups[0], 80, 90);
    mockBounds(eyeGroups[1], 180, 90);
    mockBounds(eyeGroups[2], 280, 90);
    mockBounds(eyeGroups[3], 380, 90);

    movePointer(700, 100);

    rawGazeValues().forEach(({ x, y }) => {
      expect(x.get()).toBeCloseTo(5.2, 5);
      expect(y.get()).toBeCloseTo(0, 5);
    });
  });

  it('keeps the global gaze direction accurate while a mascot turns its head', () => {
    render(<AnimatedLoginCharacters avertGaze={false} />);
    const eyeGroups = screen.getByTestId('animated-login-characters').querySelectorAll('[data-mascot-eyes]');
    const angle = Math.PI / 6;
    const halfSeparation = 20;
    mockEyePair(
      eyeGroups[0],
      {
        x: 100 - Math.cos(angle) * halfSeparation,
        y: 100 - Math.sin(angle) * halfSeparation,
      },
      {
        x: 100 + Math.cos(angle) * halfSeparation,
        y: 100 + Math.sin(angle) * halfSeparation,
      }
    );

    movePointer(700, 100);

    const [primary] = rawGazeValues();
    expect(primary.x.get()).toBeCloseTo(5.2 * Math.cos(Math.PI / 6), 5);
    expect(primary.y.get()).toBeCloseTo(-5.2 * Math.sin(Math.PI / 6), 5);
    expect(Math.hypot(primary.x.get(), primary.y.get())).toBeCloseTo(5.2, 5);
  });

  it('averts every gaze while the password is visible and resumes the latest target', () => {
    const { rerender } = render(<AnimatedLoginCharacters avertGaze={false} />);
    const scene = screen.getByTestId('animated-login-characters');
    const eyeGroups = scene.querySelectorAll('[data-mascot-eyes]');
    const anchors = scene.querySelectorAll('[data-mascot-anchor]');
    eyeGroups.forEach((group, index) => mockBounds(group, 80 + index * 120, 100 + index * 60));
    anchors.forEach((anchor, index) => mockBounds(anchor, 80 + index * 120, 100 + index * 60));

    fireEvent.pointerMove(window, { clientX: 700, clientY: 500 });
    rerender(<AnimatedLoginCharacters avertGaze />);

    rawGazeValues().forEach(({ x, y }) => {
      expect(x.get()).toBe(-5.2);
      expect(y.get()).toBe(-1.25);
    });
    rawPoseValues().forEach(({ bodyX, bodyRotate, neckScaleY, headX, headY, headRotate }) => {
      expect(bodyX.get()).toBe(0);
      expect(bodyRotate.get()).toBe(0);
      expect(neckScaleY.get()).toBe(1);
      expect(headX.get()).toBe(0);
      expect(headY.get()).toBe(0);
      expect(headRotate.get()).toBe(0);
    });

    fireEvent.pointerMove(window, { clientX: -600, clientY: -500 });
    rawGazeValues().forEach(({ x, y }) => {
      expect(x.get()).toBe(-5.2);
      expect(y.get()).toBe(-1.25);
    });

    rerender(<AnimatedLoginCharacters avertGaze={false} />);
    const resumedX = rawGazeValues().map(({ x }) => x.get());
    expect(new Set(resumedX.map((value) => value.toFixed(3))).size).toBeGreaterThan(1);
    rawPoseValues().forEach(({ bodyX, neckScaleY, headY }) => {
      expect(bodyX.get()).toBeLessThan(0);
      expect(neckScaleY.get()).toBeGreaterThan(1);
      expect(headY.get()).toBeLessThan(0);
    });
  });

  it('blinks each mascot on an independent random schedule', () => {
    vi.useFakeTimers();
    vi.spyOn(Math, 'random').mockReturnValue(0);
    const { unmount } = render(<AnimatedLoginCharacters avertGaze={false} />);
    const blinks = screen.getByTestId('animated-login-characters').querySelectorAll('[data-mascot-blink]');
    const blinkingIndexes = () =>
      Array.from(blinks)
        .map((blink, index) => (blink.getAttribute('data-blinking') === 'true' ? index : -1))
        .filter((index) => index >= 0);

    expect(blinkingIndexes()).toEqual([]);

    act(() => vi.advanceTimersByTime(1300));
    expect(blinkingIndexes()).toEqual([0]);
    act(() => vi.advanceTimersByTime(150));
    expect(blinkingIndexes()).toEqual([]);

    act(() => vi.advanceTimersByTime(750));
    expect(blinkingIndexes()).toEqual([1]);
    act(() => vi.advanceTimersByTime(150));
    expect(blinkingIndexes()).toEqual([]);

    act(() => vi.advanceTimersByTime(750));
    expect(blinkingIndexes()).toEqual([2]);
    act(() => vi.advanceTimersByTime(150));
    expect(blinkingIndexes()).toEqual([]);

    act(() => vi.advanceTimersByTime(750));
    expect(blinkingIndexes()).toEqual([3]);

    unmount();
  });

  it('keeps gaze changes static when reduced motion is active', () => {
    motionValueState.reduceMotion = true;
    const { rerender } = render(<AnimatedLoginCharacters avertGaze={false} />);

    fireEvent.pointerMove(window, { clientX: 1000, clientY: 800 });
    rawGazeValues().forEach(({ x, y }) => {
      expect(x.get()).toBe(0);
      expect(y.get()).toBe(0);
    });

    rerender(<AnimatedLoginCharacters avertGaze />);
    rawGazeValues().forEach(({ x, y }) => {
      expect(x.get()).toBe(-5.2);
      expect(y.get()).toBe(-1.25);
    });
    expect(
      Array.from(screen.getByTestId('animated-login-characters').querySelectorAll('[data-mascot-blink]')).every(
        (blink) => blink.getAttribute('data-blinking') === 'false'
      )
    ).toBe(true);
  });

  it('does not restore a stale closed-eye state after reduced motion changes', () => {
    vi.useFakeTimers();
    vi.spyOn(Math, 'random').mockReturnValue(0);
    const { rerender } = render(<AnimatedLoginCharacters avertGaze={false} />);
    const primaryBlink = screen
      .getByTestId('animated-login-characters')
      .querySelector('[data-mascot-blink="primary"]');

    act(() => vi.advanceTimersByTime(1300));
    expect(primaryBlink).toHaveAttribute('data-blinking', 'true');

    motionValueState.reduceMotion = true;
    rerender(<AnimatedLoginCharacters avertGaze={false} />);
    expect(primaryBlink).toHaveAttribute('data-blinking', 'false');

    motionValueState.reduceMotion = false;
    rerender(<AnimatedLoginCharacters avertGaze={false} />);
    expect(primaryBlink).toHaveAttribute('data-blinking', 'false');
    act(() => vi.advanceTimersByTime(1299));
    expect(primaryBlink).toHaveAttribute('data-blinking', 'false');
  });

  it('clears every independent blink timer when unmounted', () => {
    vi.useFakeTimers();
    vi.spyOn(Math, 'random').mockReturnValue(0.5);
    const clearTimeout = vi.spyOn(window, 'clearTimeout');
    const { unmount } = render(<AnimatedLoginCharacters avertGaze={false} />);

    unmount();
    act(() => vi.advanceTimersByTime(120_000));

    expect(clearTimeout).toHaveBeenCalledTimes(4);
  });

  it('removes the global pointer listener when unmounted', () => {
    const removeEventListener = vi.spyOn(window, 'removeEventListener');
    const { unmount } = render(<AnimatedLoginCharacters avertGaze={false} />);

    unmount();

    expect(removeEventListener).toHaveBeenCalledWith('pointermove', expect.any(Function));
  });
});
