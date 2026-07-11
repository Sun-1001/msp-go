import { useCallback, useEffect, useMemo, useRef, useState, type RefObject } from 'react';
import {
  motion,
  useMotionValue,
  useReducedMotion,
  useSpring,
  type MotionValue,
} from 'framer-motion';
import { Circle } from 'lucide-react';
import { cn } from '@/libs/utils/cn';
import {
  MASCOT_MOTION_PROFILES,
  NEUTRAL_POSE,
  calculateMascotPose,
  type MascotMotionProfile,
  type MascotPose,
  type PointerPosition,
} from './animatedLoginCharactersMotion';

interface AnimatedLoginCharactersProps {
  avertGaze: boolean;
}

interface EyeProps {
  x: MotionValue<number>;
  y: MotionValue<number>;
}

interface GazeTarget {
  eyesRef: RefObject<HTMLDivElement | null>;
  rawX: MotionValue<number>;
  rawY: MotionValue<number>;
}

interface MascotPoseTarget {
  anchorRef: RefObject<HTMLDivElement | null>;
  profile: MascotMotionProfile;
  rawBodyX: MotionValue<number>;
  rawBodyRotate: MotionValue<number>;
  rawNeckScaleY: MotionValue<number>;
  rawHeadX: MotionValue<number>;
  rawHeadY: MotionValue<number>;
  rawHeadRotate: MotionValue<number>;
}

interface MascotProps {
  className: string;
  baseClassName: string;
  bodyClassName: string;
  eyeClassName: string;
  eyeGapClassName: string;
  eyeWidthClassName: string;
  gazeId: string;
  eyesRef: RefObject<HTMLDivElement | null>;
  rawX: MotionValue<number>;
  rawY: MotionValue<number>;
  springX: MotionValue<number>;
  springY: MotionValue<number>;
  anchorRef: RefObject<HTMLDivElement | null>;
  motionProfile: MascotMotionProfile;
  rawBodyX: MotionValue<number>;
  rawBodyRotate: MotionValue<number>;
  rawNeckScaleY: MotionValue<number>;
  rawHeadX: MotionValue<number>;
  rawHeadY: MotionValue<number>;
  rawHeadRotate: MotionValue<number>;
  springBodyX: MotionValue<number>;
  springBodyRotate: MotionValue<number>;
  springNeckScaleY: MotionValue<number>;
  springHeadX: MotionValue<number>;
  springHeadY: MotionValue<number>;
  springHeadRotate: MotionValue<number>;
  onPoseUpdate: () => void;
  avertGaze: boolean;
  reduceMotion: boolean;
}

const MAX_GAZE_OFFSET = 5.2;
const AVERTED_GAZE = { x: -MAX_GAZE_OFFSET, y: -1.25 };
const BLINK_DURATION = 150;
const BLINK_JITTER_RANGE = [0, 650] as const;
const POSE_SPRING = { stiffness: 185, damping: 23, mass: 0.7 };
const HEAD_SPRING = { stiffness: 230, damping: 24, mass: 0.62 };

function useGazeTarget() {
  const eyesRef = useRef<HTMLDivElement>(null);
  const rawX = useMotionValue(0);
  const rawY = useMotionValue(0);
  const springX = useSpring(rawX, { stiffness: 420, damping: 32 });
  const springY = useSpring(rawY, { stiffness: 420, damping: 32 });

  return [eyesRef, rawX, rawY, springX, springY] as const;
}

function useMascotPose() {
  const anchorRef = useRef<HTMLDivElement>(null);
  const rawBodyX = useMotionValue(0);
  const rawBodyRotate = useMotionValue(0);
  const rawNeckScaleY = useMotionValue(1);
  const rawHeadX = useMotionValue(0);
  const rawHeadY = useMotionValue(0);
  const rawHeadRotate = useMotionValue(0);
  const bodyX = useSpring(rawBodyX, POSE_SPRING);
  const bodyRotate = useSpring(rawBodyRotate, POSE_SPRING);
  const neckScaleY = useSpring(rawNeckScaleY, POSE_SPRING);
  const headX = useSpring(rawHeadX, HEAD_SPRING);
  const headY = useSpring(rawHeadY, POSE_SPRING);
  const headRotate = useSpring(rawHeadRotate, HEAD_SPRING);
  return [
    anchorRef,
    rawBodyX,
    rawBodyRotate,
    rawNeckScaleY,
    rawHeadX,
    rawHeadY,
    rawHeadRotate,
    bodyX,
    bodyRotate,
    neckScaleY,
    headX,
    headY,
    headRotate,
  ] as const;
}

