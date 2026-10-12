import { configureStore, createListenerMiddleware } from '@reduxjs/toolkit';
import { useDispatch, useSelector } from 'react-redux';

// 各模块的 reducer 导入
import authReducer, { logout, setCredentials } from '@/modules/auth/store/authSlice';
import uiReducer from './slices/uiSlice'; // 全局 UI 状态，保留在 store 层
import exerciseReducer from '@/modules/exercise/store/exerciseSlice';
import sessionReducer from '@/modules/session/store/sessionSlice';
import mistakeReducer from '@/modules/mistake/store/mistakeSlice';
import aiConfigReducer from '@/modules/ai-config/store/aiConfigSlice';
import adminStatsReducer from '@/modules/admin/store/adminStatsSlice';
import securityLogReducer from '@/modules/admin/store/securityLogSlice';
import resourceReducer, { resourceApi, resetState as resetResourceState, selectResourceScope } from '@/modules/resource/store/resourceSlice';
import studentPortraitReducer from '@/modules/student/store/studentPortraitSlice';
import knowledgeReducer from '@/modules/knowledge/store/knowledgeSlice';
import knowledgeAdminReducer from '@/modules/admin/store/knowledgeAdminSlice';

const listenerMiddleware = createListenerMiddleware();

/**
 * Redux Store 配置
 *
 * 包含的 Slice：
 * - auth: 认证状态（token、用户信息、登录状态）
 * - ui: UI 状态（主题、侧边栏等）
 * - exercise: 练习题状态（当前题目、答案、反馈、历史记录）
 * - session: 会话状态（当前会话、消息列表、聊天模式）
 * - mistake: 错题本状态（错题列表、统计、详情、筛选）
 * - aiConfig: AI 配置状态（提供商、模型、智能体配置）
 * - adminStats: 管理员统计状态（概览、用户增长、系统状态）
 * - securityLog: 安全日志状态（日志列表、统计、筛选）
 * - resourceApi: 资源查询缓存（列表、统计、收藏和增删改）
 * - studentPortrait: 学生画像状态（画像内容、生成状态）
 * - knowledge: 知识图谱状态（节点、边、统计、筛选）
 * - knowledgeAdmin: 知识点管理状态（节点、关系 CRUD、筛选、分页）
 */
export const store = configureStore({
  reducer: {
    auth: authReducer,
    ui: uiReducer,
    exercise: exerciseReducer,
    session: sessionReducer,
    mistake: mistakeReducer,
    aiConfig: aiConfigReducer,
    adminStats: adminStatsReducer,
    securityLog: securityLogReducer,
    [resourceApi.reducerPath]: resourceReducer,
    studentPortrait: studentPortraitReducer,
    knowledge: knowledgeReducer,
    knowledgeAdmin: knowledgeAdminReducer,
  },
  middleware: (getDefaultMiddleware) => getDefaultMiddleware()
    .prepend(listenerMiddleware.middleware)
    .concat(resourceApi.middleware),
});

export type RootState = ReturnType<typeof store.getState>;
export type AppDispatch = typeof store.dispatch;

listenerMiddleware.startListening.withTypes<RootState, AppDispatch>()({
  predicate: (action, current, previous) => logout.match(action)
    || setCredentials.match(action)
    || selectResourceScope(current) !== selectResourceScope(previous),
  effect: (_action, { dispatch }) => {
    const requests = [
      ...dispatch(resourceApi.util.getRunningQueriesThunk()),
      ...dispatch(resourceApi.util.getRunningMutationsThunk()),
    ];
    requests.forEach((request) => request.abort());
    dispatch(resetResourceState());
  },
});

/**
 * 类型安全的 Redux Hooks
 *
 * 使用这些 hooks 代替原生的 useDispatch 和 useSelector，
 * 可以获得完整的类型推断支持。
 *
 * @example
 * ```tsx
 * const dispatch = useAppDispatch();
 * const user = useAppSelector(selectCurrentUser);
 * ```
 */
export const useAppDispatch = useDispatch.withTypes<AppDispatch>();
export const useAppSelector = useSelector.withTypes<RootState>();
