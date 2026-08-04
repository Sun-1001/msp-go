import { useCallback, useEffect, useRef, useState } from 'react';
import { getApiErrorMessage } from '@/libs/http/apiClient';
import {
  fetchReviewTasks,
  type PaginationInfo,
  type ReviewTask,
  type ReviewTaskCounts,
  type ReviewTaskView,
} from '@/modules/mistake/services/mistakeService';
import type { LoadingState } from '@/types';

const initialPagination: PaginationInfo = {
  page: 1,
  pageSize: 20,
  total: 0,
  totalPages: 0,
};

const initialCounts: ReviewTaskCounts = {
  active: 0,
  dueNow: 0,
  mastered: 0,
};

export function useMistakeReviewTasks(
  view: ReviewTaskView,
  requestedPage = 1,
) {
  const [tasks, setTasks] = useState<ReviewTask[]>([]);
  const [pagination, setPagination] = useState<PaginationInfo>(initialPagination);
  const [counts, setCounts] = useState<ReviewTaskCounts>(initialCounts);
  const [tasksLoading, setTasksLoading] = useState<LoadingState>('idle');
  const [tasksError, setTasksError] = useState<string | null>(null);
  const [resolvedRequestKey, setResolvedRequestKey] = useState('');
  const [reloadVersion, setReloadVersion] = useState(0);
  const requestIdRef = useRef(0);
  const page = Math.max(1, requestedPage);
  const requestKey = `${view}\u0000${page}`;

  useEffect(() => {
    const requestId = requestIdRef.current + 1;
    requestIdRef.current = requestId;
    const controller = new AbortController();

    const loadTasks = async () => {
      setTasksLoading('loading');
      setTasksError(null);
      try {
        const response = await fetchReviewTasks(
          { view, page, pageSize: 20 },
          controller.signal
        );
        if (controller.signal.aborted || requestIdRef.current !== requestId) return;
        const lastPage = Math.max(response.pagination.totalPages, 1);
        if (page > lastPage) {
          setTasks([]);
          setPagination({ ...response.pagination, page: lastPage });
          setCounts(response.counts);
          setResolvedRequestKey(requestKey);
          setTasksLoading('success');
          return;
        }
        setTasks(response.items);
        setPagination(response.pagination);
        setCounts(response.counts);
        setResolvedRequestKey(requestKey);
        setTasksLoading('success');
      } catch (error) {
        if (controller.signal.aborted || requestIdRef.current !== requestId) return;
        setTasksLoading('error');
        setTasksError(getApiErrorMessage(error, '获取复习任务失败'));
      }
    };

    void loadTasks();

    return () => controller.abort();
  }, [page, reloadVersion, requestKey, view]);

  const reloadTasks = useCallback(() => {
    setReloadVersion((version) => version + 1);
  }, []);

  return {
    tasks,
    pagination,
    counts,
    tasksLoading,
    tasksError,
    resolvedRequestKey,
    reloadTasks,
  };
}
