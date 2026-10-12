import React from 'react';
import { RequestErrorNotice } from '@/components/feedback';
import { Button } from '@/components/ui/Button';
import { Card, CardContent } from '@/components/ui/Card';
import { Input } from '@/components/ui/Input';
import { useToast } from '@/components/ui/Toast';
import { ArrowLeft, Loader2, Search, X } from 'lucide-react';
import { cn } from '@/libs/utils/cn';
import { formatRelativeTime } from '@/libs/utils/dateFormat';
import { MessageCenterListPagination } from '@/modules/message-center/MessageCenterListPagination';
import { MessageAttachments } from '@/modules/message-center/MessageAttachments';
import { MessageText } from '@/modules/message-center/MessageText';
import { MessageComposer } from '@/modules/message-center/MessageComposer';
import type { usePrivateConversationDetail } from '@/modules/message-center/components/usePrivateConversationDetail';

interface ConversationDetailPanelProps {
  conversation: ReturnType<typeof usePrivateConversationDetail>;
  viewerRole: 'student' | 'teacher';
  counterpartName: string;
  counterpartSubtitle: string;
  composerPlaceholder: string;
  sendLabel?: string;
  onBack: () => void;
}

export const ConversationDetailPanel: React.FC<ConversationDetailPanelProps> = ({
  conversation,
  viewerRole,
  counterpartName,
  counterpartSubtitle,
  composerPlaceholder,
  sendLabel,
  onBack,
}) => {
  const { toast } = useToast();
  const {
    activeConv,
    activeConvId,
    conversationDetailLoading,
    conversationDetailError,
    messageDraft,
    setMessageDraft,
    messageAttachments,
    setMessageAttachments,
    messageUploading,
    setMessageUploading,
    sendingMsg,
    loadingOlderMessages,
    conversationViewportRef,
    conversationSearchOpen,
    conversationMessageSearch,
    setConversationMessageSearch,
    conversationSearch,
    loadingConversationSearch,
    conversationSearchError,
    visibleConversationMessages,
    loadConversationDetail,
    sendPrivateMessage,
    loadOlderConversationMessages,
    toggleConversationSearch,
  } = conversation;
  const readLabel = viewerRole === 'student' ? '老师' : '学生';

  return (
    <Card className={cn('min-h-0 overflow-hidden rounded-2xl border-surface-200/80 shadow-sm dark:border-surface-700', !activeConvId ? 'hidden lg:block' : '')}>
      <CardContent className="flex h-full min-h-0 flex-col p-0">
        {conversationDetailLoading ? (
          <div className="flex min-h-64 flex-col items-center justify-center gap-3 p-6">
            <Loader2 className="h-6 w-6 animate-spin text-primary-500" />
            <Button variant="ghost" size="sm" className="lg:hidden" onClick={onBack}><ArrowLeft className="mr-1 h-4 w-4" />返回列表</Button>
          </div>
        ) : conversationDetailError ? (
          <div className="flex min-h-64 flex-col items-center justify-center gap-3 p-6 text-center text-sm text-surface-500 dark:text-surface-400">
            <RequestErrorNotice
              error={conversationDetailError}
              onRetry={() => activeConvId && void loadConversationDetail(activeConvId)}
              onRefresh={() => activeConvId && void loadConversationDetail(activeConvId)}
              className="w-full max-w-lg text-left"
            />
            <Button variant="ghost" size="sm" className="lg:hidden" onClick={onBack}><ArrowLeft className="mr-1 h-4 w-4" />返回列表</Button>
          </div>
        ) : activeConv && activeConv.id === activeConvId ? (
          <>
            <div className="shrink-0 border-b border-surface-100 p-3 sm:p-4 dark:border-surface-800">
              <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
                <div>
                  <Button variant="ghost" size="sm" className="mb-2 -ml-2 lg:hidden" onClick={onBack}><ArrowLeft className="mr-1 h-4 w-4" />返回列表</Button>
                  <div className="text-lg font-semibold text-surface-900 dark:text-surface-100">
                    {counterpartName}
                  </div>
                  <div className="text-sm text-surface-500 dark:text-surface-400">{counterpartSubtitle}</div>
                </div>
                <div className="flex w-full items-center justify-end gap-2 sm:w-auto">
                  {conversationSearchOpen && (
                    <div role="search" className="relative min-w-0 flex-1 sm:w-64 sm:flex-none">
                      <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-surface-400" />
                      <Input
                        autoFocus
                        value={conversationMessageSearch}
                        maxLength={200}
                        onChange={(event) => setConversationMessageSearch(event.target.value)}
                        placeholder={`查找与 ${counterpartName} 的聊天内容`}
                        className={cn('pl-9 pr-16', conversationSearchError && 'border-amber-500 focus-visible:ring-amber-500')}
                      />
                      <span
                        title={conversationSearchError || undefined}
                        className={cn('pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 text-xs text-surface-500 dark:text-surface-400', conversationSearchError && 'text-amber-600 dark:text-amber-400')}
                      >
                        {conversationSearchError ? '加载失败' : loadingConversationSearch ? '载入中…' : conversationMessageSearch.trim() ? `${conversationSearch.total} 条` : `共 ${activeConv.messages_total} 条`}
                      </span>
                    </div>
                  )}
                  <Button variant="outline" size="sm" className="shrink-0" onClick={toggleConversationSearch}>
                    {loadingConversationSearch ? <Loader2 className="h-4 w-4 animate-spin sm:mr-2" /> : conversationSearchOpen ? <X className="h-4 w-4 sm:mr-2" /> : <Search className="mr-2 h-4 w-4" />}
                    <span className={conversationSearchOpen ? 'hidden sm:inline' : ''}>{loadingConversationSearch ? '载入记录' : conversationSearchOpen ? '关闭查找' : '查找聊天记录'}</span>
                  </Button>
                </div>
              </div>
            </div>
            <div ref={conversationViewportRef} className="min-h-0 flex-1 space-y-4 overflow-y-auto overscroll-contain p-5">
              {!conversationSearchOpen && activeConv.messages.length < activeConv.messages_total && <Button variant="outline" size="sm" className="w-full" onClick={loadOlderConversationMessages} disabled={loadingOlderMessages}>{loadingOlderMessages ? '加载中…' : '加载更早消息'}</Button>}
              {conversationSearch.error && <RequestErrorNotice error={conversationSearch.error} onRetry={conversationSearch.refresh} onRefresh={conversationSearch.refresh} />}
              {conversationSearchOpen && conversationMessageSearch.trim() && !loadingConversationSearch && !conversationSearch.error && visibleConversationMessages.length === 0 && (
                <div className="flex min-h-32 items-center justify-center text-sm text-surface-500 dark:text-surface-400">未找到匹配的聊天记录</div>
              )}
              {visibleConversationMessages.map((msg) => (
                <div key={msg.id} className="flex w-full">
                  <div className={cn('max-w-[80%]', msg.from === viewerRole ? 'ml-auto text-right' : 'mr-auto')}>
                    {msg.text && <div className={cn(
                      'inline-block rounded-lg px-4 py-3 text-sm',
                      msg.from === viewerRole
                        ? 'bg-primary-600 text-white'
                        : 'bg-surface-100 text-surface-800 dark:bg-surface-800 dark:text-surface-100',
                    )}>
                      <MessageText text={msg.text} />
                    </div>}
                    <MessageAttachments attachments={msg.attachments} />
                    <div data-read-message-id={msg.from !== viewerRole ? msg.id : undefined} className={cn('mt-1 flex gap-2 text-xs text-surface-400', msg.from === viewerRole ? 'justify-end' : 'justify-start')}>
                      <span>{formatRelativeTime(msg.time)}</span>
                      {msg.from === viewerRole && <span>{msg.read_by_recipient ? `${readLabel}已读` : `${readLabel}未读`}</span>}
                    </div>
                  </div>
                </div>
              ))}
              {conversationSearchOpen && conversationMessageSearch.trim() && (
                <MessageCenterListPagination
                  currentPage={conversationSearch.page}
                  totalItems={conversationSearch.total}
                  pageSize={conversationSearch.pageSize}
                  disabled={loadingConversationSearch}
                  onPageChange={conversationSearch.setPage}
                />
              )}
            </div>
            <div className="shrink-0 border-t border-surface-100 p-4 dark:border-surface-800">
              <MessageComposer
                key={activeConvId}
                value={messageDraft}
                onChange={setMessageDraft}
                attachments={messageAttachments}
                onAttachmentsChange={setMessageAttachments}
                onUploadingChange={setMessageUploading}
                onError={(message) => toast({ type: 'error', title: message })}
                onFeedback={(feedback) => toast(feedback)}
                onSend={sendPrivateMessage}
                placeholder={composerPlaceholder}
                sendLabel={sendLabel}
                disabled={sendingMsg}
                uploading={messageUploading}
                sending={sendingMsg}
              />
            </div>
          </>
        ) : (
          <div className="flex h-full items-center justify-center p-8 text-sm text-surface-500 dark:text-surface-400">
            {activeConvId ? '暂无可显示的私信对话' : '请选择联系人查看私信'}
          </div>
        )}
      </CardContent>
    </Card>
  );
};
