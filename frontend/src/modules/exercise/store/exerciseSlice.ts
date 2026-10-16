import { createSlice, type PayloadAction } from '@reduxjs/toolkit';
import type { Exercise, DifficultyLevel, LoadingState } from '@/types';
import { createFieldSelector, createLoadingReducers, type WithLoadingState } from '@/store/utils/sliceFactory';

/**
 * 练习题反馈
 */
export interface ExerciseFeedback {
  correct: boolean;
  explanation: string;
  hints?: string[];
  relatedConcepts?: string[];
}

/**
 * 练习题尝试记录
 */
export interface ExerciseAttempt {
  exerciseId: string;
  answer: string;
  feedback: ExerciseFeedback;
  timestamp: string;
}

/**
 * 练习题状态
 */
export interface ExerciseState extends WithLoadingState {
  currentExercise: Exercise | null;
  currentAnswer: string;
  feedback: ExerciseFeedback | null;
  history: ExerciseAttempt[];
  filters: {
    difficulty?: DifficultyLevel;
    conceptId?: string;
  };
  submitState: LoadingState;
}

const initialState: ExerciseState = {
  currentExercise: null,
  currentAnswer: '',
  feedback: null,
  history: [],
  filters: {},
  loadingState: 'idle',
  submitState: 'idle',
  error: null,
};

const exerciseSlice = createSlice({
  name: 'exercise',
  initialState,
  reducers: {
    // 使用工厂函数创建通用加载状态 reducers (DRY 原则)
    ...createLoadingReducers<ExerciseState>(),

    // 设置提交状态
    setSubmitState(state, action: PayloadAction<LoadingState>) {
      state.submitState = action.payload;
      if (action.payload === 'loading') {
        state.error = null;
      }
    },

    // 设置当前练习题
    setCurrentExercise(state, action: PayloadAction<Exercise>) {
      state.currentExercise = action.payload;
      state.currentAnswer = '';
      state.feedback = null;
      state.error = null;
    },

    // 更新用户答案
    setCurrentAnswer(state, action: PayloadAction<string>) {
      state.currentAnswer = action.payload;
    },

    // 设置反馈信息
    setFeedback(state, action: PayloadAction<ExerciseFeedback>) {
      state.feedback = action.payload;

      // 添加到历史记录
      if (state.currentExercise) {
        state.history.push({
          exerciseId: state.currentExercise.id,
          answer: state.currentAnswer,
          feedback: action.payload,
          timestamp: new Date().toISOString(),
        });
      }
    },

    // 清除当前练习
    clearCurrentExercise(state) {
      state.currentExercise = null;
      state.currentAnswer = '';
      state.feedback = null;
      state.error = null;
    },

    // 设置筛选条件
    setFilters(state, action: PayloadAction<ExerciseState['filters']>) {
      state.filters = action.payload;
    },

    // 清除历史记录
    clearHistory(state) {
      state.history = [];
    },

    // 重置状态
    resetExerciseState() {
      return initialState;
    },
  },
});

// 导出所有 actions（包括工厂函数生成的）
export const {
  setLoadingState,
  setError,
  clearError,
  setSubmitState,
  setCurrentExercise,
  setCurrentAnswer,
  setFeedback,
  clearCurrentExercise,
  setFilters,
  clearHistory,
  resetExerciseState,
} = exerciseSlice.actions;

// ============ Selectors ============
// 使用工厂函数生成字段 selectors
export const selectCurrentExercise = createFieldSelector<ExerciseState, 'exercise', 'currentExercise'>('exercise', 'currentExercise');
export const selectCurrentAnswer = createFieldSelector<ExerciseState, 'exercise', 'currentAnswer'>('exercise', 'currentAnswer');
export const selectFeedback = createFieldSelector<ExerciseState, 'exercise', 'feedback'>('exercise', 'feedback');
export const selectExerciseHistory = createFieldSelector<ExerciseState, 'exercise', 'history'>('exercise', 'history');
export const selectExerciseFilters = createFieldSelector<ExerciseState, 'exercise', 'filters'>('exercise', 'filters');
export const selectExerciseLoadingState = createFieldSelector<ExerciseState, 'exercise', 'loadingState'>('exercise', 'loadingState');
export const selectExerciseSubmitState = createFieldSelector<ExerciseState, 'exercise', 'submitState'>('exercise', 'submitState');
export const selectExerciseError = createFieldSelector<ExerciseState, 'exercise', 'error'>('exercise', 'error');

// 派生 selectors
export const selectIsExerciseLoading = (state: { exercise: ExerciseState }) =>
  state.exercise.loadingState === 'loading';

export const selectIsExerciseSubmitting = (state: { exercise: ExerciseState }) =>
  state.exercise.submitState === 'loading';

export default exerciseSlice.reducer;
