/**
 * Exercise 模块 - 练习题
 */

// Components
export { ExercisePanel } from './components/ExercisePanel';
export { EmptyExerciseState } from './components/EmptyExerciseState';
export { AIPracticeConfigurator } from './components/AIPracticeConfigurator';
export type { AIPracticeConfiguratorProps } from './components/AIPracticeConfigurator';

// Hooks / ViewModels
export { useExerciseViewModel } from './hooks/exerciseViewModel';

// Services
export { default as exerciseService } from './services/exerciseService';
export type { Question } from './services/exerciseService';

// Store
export { default as exerciseReducer } from './store/exerciseSlice';
