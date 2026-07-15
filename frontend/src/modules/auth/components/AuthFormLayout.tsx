import type { ReactNode } from 'react';
import { AnimatedLoginCharacters } from './AnimatedLoginCharacters';

interface AuthFormLayoutProps {
  children: ReactNode;
  avertGaze?: boolean;
}

export function AuthFormLayout({ children, avertGaze = false }: AuthFormLayoutProps) {
  return (
    <div
      className="relative z-[1] grid w-full overflow-hidden lg:min-h-[680px] lg:grid-cols-[minmax(390px,0.92fr)_minmax(440px,1.08fr)]"
      data-testid="auth-form-layout"
    >
      <AnimatedLoginCharacters avertGaze={avertGaze} />

      <div className="max-h-[calc(100vh-2rem)] overflow-y-auto bg-white px-6 py-8 dark:bg-surface-900 sm:px-8 lg:px-12 lg:py-10">
        <div className="mx-auto w-full max-w-[420px] space-y-5">{children}</div>
      </div>
    </div>
  );
}
