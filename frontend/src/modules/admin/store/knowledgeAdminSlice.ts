/**
 * 知识点管理 Redux Slice
 *
 * 管理知识节点和关系的 CRUD 操作状态
 * 遵循 DRY、KISS 原则，使用工厂函数消除重复代码
 */

import { createSlice, type PayloadAction } from '@reduxjs/toolkit';
import { createLoadingReducers, type WithLoadingState } from '@/store/utils/sliceFactory';
import { createSafeThunk } from '@/store/utils/thunkFactory';
import { knowledgeAdminService } from '@/modules/admin/services/knowledgeAdminService';
import type {
  KnowledgeNodeAdmin,
  KnowledgeRelationAdmin,
  KnowledgeStats,
  SimpleNode,
  KnowledgeNodeCreateData,
  KnowledgeNodeUpdateData,
  KnowledgeRelationCreateData,
  KnowledgeRelationUpdateData,
} from '@/modules/admin/types/knowledgeAdmin';

/**
 * 知识点管理状态接口
 */
export interface KnowledgeAdminState extends WithLoadingState {
  // Tab 状态
  activeTab: 'nodes' | 'relations' | 'graph';

  // 统计数据
  stats: KnowledgeStats | null;
  statsLoading: boolean;

  // 节点数据
  nodes: KnowledgeNodeAdmin[];
  nodesLoading: boolean;
  nodesError: string | null;
  nodePage: number;
  nodeTotalPages: number;
  nodeTotal: number;

  // 节点筛选
  searchTerm: string;
  chapterFilter: string;
  typeFilter: string;
  chapters: string[];

  // 关系数据
  relations: KnowledgeRelationAdmin[];
  relationsLoading: boolean;
  allNodes: SimpleNode[];
  allNodesLoading: boolean;
  allNodesError: string | null;

  // 模态框状态
  nodeModalOpen: boolean;
  editingNode: KnowledgeNodeAdmin | null;
  relationModalOpen: boolean;
  editingRelation: KnowledgeRelationAdmin | null;
  deleteConfirm: { type: 'node' | 'relation'; id: string; name: string } | null;
  saving: boolean;
}

const initialState: KnowledgeAdminState = {
  activeTab: 'nodes',
  stats: null,
  statsLoading: false,
  nodes: [],
  nodesLoading: false,
  nodesError: null,
  nodePage: 1,
  nodeTotalPages: 1,
  nodeTotal: 0,
  searchTerm: '',
  chapterFilter: '',
  typeFilter: '',
  chapters: [],
  relations: [],
  relationsLoading: false,
  allNodes: [],
  allNodesLoading: false,
  allNodesError: null,
  nodeModalOpen: false,
  editingNode: null,
  relationModalOpen: false,
  editingRelation: null,
  deleteConfirm: null,
  saving: false,
  loadingState: 'idle',
  error: null,
};

// ========== Async Thunks ==========

/**
 * 获取统计数据
 */
export const fetchStats = createSafeThunk(
  'knowledgeAdmin/fetchStats',
  async () => await knowledgeAdminService.getStats(),
  '获取统计数据失败'
);

/**
 * 获取章节列表
 */
export const fetchChapters = createSafeThunk(
  'knowledgeAdmin/fetchChapters',
  async () => await knowledgeAdminService.getChapters(),
  '获取章节列表失败'
);

/**
 * 获取节点列表
 */
export const fetchNodes = createSafeThunk(
  'knowledgeAdmin/fetchNodes',
  async (params: {
    page?: number;
    page_size?: number;
    chapter?: string;
    type?: string;
    search?: string;
  }) => await knowledgeAdminService.listNodes(params),
  '获取节点列表失败'
);

/**
 * 获取关系列表
 */
export const fetchRelations = createSafeThunk(
  'knowledgeAdmin/fetchRelations',
  async (nodeId?: string) => await knowledgeAdminService.listRelations(nodeId),
  '获取关系列表失败'
);

/**
 * 获取所有节点简要信息
 */
export const fetchAllNodesSimple = createSafeThunk(
  'knowledgeAdmin/fetchAllNodesSimple',
  async () => await knowledgeAdminService.getAllNodesSimple(),
  '获取节点列表失败'
);

