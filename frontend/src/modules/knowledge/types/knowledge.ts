/**
 * 知识图谱相关类型定义
 */

/**
 * 知识节点类型
 */
export type KnowledgeNodeType = 'concept' | 'theorem' | 'method';

/**
 * 知识关系类型
 */
export type KnowledgeRelationType = 'prerequisite' | 'used_in' | 'related';

/**
 * 用户选择的图谱视图模式。auto 会根据设备能力和数据规模解析。
 */
export type KnowledgeGraphViewMode = 'auto' | '3d' | '2d' | 'list';

/**
 * 实际启用的图谱渲染器。
 */
export type ResolvedKnowledgeGraphViewMode = Exclude<KnowledgeGraphViewMode, 'auto'>;

/**
 * 页面学习视角。
 */
export type KnowledgeGraphExperienceMode = 'explore' | 'path';

/**
 * 知识节点
 */
export interface KnowledgeNode {
  id: string;
  label: string;
  type: KnowledgeNodeType;
  mastery: number; // 0-1
  chapter?: string;
  description?: string;
  formula?: string;
}

/**
 * 知识关系边
 */
export interface KnowledgeEdge {
  source: string;
  target: string;
  relation: KnowledgeRelationType;
}

/**
 * 知识图谱统计信息
 */
export interface KnowledgeGraphStatistics {
  total_nodes: number;
  mastered_nodes: number;
  overall_mastery: number;
}

/**
 * 知识图谱数据
 */
export interface KnowledgeGraphData {
  nodes: KnowledgeNode[];
  edges: KnowledgeEdge[];
  statistics: KnowledgeGraphStatistics;
}

/**
 * 知识图谱筛选条件
 */
export interface KnowledgeGraphFilters {
  chapter?: string;
  type?: KnowledgeNodeType;
  search?: string;
}

/**
 * 知识图谱 API 响应
 */
export interface KnowledgeGraphResponse {
  nodes: KnowledgeNode[];
  edges: KnowledgeEdge[];
  statistics: KnowledgeGraphStatistics;
}

/**
 * 当前学生唯一学习目标。
 */
export interface LearningGoalResponse {
  target_id: string | null;
  updated_at: string | null;
}

export type LearningPathStatus = 'completed' | 'locked' | 'current' | 'available';

export interface LearningPathItem {
  id: string;
  title: string;
  description: string;
  chapter?: string | null;
  status: LearningPathStatus;
  locked_by?: string[];
  recommendation?: string;
  mastery: number;
  confidence: number;
  exercises: number;
  difficulty: number;
}

export interface LearningPathResponse {
  path: LearningPathItem[];
  estimated_exercises: number;
  statistics: {
    total: number;
    completed: number;
    progress: number;
  };
}
