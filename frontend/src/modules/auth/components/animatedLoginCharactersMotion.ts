export interface PointerPosition {
  clientX: number;
  clientY: number;
  hasValue: boolean;
}

export interface MascotMotionProfile {
  bodyHeight: number;
  nearDistance: number;
  farDistance: number;
  maxNeckStretch: number;
  maxBodyX: number;
  maxBodyRotate: number;
  maxHeadX: number;
  maxHeadY: number;
  maxHeadRotate: number;
  initialBlinkDelay: number;
  blinkDelayRange: readonly [number, number];
}

export interface MascotPose {
  bodyX: number;
  bodyRotate: number;
  neckScaleY: number;
  headX: number;
  headY: number;
  headRotate: number;
}

export const NEUTRAL_POSE: MascotPose = {
  bodyX: 0,
  bodyRotate: 0,
  neckScaleY: 1,
  headX: 0,
  headY: 0,
  headRotate: 0,
};

export const MASCOT_MOTION_PROFILES = {
  primary: {
    bodyHeight: 320,
    nearDistance: 105,
    farDistance: 780,
    maxNeckStretch: 0.11,
    maxBodyX: 8,
    maxBodyRotate: 4,
    maxHeadX: 12,
    maxHeadY: 9,
    maxHeadRotate: 5,
    initialBlinkDelay: 1300,
    blinkDelayRange: [3200, 6500],
  },
  secondary: {
    bodyHeight: 252,
    nearDistance: 90,
    farDistance: 700,
    maxNeckStretch: 0.14,
    maxBodyX: 11,
    maxBodyRotate: 6,
    maxHeadX: 14,
    maxHeadY: 10,
    maxHeadRotate: 7,
    initialBlinkDelay: 2200,
    blinkDelayRange: [4100, 7600],
  },
  emerald: {
    bodyHeight: 172,
    nearDistance: 100,
    farDistance: 650,
    maxNeckStretch: 0.17,
    maxBodyX: 9,
    maxBodyRotate: 5,
    maxHeadX: 13,
    maxHeadY: 11,
    maxHeadRotate: 6,
    initialBlinkDelay: 3100,
    blinkDelayRange: [5000, 8600],
  },
  amber: {
    bodyHeight: 202,
    nearDistance: 80,
    farDistance: 620,
    maxNeckStretch: 0.19,
    maxBodyX: 10,
    maxBodyRotate: 6,
    maxHeadX: 16,
    maxHeadY: 12,
    maxHeadRotate: 8,
    initialBlinkDelay: 4000,
    blinkDelayRange: [2700, 5800],
  },
} satisfies Record<string, MascotMotionProfile>;

const NEAR_HEAD_TURN_SHARE = 0.4;

function clamp(value: number, minimum: number, maximum: number) {
  return Math.min(Math.max(value, minimum), maximum);
}

function smoothStep(value: number) {
  return value * value * (3 - 2 * value);
}

export function calculateMascotPose(
  bounds: Pick<DOMRect, 'left' | 'top' | 'width' | 'height'>,
  pointer: Pick<PointerPosition, 'clientX' | 'clientY'>,
  profile: MascotMotionProfile
): MascotPose {
  const deltaX = pointer.clientX - (bounds.left + bounds.width / 2);
  const deltaY = pointer.clientY - (bounds.top + bounds.height / 2);
  const distance = Math.hypot(deltaX, deltaY);

  if (distance < 0.5) return NEUTRAL_POSE;

  const attentionIntensity = smoothStep(
    clamp(distance / Math.max(profile.nearDistance, 1), 0, 1)
  );
  const rawReachIntensity = clamp(
    (distance - profile.nearDistance) /
      Math.max(profile.farDistance - profile.nearDistance, 1),
    0,
    1
  );
  const reachIntensity = smoothStep(rawReachIntensity);
  const headIntensity =
    attentionIntensity *
    (NEAR_HEAD_TURN_SHARE + (1 - NEAR_HEAD_TURN_SHARE) * reachIntensity);
  const directionX = deltaX / distance;
  const directionY = deltaY / distance;
  const downwardStretchDamping = 1 - Math.max(0, directionY);
  const stretchBias = clamp(
    Math.max(0, -directionY) + Math.abs(directionX) * 0.45 * downwardStretchDamping,
    0,
    1
  );
  const neckScaleY = 1 + profile.maxNeckStretch * reachIntensity * stretchBias;
  const neckExtension = profile.bodyHeight * (neckScaleY - 1);

  return {
    bodyX: directionX * profile.maxBodyX * reachIntensity,
    bodyRotate: directionX * profile.maxBodyRotate * reachIntensity,
    neckScaleY,
    headX: directionX * profile.maxHeadX * headIntensity,
    headY: -neckExtension + directionY * profile.maxHeadY * headIntensity,
    headRotate: directionX * profile.maxHeadRotate * headIntensity,
  };
}
