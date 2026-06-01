import React, { useState, useCallback, useRef, useEffect } from 'react';
import { Modal } from '@/components/ui/Modal';
import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';
import { XCircle, RefreshCw } from 'lucide-react';
import { xidianService, type XidianCaptchaChallenge } from '@/modules/xidian/services/xidianService';
import { clearCredential } from '../services/credentialStorage';
import { getApiErrorMessage } from '@/libs/http/apiClient';
import { XIDIAN_REAUTH_EVENT } from '@/libs/auth/xidianEvents';
import { XidianReauthContext } from '../hooks/useXidianReauth';

interface ReauthRequest {
  resolve: () => void;
  reject: (error: Error) => void;
}

interface XidianReauthProviderProps {
  children: React.ReactNode;
}

const parseXidianError = (err: unknown) => {
  const errorData = (err as { response?: { data?: { code?: string; message?: string } } })?.response?.data;
  const code = errorData?.code;
  // 区分密码变更和网络错误
  if (code === 'PASSWORD_WRONG') {
    return '密码可能已变更，请输入最新密码';
  }
  if (code === 'CAPTCHA_FAILED') {
    return '验证码校验失败，请重新拖动滑块';
  }
  return errorData?.message || getApiErrorMessage(err, '网络错误，请检查网络后重试');
};