/**
 * 创建节点
 */
export const createNode = createSafeThunk(
  'knowledgeAdmin/createNode',
  async (data: KnowledgeNodeCreateData) => await knowledgeAdminService.createNode(data),
  '创建节点失败'
);

/**
 * 更新节点
 */
export const updateNode = createSafeThunk(
  'knowledgeAdmin/updateNode',
  async ({ nodeId, data }: { nodeId: string; data: KnowledgeNodeUpdateData }) =>
    await knowledgeAdminService.updateNode(nodeId, data),
  '更新节点失败'
);

/**
 * 删除节点
 */
export const deleteNode = createSafeThunk(
  'knowledgeAdmin/deleteNode',
  async (nodeId: string) => await knowledgeAdminService.deleteNode(nodeId),
  '删除节点失败'
);

/**
 * 创建关系
 */
export const createRelation = createSafeThunk(
  'knowledgeAdmin/createRelation',
  async (data: KnowledgeRelationCreateData) => await knowledgeAdminService.createRelation(data),
  '创建关系失败'
);

/**
 * 更新关系
 */
export const updateRelation = createSafeThunk(
  'knowledgeAdmin/updateRelation',
  async ({ relationId, data }: { relationId: string; data: KnowledgeRelationUpdateData }) =>
    await knowledgeAdminService.updateRelation(relationId, data),
  '更新关系失败'
);

/**
 * 删除关系
 */
export const deleteRelation = createSafeThunk(
  'knowledgeAdmin/deleteRelation',
  async (relationId: string) => await knowledgeAdminService.deleteRelation(relationId),
  '删除关系失败'
);

// ========== Slice ==========

