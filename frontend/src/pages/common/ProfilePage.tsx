import React, { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { useAppDispatch, useAppSelector } from '@/store';
import { selectCurrentUser, fetchCurrentUser } from '@/modules/auth/store/authSlice';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/Card';
import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';
import { Modal } from '@/components/ui/Modal';
import { User, Phone, Mail, School, Lock, ArrowLeft, Loader2, CheckCircle, XCircle } from 'lucide-react';
import { getApiErrorMessage } from '@/libs/http/apiClient';
import { passwordChangeSchema, type PasswordChangeFormData } from '@/libs/validation/schemas';
import { authService } from '@/modules/auth/services/authService';
import { xidianService, type XidianBindingStatus, type XidianCaptchaChallenge } from '@/modules/xidian/services/xidianService';
import { clearCredential } from '@/modules/xidian';

export const ProfilePage: React.FC = () => {
  const user = useAppSelector(selectCurrentUser);
  const navigate = useNavigate();

  // 修改密码表单状态
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [submitStatus, setSubmitStatus] = useState<{ type: 'success' | 'error'; message: string } | null>(null);
  const [xidianStatus, setXidianStatus] = useState<XidianBindingStatus | null>(null);
  const [xidianLoading, setXidianLoading] = useState(false);
  const [bindingModalOpen, setBindingModalOpen] = useState(false);
  const [captchaChallenge, setCaptchaChallenge] = useState<XidianCaptchaChallenge | null>(null);
  const [captchaLoading, setCaptchaLoading] = useState(false);
  const [bindingForm, setBindingForm] = useState({ username: '', password: '' });
  const [sliderValue, setSliderValue] = useState(0);
  const [bindingError, setBindingError] = useState<string | null>(null);
  const [bindingSubmitting, setBindingSubmitting] = useState(false);
  const [xidianActionStatus, setXidianActionStatus] = useState<{ type: 'success' | 'error'; message: string } | null>(null);
  const [syncingType, setSyncingType] = useState<'classtable' | 'exams' | 'scores' | null>(null);
  const [emailBindModalOpen, setEmailBindModalOpen] = useState(false);
  const [emailBindValue, setEmailBindValue] = useState('');
  const [emailCodeValue, setEmailCodeValue] = useState('');
  const [emailCodeSent, setEmailCodeSent] = useState(false);
  const [emailBindSubmitting, setEmailBindSubmitting] = useState(false);
  const [emailBindError, setEmailBindError] = useState<string | null>(null);
  const [emailActionStatus, setEmailActionStatus] = useState<{ type: 'success' | 'error'; message: string } | null>(null);

  const dispatch = useAppDispatch();

  const {
    register,
    handleSubmit,
    reset,
    formState: { errors },
  } = useForm<PasswordChangeFormData>({
    resolver: zodResolver(passwordChangeSchema),
  });

  const onSubmitPasswordChange = async (data: PasswordChangeFormData) => {
    setIsSubmitting(true);
    setSubmitStatus(null);

    try {
      const response = await authService.changePassword({
        old_password: data.currentPassword,
        new_password: data.newPassword,
      });
      setSubmitStatus({ type: 'success', message: response.message || '密码修改成功' });
      reset();
    } catch (error: unknown) {
      const errorMessage = error instanceof Error
        ? error.message
        : (error as { response?: { data?: { detail?: string } } })?.response?.data?.detail || '密码修改失败，请重试';
      setSubmitStatus({ type: 'error', message: errorMessage });
    } finally {
      setIsSubmitting(false);
    }
  };

  const parseXidianError = (error: unknown) => {
    const errorData = (error as { response?: { data?: { message?: string; code?: string } } })?.response?.data;
    return {
      message: errorData?.message || getApiErrorMessage(error, '操作失败，请稍后重试'),
      code: errorData?.code,
    };
  };

  const loadXidianStatus = async () => {
    if (!user?.id) return;
    setXidianLoading(true);
    try {
      const status = await xidianService.getBindingStatus();
      setXidianStatus(status);
    } catch (error) {
      setXidianActionStatus({ type: 'error', message: parseXidianError(error).message });
    } finally {
      setXidianLoading(false);
    }
  };

  useEffect(() => {
    loadXidianStatus();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [user?.id]);

  const handleOpenBinding = async () => {
    setBindingModalOpen(true);
    setBindingError(null);
    setSliderValue(0);
    clearCredential();
    setBindingForm({ username: '', password: '' });
    setCaptchaLoading(true);
    try {
      const challenge = await xidianService.startBinding();
      setCaptchaChallenge(challenge);
    } catch (error) {
      setBindingError(parseXidianError(error).message);
    } finally {
      setCaptchaLoading(false);
    }
  };

  const handleRefreshCaptcha = async () => {
    setBindingError(null);
    setCaptchaLoading(true);
    try {
      const challenge = await xidianService.startBinding();
      setCaptchaChallenge(challenge);
      setSliderValue(0);
    } catch (error) {
      setBindingError(parseXidianError(error).message);
    } finally {
      setCaptchaLoading(false);
    }
  };

  const handleCompleteBinding = async () => {
    if (!captchaChallenge) {
      setBindingError('请先获取验证码');
      return;
    }
    setBindingSubmitting(true);
    setBindingError(null);
    try {
      const response = await xidianService.completeBinding({
        challenge_id: captchaChallenge.challenge_id,
        slider_position: sliderValue,
        username: bindingForm.username || undefined,
        password: bindingForm.password || undefined,
      });
      setXidianStatus({
        is_bound: response.is_bound,
        username: response.username,
        is_postgraduate: response.is_postgraduate,
        last_verified_at: response.last_verified_at,
      });
      setXidianActionStatus({ type: 'success', message: '绑定成功' });
      setBindingModalOpen(false);
      setBindingForm({ username: '', password: '' });
    } catch (error) {
      const parsed = parseXidianError(error);
      setBindingError(parsed.message);
    } finally {
      setBindingSubmitting(false);
    }
  };

  const handleUnbind = async () => {
    setXidianActionStatus(null);
    try {
      await xidianService.unbind();
      setXidianStatus({ is_bound: false });
      // 解绑时清除保存的凭证
      clearCredential();
      setXidianActionStatus({ type: 'success', message: '已解绑' });
    } catch (error) {
      setXidianActionStatus({ type: 'error', message: parseXidianError(error).message });
    }
  };

  const handleSync = async (type: 'classtable' | 'exams' | 'scores') => {
    setSyncingType(type);
    setXidianActionStatus(null);
    try {
      if (type === 'classtable') {
        await xidianService.syncClasstable();
      } else if (type === 'exams') {
        await xidianService.syncExams();
      } else {
        await xidianService.syncScores();
      }
      setXidianActionStatus({ type: 'success', message: '同步成功' });
      await loadXidianStatus();
    } catch (error) {
      const parsed = parseXidianError(error);
      setXidianActionStatus({ type: 'error', message: parsed.message });
      if (parsed.code === 'CAPTCHA_REQUIRED') {
        await handleOpenBinding();
        setBindingError('会话已过期，请重新验证');
      }
    } finally {
      setSyncingType(null);
    }
  };

  const isEmailBound = Boolean(user?.email && user.email_verified);

  const handleOpenEmailBind = () => {
    setEmailBindModalOpen(true);
    setEmailBindValue(isEmailBound ? '' : (user?.email || ''));
    setEmailCodeValue('');
    setEmailCodeSent(false);
    setEmailBindError(null);
    setEmailActionStatus(null);
  };

  const handleSendEmailCode = async () => {
    const email = emailBindValue.trim();
    if (!email) {
      setEmailBindError('请输入邮箱地址');
      return;
    }
    if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) {
      setEmailBindError('请输入有效的邮箱格式');
      return;
    }
    setEmailBindSubmitting(true);
    setEmailBindError(null);
    try {
      const res = await authService.bindEmail(email);
      setEmailCodeSent(true);
      setEmailActionStatus({ type: 'success', message: res.message });
    } catch (err: unknown) {
      setEmailBindError(getApiErrorMessage(err, '发送失败，请重试'));
    } finally {
      setEmailBindSubmitting(false);
    }
  };

  const handleVerifyEmailCode = async () => {
    const email = emailBindValue.trim();
    const code = emailCodeValue.trim();
    if (!email || !code) {
      setEmailBindError('请输入邮箱和验证码');
      return;
    }
    if (code.length !== 6 || !/^\d+$/.test(code)) {
      setEmailBindError('请输入 6 位数字验证码');
      return;
    }
    setEmailBindSubmitting(true);
    setEmailBindError(null);
    try {
      await authService.verifyEmailByCode(email, code);
      setEmailActionStatus({ type: 'success', message: '邮箱验证成功' });
      setEmailBindModalOpen(false);
      dispatch(fetchCurrentUser());
    } catch (err: unknown) {
      setEmailBindError(getApiErrorMessage(err, '验证失败，请重试'));
    } finally {
      setEmailBindSubmitting(false);
    }
  };

  const isXidianBound = xidianStatus?.is_bound;
  const lastSyncText = xidianStatus?.last_sync_at
    ? new Date(xidianStatus.last_sync_at).toLocaleString()
    : null;

  return (
    <div className="container mx-auto px-4 py-8 max-w-4xl">
      <Button
        variant="ghost"
        className="mb-4 pl-0 hover:bg-transparent hover:text-primary-600 dark:hover:text-primary-400"
        onClick={() => navigate(-1)}
      >
        <ArrowLeft className="w-4 h-4 mr-2" />
        返回
      </Button>
      <h1 className="text-3xl font-bold mb-8 text-surface-900 dark:text-surface-100">个人中心</h1>

      <div className="grid gap-6">
        {/* Basic Info */}
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <User className="w-5 h-5" />
              基本信息
            </CardTitle>
            <CardDescription>查看和管理您的个人基本信息</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="flex items-center gap-6">
              <div className="h-24 w-24 rounded-full bg-linear-to-tr from-primary-100 to-secondary-100 dark:from-primary-900 dark:to-secondary-900 flex items-center justify-center text-primary-700 dark:text-primary-300 font-bold text-3xl shadow-md">
                <User className="w-12 h-12" />
              </div>
              <div className="space-y-1">
                <h3 className="text-2xl font-bold text-surface-900 dark:text-surface-100">{user?.name || '未登录'}</h3>
                <div className="flex items-center gap-2 text-surface-500 dark:text-surface-400">
                  <span className="px-2 py-0.5 bg-surface-100 dark:bg-surface-700 rounded text-xs font-medium border border-surface-200 dark:border-surface-600">
                    {user?.role === 'student' ? '学生' : user?.role === 'teacher' ? '教师' : '访客'}
                  </span>
                  <span className="text-sm">ID: {user?.id}</span>
                </div>
              </div>
            </div>
          </CardContent>
        </Card>

        {/* Account Binding */}
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Lock className="w-5 h-5" />
              账号安全与绑定
            </CardTitle>
            <CardDescription>管理您的手机号、邮箱及学校账户绑定</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            {/* Phone */}
            <div className="flex items-center justify-between p-4 border border-surface-200 dark:border-surface-700 rounded-lg bg-surface-50/50 dark:bg-surface-800/50 hover:bg-surface-50 dark:hover:bg-surface-800 transition-colors">
                <div className="flex items-center gap-4">
                    <div className="p-2.5 bg-white dark:bg-surface-700 rounded-full border border-surface-200 dark:border-surface-600 text-surface-600 dark:text-surface-400">
                        <Phone className="w-5 h-5" />
                    </div>
                    <div>
                        <p className="font-medium text-surface-900 dark:text-surface-100">手机号码</p>
                        <p className="text-sm text-surface-500 dark:text-surface-400">未绑定</p>
                    </div>
                </div>
                <Button variant="outline" size="sm">绑定</Button>
            </div>

            {/* Email */}
            <div className="flex items-center justify-between p-4 border border-surface-200 dark:border-surface-700 rounded-lg bg-surface-50/50 dark:bg-surface-800/50 hover:bg-surface-50 dark:hover:bg-surface-800 transition-colors">
              <div className="flex items-center gap-4">
                <div className="p-2.5 bg-white dark:bg-surface-700 rounded-full border border-surface-200 dark:border-surface-600 text-surface-600 dark:text-surface-400">
                  <Mail className="w-5 h-5" />
                </div>
                <div className="flex flex-col gap-1.5">
                  <div className="flex items-center gap-2">
                    <p className="font-medium text-surface-900 dark:text-surface-100">电子邮箱</p>
                    {user?.email && (
                      <span
                        className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-medium ${
                          isEmailBound
                            ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400'
                            : 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400'
                        }`}
                      >
                        {isEmailBound ? '已验证' : '未验证'}
                      </span>
                    )}
                  </div>
                  <p className="text-sm text-surface-500 dark:text-surface-400">
                    {(user?.email || isEmailBound) ? (user?.email || '') : '未绑定'}
                  </p>
                </div>
              </div>
              <div className="flex items-center gap-2">
                {user?.email && !isEmailBound ? (
                  <>
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => {
                        setEmailBindValue(user?.email || '');
                        setEmailCodeValue('');
                        setEmailCodeSent(false);
                        setEmailBindError(null);
                        setEmailActionStatus(null);
                        setEmailBindModalOpen(true);
                      }}
                    >
                      验证
                    </Button>
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => {
                        setEmailBindValue('');
                        setEmailCodeValue('');
                        setEmailCodeSent(false);
                        setEmailBindError(null);
                        setEmailActionStatus(null);
                        setEmailBindModalOpen(true);
                      }}
                    >
                      换绑
                    </Button>
                  </>
                ) : (
                  <Button variant="outline" size="sm" onClick={handleOpenEmailBind}>
                    {isEmailBound ? '换绑' : '绑定'}
                  </Button>
                )}
              </div>
            </div>

             {/* Xidian Account */}
             <div className="flex items-center justify-between p-4 border border-surface-200 dark:border-surface-700 rounded-lg bg-surface-50/50 dark:bg-surface-800/50 hover:bg-surface-50 dark:hover:bg-surface-800 transition-colors">
                <div className="flex items-center gap-4">
                    <div className="p-2.5 bg-white dark:bg-surface-700 rounded-full border border-surface-200 dark:border-surface-600 text-surface-600 dark:text-surface-400">
                        <School className="w-5 h-5" />
                    </div>
                    <div>
                        <p className="font-medium text-surface-900 dark:text-surface-100">西电账号</p>
                        <p className="text-sm text-surface-500 dark:text-surface-400">
                          {xidianLoading
                            ? '加载中...'
                            : isXidianBound
                              ? `已绑定${xidianStatus?.username ? `（${xidianStatus.username}）` : ''}`
                              : '未绑定'}
                        </p>
                        {isXidianBound && (
                          <p className="text-xs text-surface-400 dark:text-surface-500">
                            {xidianStatus?.is_postgraduate ? '研究生账户' : '本科账户'}
                            {lastSyncText ? ` · 最近同步：${lastSyncText}` : ''}
                          </p>
                        )}
                    </div>
                </div>
                <div className="flex items-center gap-2">
                  {isXidianBound ? (
                    <Button variant="outline" size="sm" onClick={handleUnbind}>
                      解绑
                    </Button>
                  ) : (
                    <Button variant="outline" size="sm" onClick={handleOpenBinding}>
                      绑定
                    </Button>
                  )}
                </div>
            </div>
            {isXidianBound && (
              <div className="flex flex-wrap gap-2">
                <Button
                  variant="outline"
                  size="sm"
                  disabled={syncingType === 'classtable'}
                  onClick={() => handleSync('classtable')}
                >
                  {syncingType === 'classtable' ? '同步中...' : '同步课表'}
                </Button>
                <Button
                  variant="outline"
                  size="sm"
                  disabled={syncingType === 'exams'}
                  onClick={() => handleSync('exams')}
                >
                  {syncingType === 'exams' ? '同步中...' : '同步考试'}
                </Button>
                <Button
                  variant="outline"
                  size="sm"
                  disabled={syncingType === 'scores'}
                  onClick={() => handleSync('scores')}
                >
                  {syncingType === 'scores' ? '同步中...' : '同步成绩'}
                </Button>
              </div>
            )}
            {xidianActionStatus && (
              <div
                className={`flex items-center gap-2 p-3 rounded-lg ${
                  xidianActionStatus.type === 'success'
                    ? 'bg-green-50 text-green-700 dark:bg-green-900/20 dark:text-green-400'
                    : 'bg-red-50 text-red-700 dark:bg-red-900/20 dark:text-red-400'
                }`}
              >
                {xidianActionStatus.type === 'success' ? (
                  <CheckCircle className="w-4 h-4" />
                ) : (
                  <XCircle className="w-4 h-4" />
                )}
                <span className="text-sm">{xidianActionStatus.message}</span>
              </div>
            )}
          </CardContent>
        </Card>

        {/* Password Change */}
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Lock className="w-5 h-5" />
              修改密码
            </CardTitle>
            <CardDescription>定期修改密码以保护您的账户安全</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4 max-w-md">
            <form onSubmit={handleSubmit(onSubmitPasswordChange)} className="space-y-4">
              <div className="space-y-2">
                <label className="text-sm font-medium text-surface-700 dark:text-surface-300">当前密码</label>
                <Input
                  type="password"
                  placeholder="请输入当前密码"
                  {...register('currentPassword')}
                  disabled={isSubmitting}
                />
                {errors.currentPassword && (
                  <p className="text-sm text-red-500">{errors.currentPassword.message}</p>
                )}
              </div>
              <div className="space-y-2">
                <label className="text-sm font-medium text-surface-700 dark:text-surface-300">新密码</label>
                <Input
                  type="password"
                  placeholder="请输入新密码"
                  {...register('newPassword')}
                  disabled={isSubmitting}
                />
                {errors.newPassword && (
                  <p className="text-sm text-red-500">{errors.newPassword.message}</p>
                )}
              </div>
              <div className="space-y-2">
                <label className="text-sm font-medium text-surface-700 dark:text-surface-300">确认新密码</label>
                <Input
                  type="password"
                  placeholder="请再次输入新密码"
                  {...register('confirmNewPassword')}
                  disabled={isSubmitting}
                />
                {errors.confirmNewPassword && (
                  <p className="text-sm text-red-500">{errors.confirmNewPassword.message}</p>
                )}
              </div>

              {/* 提交状态提示 */}
              {submitStatus && (
                <div
                  className={`flex items-center gap-2 p-3 rounded-lg ${
                    submitStatus.type === 'success'
                      ? 'bg-green-50 text-green-700 dark:bg-green-900/20 dark:text-green-400'
                      : 'bg-red-50 text-red-700 dark:bg-red-900/20 dark:text-red-400'
                  }`}
                >
                  {submitStatus.type === 'success' ? (
                    <CheckCircle className="w-4 h-4" />
                  ) : (
                    <XCircle className="w-4 h-4" />
                  )}
                  <span className="text-sm">{submitStatus.message}</span>
                </div>
              )}

              <div className="pt-2">
                <Button type="submit" disabled={isSubmitting}>
                  {isSubmitting ? (
                    <>
                      <Loader2 className="w-4 h-4 mr-2 animate-spin" />
                      修改中...
                    </>
                  ) : (
                    '修改密码'
                  )}
                </Button>
              </div>
            </form>
          </CardContent>
        </Card>
      </div>

      <Modal
        isOpen={bindingModalOpen}
        onClose={() => setBindingModalOpen(false)}
        title="绑定西电账号"
        className="max-w-lg"
      >
        <div className="space-y-4">
          <div className="space-y-2">
            <label className="text-sm font-medium text-surface-700 dark:text-surface-300">学号/工号</label>
            <Input
              placeholder="请输入学号/工号"
              value={bindingForm.username}
              onChange={(event) => setBindingForm((prev) => ({ ...prev, username: event.target.value }))}
              disabled={bindingSubmitting}
            />
          </div>
          <div className="space-y-2">
            <label className="text-sm font-medium text-surface-700 dark:text-surface-300">密码</label>
            <Input
              type="password"
              placeholder="请输入密码"
              value={bindingForm.password}
              onChange={(event) => setBindingForm((prev) => ({ ...prev, password: event.target.value }))}
              disabled={bindingSubmitting}
            />
          </div>

          <div className="space-y-2">
            <div className="flex items-center justify-between">
              <span className="text-sm font-medium text-surface-700 dark:text-surface-300">滑块验证码</span>
              <Button variant="ghost" size="sm" onClick={handleRefreshCaptcha} disabled={captchaLoading || bindingSubmitting}>
                刷新
              </Button>
            </div>
            <div className="flex flex-col items-center gap-3">
              {captchaLoading && <span className="text-sm text-surface-500">验证码加载中...</span>}
              {!captchaLoading && captchaChallenge && (
                <>
                  <div
                    className="relative overflow-hidden rounded-lg border border-surface-200 dark:border-surface-700"
                    style={{ width: captchaChallenge.puzzle_width, height: captchaChallenge.puzzle_height }}
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
                    onChange={(event) => setSliderValue(Number(event.target.value) / 100)}
                    className="w-full"
                    disabled={bindingSubmitting}
                  />
                </>
              )}
            </div>
          </div>

          {bindingError && (
            <div className="flex items-center gap-2 rounded-lg bg-red-50 p-3 text-red-700 dark:bg-red-900/20 dark:text-red-400">
              <XCircle className="h-4 w-4" />
              <span className="text-sm">{bindingError}</span>
            </div>
          )}

          <div className="flex justify-end gap-2 pt-2">
            <Button variant="outline" onClick={() => setBindingModalOpen(false)} disabled={bindingSubmitting}>
              取消
            </Button>
            <Button onClick={handleCompleteBinding} disabled={bindingSubmitting || captchaLoading}>
              {bindingSubmitting ? '绑定中...' : '提交绑定'}
            </Button>
          </div>
        </div>
      </Modal>

      <Modal
        isOpen={emailBindModalOpen}
        onClose={() => setEmailBindModalOpen(false)}
        title={isEmailBound ? '换绑邮箱' : '绑定邮箱'}
        className="max-w-md"
      >
        <div className="space-y-4">
          <div className="space-y-2">
            <label className="text-sm font-medium text-surface-700 dark:text-surface-300">
              {isEmailBound ? '新邮箱地址' : '邮箱地址'}
            </label>
            <Input
              type="email"
              placeholder={isEmailBound ? '请输入新的邮箱地址' : '请输入要绑定的邮箱地址'}
              value={emailBindValue}
              onChange={(e) => setEmailBindValue(e.target.value)}
              disabled={emailBindSubmitting || emailCodeSent}
            />
          </div>
          {emailCodeSent && (
            <div className="space-y-2">
              <label className="text-sm font-medium text-surface-700 dark:text-surface-300">验证码</label>
              <Input
                placeholder="请输入 6 位验证码"
                value={emailCodeValue}
                onChange={(e) => setEmailCodeValue(e.target.value.replace(/\D/g, ''))}
                maxLength={6}
                disabled={emailBindSubmitting}
              />
            </div>
          )}
          {emailBindError && (
            <div className="flex items-center gap-2 rounded-lg bg-red-50 p-3 text-red-700 dark:bg-red-900/20 dark:text-red-400">
              <XCircle className="h-4 w-4 shrink-0" />
              <span className="text-sm">{emailBindError}</span>
            </div>
          )}
          {emailActionStatus && (
            <div
              className={`flex items-center gap-2 rounded-lg p-3 ${
                emailActionStatus.type === 'success'
                  ? 'bg-green-50 text-green-700 dark:bg-green-900/20 dark:text-green-400'
                  : 'bg-red-50 text-red-700 dark:bg-red-900/20 dark:text-red-400'
              }`}
            >
              {emailActionStatus.type === 'success' ? (
                <CheckCircle className="h-4 w-4 shrink-0" />
              ) : (
                <XCircle className="h-4 w-4 shrink-0" />
              )}
              <span className="text-sm">{emailActionStatus.message}</span>
            </div>
          )}
          <div className="flex justify-end gap-2 pt-2">
            <Button variant="outline" onClick={() => setEmailBindModalOpen(false)} disabled={emailBindSubmitting}>
              取消
            </Button>
            {emailCodeSent ? (
              <Button onClick={handleVerifyEmailCode} disabled={emailBindSubmitting}>
                {emailBindSubmitting ? '验证中...' : '确认验证'}
              </Button>
            ) : (
              <Button onClick={handleSendEmailCode} disabled={emailBindSubmitting}>
                {emailBindSubmitting ? '发送中...' : '发送验证码'}
              </Button>
            )}
          </div>
        </div>
      </Modal>
    </div>
  );
};
