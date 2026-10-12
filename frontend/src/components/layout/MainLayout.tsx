import React from 'react';
import { Header } from '@/modules/auth/navigation';
import { Footer } from './Footer';

interface MainLayoutProps {
  children: React.ReactNode;
  headerVariant?: 'default' | 'transparent' | 'dark';
  footerVariant?: 'default' | 'dark';
  showFooter?: boolean;
  className?: string;
  onLoginClick?: () => void;
  onRegisterClick?: () => void;
}

export const MainLayout: React.FC<MainLayoutProps> = ({
  children,
  headerVariant = 'default',
  footerVariant = 'default',
  showFooter = true,
  className = '',
  onLoginClick,
  onRegisterClick
}) => {
  return (
    <div className={`min-h-screen bg-surface-50 dark:bg-surface-950 font-sans text-surface-900 dark:text-surface-100 flex flex-col ${className}`}>
      <Header variant={headerVariant} onLoginClick={onLoginClick} onRegisterClick={onRegisterClick} />
      <main className="grow w-full relative overflow-x-hidden">
        {children}
      </main>
      {showFooter && <Footer variant={footerVariant} />}
    </div>
  );
};