export const XidianReauthProvider: React.FC<XidianReauthProviderProps> = ({ children }) => {
  const [isOpen, setIsOpen] = useState(false);
  const [captchaChallenge, setCaptchaChallenge] = useState<XidianCaptchaChallenge | null>(null);
  const [captchaLoading, setCaptchaLoading] = useState(false);
  const [formData, setFormData] = useState({ username: '', password: '' });
  const [sliderValue, setSliderValue] = useState(0);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const pendingRequestRef = useRef<ReauthRequest | null>(null);

  const loadCaptcha = useCallback(async () => {
    setCaptchaLoading(true);
    setError(null);
    try {
      const challenge = await xidianService.startBinding();
      setCaptchaChallenge(challenge);
      setSliderValue(0);
    } catch (err) {
      setError(parseXidianError(err));
    } finally {
      setCaptchaLoading(false);
    }
  }, []);

  const openModal = useCallback(async () => {
    clearCredential();
    setFormData({ username: '', password: '' });
    setSliderValue(0);
    setError(null);
    setIsOpen(true);
    await loadCaptcha();
  }, [loadCaptcha]);

  const closeModal = useCallback((success: boolean) => {
    setIsOpen(false);
    if (pendingRequestRef.current) {
      if (success) {
        pendingRequestRef.current.resolve();
      } else {
        pendingRequestRef.current.reject(new Error('用户取消重新验证'));
      }
      pendingRequestRef.current = null;
    }
  }, []);

  const triggerReauth = useCallback((): Promise<void> => {
    return new Promise((resolve, reject) => {
      pendingRequestRef.current = { resolve, reject };
      openModal();
    });
  }, [openModal]);

  const handleSubmit = async () => {
    if (!captchaChallenge) {
      setError('请先获取验证码');
      return;
    }

    setSubmitting(true);
    setError(null);

    try {
      await xidianService.completeBinding({
        challenge_id: captchaChallenge.challenge_id,
        slider_position: sliderValue,
        username: formData.username || undefined,
        password: formData.password || undefined,
      });

      closeModal(true);
    } catch (err) {
      setError(parseXidianError(err));
      // 验证失败，刷新验证码
      await loadCaptcha();
    } finally {
      setSubmitting(false);
    }
  };

  // 监听全局重新验证事件
  useEffect(() => {
    clearCredential();
    const handleReauthEvent = () => {
      if (!isOpen) {
        openModal();
      }
    };

    window.addEventListener(XIDIAN_REAUTH_EVENT, handleReauthEvent);
    return () => {
      window.removeEventListener(XIDIAN_REAUTH_EVENT, handleReauthEvent);
    };
  }, [isOpen, openModal]);

  return (
    <XidianReauthContext.Provider value={{ triggerReauth }}>
      {children}
      <Modal
        isOpen={isOpen}
        onClose={() => closeModal(false)}
        title="西电账号需要重新验证"
        className="max-w-lg"
      >
        <div className="space-y-4">
          <p className="text-sm text-surface-500 dark:text-surface-400">
            自动登录失败，可能是密码已变更或会话异常。请重新输入凭证完成验证。
          </p>

          <div className="space-y-2">
            <label className="text-sm font-medium text-surface-700 dark:text-surface-300">
              学号/工号
            </label>
            <Input
              placeholder="请输入学号/工号"
              value={formData.username}
              onChange={(e) => setFormData((prev) => ({ ...prev, username: e.target.value }))}
              disabled={submitting}
            />
          </div>

          <div className="space-y-2">
            <label className="text-sm font-medium text-surface-700 dark:text-surface-300">
              密码
            </label>
            <Input
              type="password"
              placeholder="请输入密码"
              value={formData.password}
              onChange={(e) => setFormData((prev) => ({ ...prev, password: e.target.value }))}
              disabled={submitting}
            />
          </div>

          <div className="space-y-2">
            <div className="flex items-center justify-between">
              <span className="text-sm font-medium text-surface-700 dark:text-surface-300">
                滑块验证码
              </span>
              <Button
                variant="ghost"
                size="sm"
                onClick={loadCaptcha}
                disabled={captchaLoading || submitting}
              >
                <RefreshCw className={`h-4 w-4 mr-1 ${captchaLoading ? 'animate-spin' : ''}`} />
                刷新
              </Button>
            </div>

            <div className="flex flex-col items-center gap-3">
              {captchaLoading && (
                <div className="flex items-center gap-2 py-8">
                  <RefreshCw className="h-5 w-5 animate-spin text-surface-400" />
                  <span className="text-sm text-surface-500">验证码加载中...</span>
                </div>
              )}
              {!captchaLoading && captchaChallenge && (
                <>
                  <div
                    className="relative overflow-hidden rounded-lg border border-surface-200 dark:border-surface-700"
                    style={{
                      width: captchaChallenge.puzzle_width,
                      height: captchaChallenge.puzzle_height,
                    }}
                  >
                    <img
                      src={`data:image/png;base64,${captchaChallenge.captcha_big}`}
                      alt="captcha"
                      className="h-full w-full object-cover"
                    />
                    <img
                      src={`data:image/png;base64,${captchaChallenge.captcha_piece}`}
                      alt="piece"
                      className="absolute"
                      style={{
                        left: Math.max(0, sliderValue * captchaChallenge.puzzle_width),
                        top: captchaChallenge.piece_y || 0,
                        width: captchaChallenge.piece_width,
                        height: captchaChallenge.piece_height,
                      }}
                    />
                  </div>
                  <input
                    type="range"
                    min={0}
                    max={100}
                    value={Math.round(sliderValue * 100)}
                    onChange={(e) => setSliderValue(Number(e.target.value) / 100)}
                    className="w-full"
                    disabled={submitting}
                  />
                </>
              )}
            </div>
          </div>

          {error && (
            <div className="flex items-center gap-2 rounded-lg bg-red-50 p-3 text-red-700 dark:bg-red-900/20 dark:text-red-400">
              <XCircle className="h-4 w-4 shrink-0" />
              <span className="text-sm">{error}</span>
            </div>
          )}

          <div className="flex justify-end gap-2 pt-2">
            <Button variant="outline" onClick={() => closeModal(false)} disabled={submitting}>
              取消
            </Button>
            <Button onClick={handleSubmit} disabled={submitting || captchaLoading}>
              {submitting ? '验证中...' : '重新验证'}
            </Button>
          </div>
        </div>
      </Modal>
    </XidianReauthContext.Provider>
  );
};