function randomBetween([minimum, maximum]: readonly [number, number]) {
  return Math.round(minimum + Math.random() * (maximum - minimum));
}

function useIndependentBlink(profile: MascotMotionProfile, reduceMotion: boolean) {
  const [blinkState, setBlinkState] = useState(() => ({ reduceMotion, isBlinking: false }));

  if (blinkState.reduceMotion !== reduceMotion) {
    setBlinkState({ reduceMotion, isBlinking: false });
  }

  useEffect(() => {
    if (reduceMotion) return;

    let disposed = false;
    let blinkTimer: ReturnType<typeof setTimeout> | undefined;
    let reopenTimer: ReturnType<typeof setTimeout> | undefined;

    const scheduleBlink = (delay: number) => {
      blinkTimer = setTimeout(() => {
        if (disposed) return;

        setBlinkState({ reduceMotion: false, isBlinking: true });
        reopenTimer = setTimeout(() => {
          if (disposed) return;

          setBlinkState({ reduceMotion: false, isBlinking: false });
          scheduleBlink(randomBetween(profile.blinkDelayRange));
        }, BLINK_DURATION);
      }, delay);
    };

    scheduleBlink(profile.initialBlinkDelay + randomBetween(BLINK_JITTER_RANGE));

    return () => {
      disposed = true;
      if (blinkTimer !== undefined) clearTimeout(blinkTimer);
      if (reopenTimer !== undefined) clearTimeout(reopenTimer);
    };
  }, [profile, reduceMotion]);

  return !reduceMotion && blinkState.reduceMotion === reduceMotion && blinkState.isBlinking;
}

function setGaze(target: GazeTarget, x: number, y: number) {
  target.rawX.set(x);
  target.rawY.set(y);
}

function getBoundsCenter(bounds: DOMRect) {
  return {
    x: bounds.left + bounds.width / 2,
    y: bounds.top + bounds.height / 2,
  };
}

function getEyeGeometry(eyes: Element) {
  const eyeElements = eyes.querySelectorAll('[data-mascot-eye]');
  if (eyeElements.length >= 2) {
    const first = getBoundsCenter(eyeElements[0].getBoundingClientRect());
    const second = getBoundsCenter(eyeElements[1].getBoundingClientRect());
    const separation = Math.hypot(second.x - first.x, second.y - first.y);

    if (separation >= 0.5) {
      return {
        centerX: (first.x + second.x) / 2,
        centerY: (first.y + second.y) / 2,
        rotation: Math.atan2(second.y - first.y, second.x - first.x),
      };
    }
  }

  const bounds = eyes.getBoundingClientRect();
  return {
    centerX: bounds.left + bounds.width / 2,
    centerY: bounds.top + bounds.height / 2,
    rotation: 0,
  };
}

function calculateGaze(target: GazeTarget, clientX: number, clientY: number) {
  const eyes = target.eyesRef.current;
  if (!eyes) return null;

  const geometry = getEyeGeometry(eyes);
  const deltaX = clientX - geometry.centerX;
  const deltaY = clientY - geometry.centerY;
  const distance = Math.hypot(deltaX, deltaY);

  if (distance < 0.5) {
    return { target, x: 0, y: 0 };
  }

  const directionX = deltaX / distance;
  const directionY = deltaY / distance;
  const localDirectionX =
    Math.cos(geometry.rotation) * directionX + Math.sin(geometry.rotation) * directionY;
  const localDirectionY =
    -Math.sin(geometry.rotation) * directionX + Math.cos(geometry.rotation) * directionY;

  return {
    target,
    x: localDirectionX * MAX_GAZE_OFFSET,
    y: localDirectionY * MAX_GAZE_OFFSET,
  };
}

function calculatePose(target: MascotPoseTarget, pointer: PointerPosition) {
  const anchor = target.anchorRef.current;
  if (!anchor) return null;

  return {
    target,
    pose: calculateMascotPose(anchor.getBoundingClientRect(), pointer, target.profile),
  };
}