const knowledgeAdminSlice = createSlice({
  name: 'knowledgeAdmin',
  initialState,
  reducers: {
    // 使用工厂函数创建通用加载状态 reducers (DRY 原则)
    ...createLoadingReducers<KnowledgeAdminState>(),

    // Tab 切换
    setActiveTab(state, action: PayloadAction<'nodes' | 'relations' | 'graph'>) {
      state.activeTab = action.payload;
    },

    // 节点筛选
    setSearchTerm(state, action: PayloadAction<string>) {
      state.searchTerm = action.payload;
      state.nodePage = 1; // 重置页码
    },

    setChapterFilter(state, action: PayloadAction<string>) {
      state.chapterFilter = action.payload;
      state.nodePage = 1; // 重置页码
    },

    setTypeFilter(state, action: PayloadAction<string>) {
      state.typeFilter = action.payload;
      state.nodePage = 1; // 重置页码
    },

    setNodePage(state, action: PayloadAction<number>) {
      state.nodePage = action.payload;
    },

    // 模态框控制
    openNodeModal(state, action: PayloadAction<KnowledgeNodeAdmin | null>) {
      state.nodeModalOpen = true;
      state.editingNode = action.payload;
    },

    closeNodeModal(state) {
      state.nodeModalOpen = false;
      state.editingNode = null;
    },

    openRelationModal(state, action: PayloadAction<KnowledgeRelationAdmin | null>) {
      state.relationModalOpen = true;
      state.editingRelation = action.payload;
    },

    closeRelationModal(state) {
      state.relationModalOpen = false;
      state.editingRelation = null;
    },

    // 删除确认
    setDeleteConfirm(
      state,
      action: PayloadAction<{ type: 'node' | 'relation'; id: string; name: string } | null>
    ) {
      state.deleteConfirm = action.payload;
    },

    // 清空状态
    resetKnowledgeAdmin(state) {
      Object.assign(state, initialState);
    },
  },
  extraReducers: (builder) => {
    // ========== 统计数据 ==========
    builder
      .addCase(fetchStats.pending, (state) => {
        state.statsLoading = true;
      })
      .addCase(fetchStats.fulfilled, (state, action) => {
        state.statsLoading = false;
        state.stats = action.payload;
      })
      .addCase(fetchStats.rejected, (state) => {
        state.statsLoading = false;
      });

    // ========== 章节列表 ==========
    builder
      .addCase(fetchChapters.fulfilled, (state, action) => {
        state.chapters = action.payload;
      });

    // ========== 节点列表 ==========
    builder
      .addCase(fetchNodes.pending, (state) => {
        state.nodesLoading = true;
        state.nodesError = null;
      })
      .addCase(fetchNodes.fulfilled, (state, action) => {
        state.nodesLoading = false;
        state.nodes = action.payload.items;
        state.nodeTotal = action.payload.total;
        state.nodeTotalPages = action.payload.total_pages;
      })
      .addCase(fetchNodes.rejected, (state, action) => {
        state.nodesLoading = false;
        state.nodesError = action.payload || '获取节点列表失败';
      });

    // ========== 关系列表 ==========
    builder
      .addCase(fetchRelations.pending, (state) => {
        state.relationsLoading = true;
      })
      .addCase(fetchRelations.fulfilled, (state, action) => {
        state.relationsLoading = false;
        state.relations = action.payload.items;
      })
      .addCase(fetchRelations.rejected, (state) => {
        state.relationsLoading = false;
      });

    // ========== 所有节点简要信息 ==========
    builder
      .addCase(fetchAllNodesSimple.pending, (state) => {
        state.allNodesLoading = true;
        state.allNodesError = null;
      })
      .addCase(fetchAllNodesSimple.fulfilled, (state, action) => {
        state.allNodesLoading = false;
        state.allNodes = action.payload;
      })
      .addCase(fetchAllNodesSimple.rejected, (state, action) => {
        state.allNodesLoading = false;
        state.allNodesError = action.payload || '获取节点列表失败';
      });

    // ========== 创建节点 ==========
    builder
      .addCase(createNode.pending, (state) => {
        state.saving = true;
      })
      .addCase(createNode.fulfilled, (state) => {
        state.saving = false;
        state.nodeModalOpen = false;
        state.editingNode = null;
      })
      .addCase(createNode.rejected, (state) => {
        state.saving = false;
      });

    // ========== 更新节点 ==========
    builder
      .addCase(updateNode.pending, (state) => {
        state.saving = true;
      })
      .addCase(updateNode.fulfilled, (state) => {
        state.saving = false;
        state.nodeModalOpen = false;
        state.editingNode = null;
      })
      .addCase(updateNode.rejected, (state) => {
        state.saving = false;
      });

    // ========== 删除节点 ==========
    builder
      .addCase(deleteNode.pending, (state) => {
        state.saving = true;
      })
      .addCase(deleteNode.fulfilled, (state) => {
        state.saving = false;
        state.deleteConfirm = null;
      })
      .addCase(deleteNode.rejected, (state) => {
        state.saving = false;
      });

    // ========== 创建关系 ==========
    builder
      .addCase(createRelation.pending, (state) => {
        state.saving = true;
      })
      .addCase(createRelation.fulfilled, (state) => {
        state.saving = false;
        state.relationModalOpen = false;
        state.editingRelation = null;
      })
      .addCase(createRelation.rejected, (state) => {
        state.saving = false;
      });

    // ========== 更新关系 ==========
    builder
      .addCase(updateRelation.pending, (state) => {
        state.saving = true;
      })
      .addCase(updateRelation.fulfilled, (state) => {
        state.saving = false;
        state.relationModalOpen = false;
        state.editingRelation = null;
      })
      .addCase(updateRelation.rejected, (state) => {
        state.saving = false;
      });

    // ========== 删除关系 ==========
    builder
      .addCase(deleteRelation.pending, (state) => {
        state.saving = true;
      })
      .addCase(deleteRelation.fulfilled, (state) => {
        state.saving = false;
        state.deleteConfirm = null;
      })
      .addCase(deleteRelation.rejected, (state) => {
        state.saving = false;
      });
  },
});

export const {
  setLoadingState,
  setError,
  clearError,
  setActiveTab,
  setSearchTerm,
  setChapterFilter,
  setTypeFilter,
  setNodePage,
  openNodeModal,
  closeNodeModal,
  openRelationModal,
  closeRelationModal,
  setDeleteConfirm,
  resetKnowledgeAdmin,
} = knowledgeAdminSlice.actions;

export default knowledgeAdminSlice.reducer;
