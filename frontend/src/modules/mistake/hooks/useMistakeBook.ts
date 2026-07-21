import { useCallback, useEffect } from 'react';
import { useAppDispatch, useAppSelector } from '@/store';
import { fetchPortrait, generatePortrait, clearPortrait } from '@/modules/student/store/studentPortraitSlice';
import {
  fetchMistakes,
  deleteMistake,
  markAsMastered,
  selectMistakes,
  selectPagination,
  selectLoadingState,
  selectError,
} from '@/modules/mistake/store/mistakeSlice';

export function getDifficultyBadge(difficulty: number) {
  if (difficulty >= 0.7) return { variant: 'destructive' as const, label: '困难' };
  if (difficulty >= 0.4) return { variant: 'warning' as const, label: '中等' };
  return { variant: 'success' as const, label: '简单' };
}

export function getErrorTypeLabel(errorType: string | null) {
  const labels: Record<string, string> = {
    conceptual: '概念性错误',
    procedural: '过程性错误',
    logical: '逻辑错误',
    symbolic: '符号错误',
    calculation: '计算错误',
  };
  return errorType ? labels[errorType] || '未知错误' : '未分类';
}

export function useMistakeBook() {
  const dispatch = useAppDispatch();
  const { portrait, loadingState: portraitLoading, generating, clearing } = useAppSelector(
    (state) => state.studentPortrait,
  );
  const mistakes = useAppSelector(selectMistakes);
  const pagination = useAppSelector(selectPagination);
  const mistakesLoading = useAppSelector(selectLoadingState);
  const mistakesError = useAppSelector(selectError);

  useEffect(() => {
    dispatch(fetchMistakes({ page: 1, pageSize: 20 }));
  }, [dispatch]);

  const handleTabChange = useCallback((value: string) => {
    if (value === 'mistakes') {
      dispatch(fetchMistakes({ page: 1, pageSize: 20 }));
    } else if (value === 'portrait') {
      dispatch(fetchPortrait());
    }
  }, [dispatch]);

  const handleDeleteMistake = useCallback(async (attemptId: string) => {
    if (window.confirm('确定要删除这条错题记录吗？删除后无法恢复。')) {
      await dispatch(deleteMistake(attemptId));
    }
  }, [dispatch]);

  const handleMarkAsMastered = useCallback(async (attemptId: string) => {
    await dispatch(markAsMastered(attemptId));
  }, [dispatch]);

  const handleFetchMistakes = useCallback((page: number) => {
    dispatch(fetchMistakes({ page, pageSize: 20 }));
  }, [dispatch]);

  const handleGeneratePortrait = useCallback(() => {
    dispatch(generatePortrait());
  }, [dispatch]);

  const handleClearPortrait = useCallback(() => {
    if (window.confirm('确定要清除画像吗？清除后需要重新生成。')) {
      dispatch(clearPortrait());
    }
  }, [dispatch]);

  return {
    mistakes,
    pagination,
    mistakesLoading,
    mistakesError,
    portrait,
    portraitLoading,
    generating,
    clearing,
    handleTabChange,
    handleDeleteMistake,
    handleMarkAsMastered,
    handleFetchMistakes,
    handleGeneratePortrait,
    handleClearPortrait,
  };
}