function setPose(target: MascotPoseTarget, pose: MascotPose) {
  target.rawBodyX.set(pose.bodyX);
  target.rawBodyRotate.set(pose.bodyRotate);
  target.rawNeckScaleY.set(pose.neckScaleY);
  target.rawHeadX.set(pose.headX);
  target.rawHeadY.set(pose.headY);
  target.rawHeadRotate.set(pose.headRotate);
}

function Eye({ x, y }: EyeProps) {
  return (
    <span data-mascot-eye className="relative block h-5 w-5 shrink-0 drop-shadow-sm">
      <Circle className="absolute inset-0 h-5 w-5 fill-white stroke-white" aria-hidden="true" />
      <motion.span
        data-mascot-pupil
        className="absolute left-1/2 top-1/2 h-2 w-2 -translate-x-1/2 -translate-y-1/2"
        style={{ x, y }}
      >
        <Circle className="h-2 w-2 fill-surface-950 stroke-surface-950" aria-hidden="true" />
      </motion.span>
    </span>
  );
}

function Mascot({
  className,
  baseClassName,
  bodyClassName,
  eyeClassName,
  eyeGapClassName,
  eyeWidthClassName,
  gazeId,
  eyesRef,
  rawX,
  rawY,
  springX,
  springY,
  anchorRef,
  motionProfile,
  rawBodyX,
  rawBodyRotate,
  rawNeckScaleY,
  rawHeadX,
  rawHeadY,
  rawHeadRotate,
  springBodyX,
  springBodyRotate,
  springNeckScaleY,
  springHeadX,
  springHeadY,
  springHeadRotate,
  onPoseUpdate,
  avertGaze,
  reduceMotion,
}: MascotProps) {
  const eyeX = reduceMotion ? rawX : springX;
  const eyeY = reduceMotion ? rawY : springY;
  const bodyX = reduceMotion ? rawBodyX : springBodyX;
  const bodyRotate = reduceMotion ? rawBodyRotate : springBodyRotate;
  const neckScaleY = reduceMotion ? rawNeckScaleY : springNeckScaleY;
  const headX = reduceMotion ? rawHeadX : springHeadX;
  const headY = reduceMotion ? rawHeadY : springHeadY;
  const headRotate = reduceMotion ? rawHeadRotate : springHeadRotate;
  const isBlinking = useIndependentBlink(motionProfile, reduceMotion);

  return (
    <motion.div
      className={cn('absolute origin-bottom overflow-visible', className)}
      animate={avertGaze ? { x: -3, rotate: -2 } : { x: 0, rotate: 0 }}
      transition={reduceMotion ? { duration: 0 } : { type: 'spring', stiffness: 240, damping: 24 }}
    >
      <motion.div
        data-mascot-pose={gazeId}
        className="absolute inset-0 origin-bottom"
        style={{ x: bodyX, rotate: bodyRotate }}
        onUpdate={onPoseUpdate}
      >
        <div
          data-mascot-base={gazeId}
          className={cn('pointer-events-none absolute inset-x-0 -bottom-3 h-4', baseClassName)}
          aria-hidden="true"
        />
        <motion.div
          data-mascot-body={gazeId}
          className={cn(
            'absolute inset-0 origin-bottom overflow-hidden border-x border-t border-white/30 shadow-[0_24px_48px_rgba(15,23,42,0.16)]',
            bodyClassName
          )}
          style={{ scaleY: neckScaleY }}
          onUpdate={onPoseUpdate}
        >
          <div className="absolute inset-x-0 top-0 h-px bg-white/70" aria-hidden="true" />
        </motion.div>
        <div className={cn('absolute', eyeClassName)}>
          <motion.div
            ref={eyesRef}
            data-mascot-eyes={gazeId}
            className="origin-center"
            style={{ x: headX, y: headY, rotate: headRotate }}
            onUpdate={onPoseUpdate}
          >
            <motion.div
              data-mascot-blink={gazeId}
              data-blinking={isBlinking ? 'true' : 'false'}
              className={cn('flex origin-center items-center', eyeGapClassName)}
              animate={{ scaleY: isBlinking ? 0.06 : 1 }}
              transition={
                reduceMotion
                  ? { duration: 0 }
                  : { duration: isBlinking ? 0.055 : 0.09, ease: 'easeInOut' }
              }
            >
              <Eye x={eyeX} y={eyeY} />
              <Eye x={eyeX} y={eyeY} />
            </motion.div>
          </motion.div>
        </div>
      </motion.div>
      <div
        ref={anchorRef}
        data-mascot-anchor={gazeId}
        className={cn('pointer-events-none absolute h-5', eyeClassName, eyeWidthClassName)}
      />
    </motion.div>
  );
}

