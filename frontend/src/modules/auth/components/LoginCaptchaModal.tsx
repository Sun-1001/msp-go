import { useCallback } from 'react';
import { ShieldCheck } from 'lucide-react';
import { Modal } from '@/components/ui/Modal';
import { SliderCaptcha } from './SliderCaptcha';

interface LoginCaptchaModalProps {
  isOpen: boolean;
  onClose: () => void;
  onVerified: (captchaToken: string) => void;
}

export function LoginCaptchaModal({ isOpen, onClose, onVerified }: LoginCaptchaModalProps) {
  const handleTokenChange = useCallback((captchaToken: string | null) => {
    if (captchaToken) {
      onVerified(captchaToken);
    }
  }, [onVerified]);

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title={(
        <span className="flex items-center gap-2">
          <ShieldCheck className="h-5 w-5 text-secondary-600 dark:text-secondary-300" aria-hidden="true" />
          安全验证
        </span>
      )}
      className="max-w-sm p-6 sm:p-8"
    >
      <SliderCaptcha onTokenChange={handleTokenChange} />
    </Modal>
  );
}