export function AnimatedLoginCharacters({ avertGaze }: AnimatedLoginCharactersProps) {
  const shouldReduceMotion = useReducedMotion() ?? false;
  const [primaryEyesRef, primaryRawX, primaryRawY, primarySpringX, primarySpringY] = useGazeTarget();
  const [secondaryEyesRef, secondaryRawX, secondaryRawY, secondarySpringX, secondarySpringY] = useGazeTarget();
  const [emeraldEyesRef, emeraldRawX, emeraldRawY, emeraldSpringX, emeraldSpringY] = useGazeTarget();
  const [amberEyesRef, amberRawX, amberRawY, amberSpringX, amberSpringY] = useGazeTarget();
  const [
    primaryAnchorRef,
    primaryRawBodyX,
    primaryRawBodyRotate,
    primaryRawNeckScaleY,
    primaryRawHeadX,
    primaryRawHeadY,
    primaryRawHeadRotate,
    primaryBodyX,
    primaryBodyRotate,
    primaryNeckScaleY,
    primaryHeadX,
    primaryHeadY,
    primaryHeadRotate,
  ] = useMascotPose();
  const [
    secondaryAnchorRef,
    secondaryRawBodyX,
    secondaryRawBodyRotate,
    secondaryRawNeckScaleY,
    secondaryRawHeadX,
    secondaryRawHeadY,
    secondaryRawHeadRotate,
    secondaryBodyX,
    secondaryBodyRotate,
    secondaryNeckScaleY,
    secondaryHeadX,
    secondaryHeadY,
    secondaryHeadRotate,
  ] = useMascotPose();
  const [
    emeraldAnchorRef,
    emeraldRawBodyX,
    emeraldRawBodyRotate,
    emeraldRawNeckScaleY,
    emeraldRawHeadX,
    emeraldRawHeadY,
    emeraldRawHeadRotate,
    emeraldBodyX,
    emeraldBodyRotate,
    emeraldNeckScaleY,
    emeraldHeadX,
    emeraldHeadY,
    emeraldHeadRotate,
  ] = useMascotPose();
  const [
    amberAnchorRef,
    amberRawBodyX,
    amberRawBodyRotate,
    amberRawNeckScaleY,
    amberRawHeadX,
    amberRawHeadY,
    amberRawHeadRotate,
    amberBodyX,
    amberBodyRotate,
    amberNeckScaleY,
    amberHeadX,
    amberHeadY,
    amberHeadRotate,
  ] = useMascotPose();
  const gazeTargets = useMemo(
    () => [
      { eyesRef: primaryEyesRef, rawX: primaryRawX, rawY: primaryRawY },
      { eyesRef: secondaryEyesRef, rawX: secondaryRawX, rawY: secondaryRawY },
      { eyesRef: emeraldEyesRef, rawX: emeraldRawX, rawY: emeraldRawY },
      { eyesRef: amberEyesRef, rawX: amberRawX, rawY: amberRawY },
    ],
    [
      amberEyesRef,
      amberRawX,
      amberRawY,
      emeraldEyesRef,
      emeraldRawX,
      emeraldRawY,
      primaryEyesRef,
      primaryRawX,
      primaryRawY,
      secondaryEyesRef,
      secondaryRawX,
      secondaryRawY,
    ]
  );
  const poseTargets = useMemo(
    () => [
      {
        anchorRef: primaryAnchorRef,
        profile: MASCOT_MOTION_PROFILES.primary,
        rawBodyX: primaryRawBodyX,
        rawBodyRotate: primaryRawBodyRotate,
        rawNeckScaleY: primaryRawNeckScaleY,
        rawHeadX: primaryRawHeadX,
        rawHeadY: primaryRawHeadY,
        rawHeadRotate: primaryRawHeadRotate,
      },
      {
        anchorRef: secondaryAnchorRef,
        profile: MASCOT_MOTION_PROFILES.secondary,
        rawBodyX: secondaryRawBodyX,
        rawBodyRotate: secondaryRawBodyRotate,
        rawNeckScaleY: secondaryRawNeckScaleY,
        rawHeadX: secondaryRawHeadX,
        rawHeadY: secondaryRawHeadY,
        rawHeadRotate: secondaryRawHeadRotate,
      },
      {
        anchorRef: emeraldAnchorRef,
        profile: MASCOT_MOTION_PROFILES.emerald,
        rawBodyX: emeraldRawBodyX,
        rawBodyRotate: emeraldRawBodyRotate,
        rawNeckScaleY: emeraldRawNeckScaleY,
        rawHeadX: emeraldRawHeadX,
        rawHeadY: emeraldRawHeadY,
        rawHeadRotate: emeraldRawHeadRotate,
      },
      {
        anchorRef: amberAnchorRef,
        profile: MASCOT_MOTION_PROFILES.amber,
        rawBodyX: amberRawBodyX,
        rawBodyRotate: amberRawBodyRotate,
        rawNeckScaleY: amberRawNeckScaleY,
        rawHeadX: amberRawHeadX,
        rawHeadY: amberRawHeadY,
        rawHeadRotate: amberRawHeadRotate,
      },
    ],
    [
      amberAnchorRef,
      amberRawBodyRotate,
      amberRawBodyX,
      amberRawHeadRotate,
      amberRawHeadX,
      amberRawHeadY,
      amberRawNeckScaleY,
      emeraldAnchorRef,
      emeraldRawBodyRotate,
      emeraldRawBodyX,
      emeraldRawHeadRotate,
      emeraldRawHeadX,
      emeraldRawHeadY,
      emeraldRawNeckScaleY,
      primaryAnchorRef,
      primaryRawBodyRotate,
      primaryRawBodyX,
      primaryRawHeadRotate,
      primaryRawHeadX,
      primaryRawHeadY,
      primaryRawNeckScaleY,
      secondaryAnchorRef,
      secondaryRawBodyRotate,
      secondaryRawBodyX,
      secondaryRawHeadRotate,
      secondaryRawHeadX,
      secondaryRawHeadY,
      secondaryRawNeckScaleY,
    ]
  );
  const latestPointer = useRef<PointerPosition>({ clientX: 0, clientY: 0, hasValue: false });
  const pointerFrame = useRef<number | null>(null);
  const updateBodiesOnFrame = useRef(false);

  const focusAllGazes = useCallback(
    (clientX: number, clientY: number) => {
      const updates = gazeTargets
        .map((target) => calculateGaze(target, clientX, clientY))
        .filter((update) => update !== null);

      updates.forEach(({ target, x, y }) => setGaze(target, x, y));
    },
    [gazeTargets]
  );

  const focusAllBodies = useCallback(
    (pointer: PointerPosition) => {
      const updates = poseTargets
        .map((target) => calculatePose(target, pointer))
        .filter((update) => update !== null);

      updates.forEach(({ target, pose }) => setPose(target, pose));
    },
    [poseTargets]
  );

  const schedulePointerRefresh = useCallback(
    (includeBodies: boolean) => {
      if (avertGaze || shouldReduceMotion || !latestPointer.current.hasValue) return;

      if (includeBodies) updateBodiesOnFrame.current = true;

      const flushPointer = () => {
        if (avertGaze || !latestPointer.current.hasValue) return;

        focusAllGazes(latestPointer.current.clientX, latestPointer.current.clientY);
        if (updateBodiesOnFrame.current) {
          updateBodiesOnFrame.current = false;
          focusAllBodies(latestPointer.current);
        }
      };

      if (typeof window.requestAnimationFrame !== 'function') {
        flushPointer();
        return;
      }

      if (pointerFrame.current !== null) return;

      pointerFrame.current = window.requestAnimationFrame(() => {
        pointerFrame.current = null;
        flushPointer();
      });
    },
    [avertGaze, focusAllBodies, focusAllGazes, shouldReduceMotion]
  );

  const refreshGazeDuringPose = useCallback(() => {
    schedulePointerRefresh(false);
  }, [schedulePointerRefresh]);

  useEffect(() => {
    if (avertGaze) {
      gazeTargets.forEach((target) => setGaze(target, AVERTED_GAZE.x, AVERTED_GAZE.y));
      poseTargets.forEach((target) => setPose(target, NEUTRAL_POSE));
    } else if (shouldReduceMotion || !latestPointer.current.hasValue) {
      gazeTargets.forEach((target) => setGaze(target, 0, 0));
      poseTargets.forEach((target) => setPose(target, NEUTRAL_POSE));
    } else {
      focusAllGazes(latestPointer.current.clientX, latestPointer.current.clientY);
      focusAllBodies(latestPointer.current);
    }

    if (shouldReduceMotion) return;

    const handlePointerMove = (event: PointerEvent) => {
      latestPointer.current = {
        clientX: event.clientX,
        clientY: event.clientY,
        hasValue: true,
      };

      if (!avertGaze) {
        schedulePointerRefresh(true);
      }
    };

    window.addEventListener('pointermove', handlePointerMove, { passive: true });
    return () => {
      window.removeEventListener('pointermove', handlePointerMove);
      updateBodiesOnFrame.current = false;
      if (pointerFrame.current !== null && typeof window.cancelAnimationFrame === 'function') {
        window.cancelAnimationFrame(pointerFrame.current);
        pointerFrame.current = null;
      }
    };
  }, [
    avertGaze,
    focusAllBodies,
    focusAllGazes,
    gazeTargets,
    poseTargets,
    schedulePointerRefresh,
    shouldReduceMotion,
  ]);

  return (
    <section
      className="relative hidden min-h-[680px] overflow-hidden bg-[linear-gradient(145deg,#f0f9ff_0%,#ffffff_46%,#f5f3ff_100%)] lg:flex lg:flex-col lg:justify-between dark:bg-[linear-gradient(145deg,#07111f_0%,#111827_48%,#21143d_100%)]"
      aria-hidden="true"
      data-testid="animated-login-characters"
      data-gaze={avertGaze ? 'averted' : 'tracking'}
    >
      <div
        className="absolute inset-0 opacity-45 [background-image:linear-gradient(rgba(14,165,233,0.08)_1px,transparent_1px),linear-gradient(90deg,rgba(139,92,246,0.08)_1px,transparent_1px)] [background-size:32px_32px] dark:opacity-25"
        aria-hidden="true"
      />

      <div className="relative z-10 flex items-center gap-3 px-8 pt-8">
        <span className="flex h-9 w-9 items-center justify-center rounded-lg bg-linear-to-br from-primary-500 to-secondary-600 text-sm font-bold text-white shadow-lg shadow-primary-500/20">
          M
        </span>
        <span className="text-sm font-semibold text-surface-800 dark:text-white">高数智学</span>
      </div>

      <div className="relative mx-auto h-[410px] w-full max-w-[430px] overflow-hidden">
        <Mascot
          className="bottom-[-2px] left-[54px] z-10 h-[320px] w-[150px]"
          baseClassName="bg-primary-700"
          bodyClassName="rounded-t-[52px] bg-linear-to-b from-primary-400 via-primary-500 to-primary-700"
          eyeClassName="left-8 top-14"
          eyeGapClassName="gap-8"
          eyeWidthClassName="w-[72px]"
          gazeId="primary"
          eyesRef={primaryEyesRef}
          rawX={primaryRawX}
          rawY={primaryRawY}
          springX={primarySpringX}
          springY={primarySpringY}
          anchorRef={primaryAnchorRef}
          motionProfile={MASCOT_MOTION_PROFILES.primary}
          rawBodyX={primaryRawBodyX}
          rawBodyRotate={primaryRawBodyRotate}
          rawNeckScaleY={primaryRawNeckScaleY}
          rawHeadX={primaryRawHeadX}
          rawHeadY={primaryRawHeadY}
          rawHeadRotate={primaryRawHeadRotate}
          springBodyX={primaryBodyX}
          springBodyRotate={primaryBodyRotate}
          springNeckScaleY={primaryNeckScaleY}
          springHeadX={primaryHeadX}
          springHeadY={primaryHeadY}
          springHeadRotate={primaryHeadRotate}
          onPoseUpdate={refreshGazeDuringPose}
          avertGaze={avertGaze}
          reduceMotion={shouldReduceMotion}
        />

        <Mascot
          className="bottom-[-2px] left-[188px] z-20 h-[252px] w-[112px]"
          baseClassName="bg-secondary-700"
          bodyClassName="rounded-t-[38px] bg-linear-to-b from-secondary-400 via-secondary-500 to-secondary-700"
          eyeClassName="left-5 top-12"
          eyeGapClassName="gap-5"
          eyeWidthClassName="w-[60px]"
          gazeId="secondary"
          eyesRef={secondaryEyesRef}
          rawX={secondaryRawX}
          rawY={secondaryRawY}
          springX={secondarySpringX}
          springY={secondarySpringY}
          anchorRef={secondaryAnchorRef}
          motionProfile={MASCOT_MOTION_PROFILES.secondary}
          rawBodyX={secondaryRawBodyX}
          rawBodyRotate={secondaryRawBodyRotate}
          rawNeckScaleY={secondaryRawNeckScaleY}
          rawHeadX={secondaryRawHeadX}
          rawHeadY={secondaryRawHeadY}
          rawHeadRotate={secondaryRawHeadRotate}
          springBodyX={secondaryBodyX}
          springBodyRotate={secondaryBodyRotate}
          springNeckScaleY={secondaryNeckScaleY}
          springHeadX={secondaryHeadX}
          springHeadY={secondaryHeadY}
          springHeadRotate={secondaryHeadRotate}
          onPoseUpdate={refreshGazeDuringPose}
          avertGaze={avertGaze}
          reduceMotion={shouldReduceMotion}
        />

        <Mascot
          className="bottom-[-2px] left-[10px] z-30 h-[172px] w-[210px]"
          baseClassName="bg-teal-700"
          bodyClassName="rounded-t-[105px] bg-linear-to-b from-emerald-400 via-emerald-500 to-teal-700"
          eyeClassName="left-[70px] top-[66px]"
          eyeGapClassName="gap-8"
          eyeWidthClassName="w-[72px]"
          gazeId="emerald"
          eyesRef={emeraldEyesRef}
          rawX={emeraldRawX}
          rawY={emeraldRawY}
          springX={emeraldSpringX}
          springY={emeraldSpringY}
          anchorRef={emeraldAnchorRef}
          motionProfile={MASCOT_MOTION_PROFILES.emerald}
          rawBodyX={emeraldRawBodyX}
          rawBodyRotate={emeraldRawBodyRotate}
          rawNeckScaleY={emeraldRawNeckScaleY}
          rawHeadX={emeraldRawHeadX}
          rawHeadY={emeraldRawHeadY}
          rawHeadRotate={emeraldRawHeadRotate}
          springBodyX={emeraldBodyX}
          springBodyRotate={emeraldBodyRotate}
          springNeckScaleY={emeraldNeckScaleY}
          springHeadX={emeraldHeadX}
          springHeadY={emeraldHeadY}
          springHeadRotate={emeraldHeadRotate}
          onPoseUpdate={refreshGazeDuringPose}
          avertGaze={avertGaze}
          reduceMotion={shouldReduceMotion}
        />

        <Mascot
          className="bottom-[-2px] left-[280px] z-40 h-[202px] w-[118px]"
          baseClassName="bg-amber-600"
          bodyClassName="rounded-t-[60px] bg-linear-to-b from-amber-300 via-amber-400 to-amber-600"
          eyeClassName="left-7 top-11"
          eyeGapClassName="gap-5"
          eyeWidthClassName="w-[60px]"
          gazeId="amber"
          eyesRef={amberEyesRef}
          rawX={amberRawX}
          rawY={amberRawY}
          springX={amberSpringX}
          springY={amberSpringY}
          anchorRef={amberAnchorRef}
          motionProfile={MASCOT_MOTION_PROFILES.amber}
          rawBodyX={amberRawBodyX}
          rawBodyRotate={amberRawBodyRotate}
          rawNeckScaleY={amberRawNeckScaleY}
          rawHeadX={amberRawHeadX}
          rawHeadY={amberRawHeadY}
          rawHeadRotate={amberRawHeadRotate}
          springBodyX={amberBodyX}
          springBodyRotate={amberBodyRotate}
          springNeckScaleY={amberNeckScaleY}
          springHeadX={amberHeadX}
          springHeadY={amberHeadY}
          springHeadRotate={amberHeadRotate}
          onPoseUpdate={refreshGazeDuringPose}
          avertGaze={avertGaze}
          reduceMotion={shouldReduceMotion}
        />
      </div>

      <div className="h-12" aria-hidden="true" />
    </section>
  );
}
