import React, { useState, useEffect, useMemo, useCallback } from 'react';
import { MainLayout } from '../../components/layout/MainLayout';
import { Card, CardContent, CardHeader, CardTitle } from '../../components/ui/Card';
import { Button } from '../../components/ui/Button';
import { Badge } from '../../components/ui/Badge';
import { Progress } from '../../components/ui/Progress';
import { Select } from '../../components/ui/Select';
import { Input } from '../../components/ui/Input';
import { KnowledgeGraph, KnowledgeGraphLegend, type KnowledgeNode } from '@/modules/knowledge';
import { useAppDispatch, useAppSelector } from '../../store';
import { fetchKnowledgeGraph, updateFilter, selectNode } from '@/modules/knowledge/store/knowledgeSlice';
import { knowledgeService } from '@/modules/knowledge/services/knowledgeService';
import { buildKnowledgeGraphIndex } from '@/libs/graph';
import {
  Search,
  Filter,
  CircleDot,
  BookOpen,
  Lightbulb,
  Loader2,
  AlertCircle,
} from 'lucide-react';



const typeOptions = [
  { value: '', label: '全部类型' },
  { value: 'concept', label: '概念' },
  { value: 'theorem', label: '定理' },
  { value: 'method', label: '方法' },
];

const getNodeTypeIcon = (type: string) => {
  switch (type) {
    case 'concept':
      return <CircleDot className="h-4 w-4" />;
    case 'theorem':
      return <BookOpen className="h-4 w-4" />;
    case 'method':
      return <Lightbulb className="h-4 w-4" />;
    default:
      return <CircleDot className="h-4 w-4" />;
  }
};

const getNodeTypeBadge = (type: string) => {
  switch (type) {
    case 'concept':
      return <Badge variant="default">概念</Badge>;
    case 'theorem':
      return <Badge variant="secondary">定理</Badge>;
    case 'method':
      return <Badge variant="success">方法</Badge>;
    default:
      return <Badge variant="outline">{type}</Badge>;
  }
};

const getMasteryColor = (mastery: number) => {
  if (mastery >= 0.8) return 'success';
  if (mastery >= 0.6) return 'default';
  if (mastery >= 0.4) return 'warning';
  return 'destructive';
};

export const KnowledgeGraphPage: React.FC = () => {
  const dispatch = useAppDispatch();
  const { nodes, edges, statistics, filters, selectedNodeId, loadingState, error } = useAppSelector(
    (state) => state.knowledge
  );

  const [localSearchTerm, setLocalSearchTerm] = useState('');

  // 动态章节选项
  const [chapterOptions, setChapterOptions] = useState([
    { value: '', label: '全部章节' },
  ]);

  // 组件挂载时获取章节列表；图谱数据由 filters effect 统一加载。
  useEffect(() => {
    knowledgeService.getChapters().then((chapters) => {
      setChapterOptions([
        { value: '', label: '全部章节' },
        ...chapters.map(ch => ({ value: ch, label: ch })),
      ]);
    }).catch(() => {
      // 静默处理，保留默认的"全部章节"选项
    });
  }, []);

  // 筛选条件变化时重新获取数据
  useEffect(() => {
    dispatch(fetchKnowledgeGraph(filters));
  }, [dispatch, filters]);

  // 搜索防抖处理
  useEffect(() => {
    const timer = setTimeout(() => {
      if (localSearchTerm !== filters.search) {
        dispatch(updateFilter({ key: 'search', value: localSearchTerm || undefined }));
      }
    }, 500);

    return () => clearTimeout(timer);
  }, [localSearchTerm, filters.search, dispatch]);

  const handleChapterChange = useCallback((value: string) => {
    dispatch(updateFilter({ key: 'chapter', value: value || undefined }));
  }, [dispatch]);

  const handleTypeChange = useCallback((value: string) => {
    dispatch(updateFilter({ key: 'type', value: value || undefined }));
  }, [dispatch]);

  const handleNodeClick = useCallback((node: KnowledgeNode) => {
    dispatch(selectNode(node.id));
  }, [dispatch]);

  const graphIndex = useMemo(
    () => buildKnowledgeGraphIndex(nodes, edges),
    [nodes, edges],
  );

  const selectedNode = selectedNodeId
    ? graphIndex.nodesById.get(selectedNodeId) ?? null
    : null;
  const prerequisiteEdges = selectedNode
    ? graphIndex.prerequisiteEdgesByTarget.get(selectedNode.id) ?? []
    : [];
  const successorEdges = selectedNode
    ? graphIndex.successorEdgesBySource.get(selectedNode.id) ?? []
    : [];

  // 加载状态
  if (loadingState === 'loading') {
    return (
      <MainLayout>
        <div className="container mx-auto px-6 py-8 max-w-7xl">
          <div className="flex items-center justify-center h-[400px]">
            <div className="text-center">
              <Loader2 className="h-8 w-8 animate-spin text-primary-600 mx-auto mb-4" />
              <p className="text-surface-500 dark:text-surface-400">加载知识图谱数据中...</p>
            </div>
          </div>
        </div>
      </MainLayout>
    );
  }

  // 错误状态
  if (loadingState === 'error') {
    return (
      <MainLayout>
        <div className="container mx-auto px-6 py-8 max-w-7xl">
          <div className="flex items-center justify-center h-[400px]">
            <div className="text-center">
              <AlertCircle className="h-8 w-8 text-destructive-600 mx-auto mb-4" />
              <p className="text-surface-900 dark:text-surface-100 font-medium mb-2">加载失败</p>
              <p className="text-surface-500 dark:text-surface-400 mb-4">{error}</p>
              <Button onClick={() => dispatch(fetchKnowledgeGraph(filters))}>重试</Button>
            </div>
          </div>
        </div>
      </MainLayout>
    );
  }

  return (
    <MainLayout>
      <div className="container mx-auto px-6 py-8 max-w-7xl">
        {/* 页面标题 */}
        <div className="mb-8">
          <h1 className="text-3xl font-bold text-surface-900 dark:text-surface-100 mb-2">
            知识图谱
          </h1>
          <p className="text-surface-500 dark:text-surface-400">
            可视化探索高等数学知识点之间的关联，了解你的学习进度
          </p>
        </div>

        <div className="grid grid-cols-1 lg:grid-cols-4 gap-6">
          {/* 左侧筛选面板 */}
          <div className="lg:col-span-1 space-y-6">
            <Card>
              <CardHeader>
                <CardTitle className="text-lg flex items-center gap-2">
                  <Filter className="h-5 w-5" />
                  筛选
                </CardTitle>
              </CardHeader>
              <CardContent className="space-y-4">
                <div>
                  <label className="text-sm font-medium text-surface-700 dark:text-surface-300 mb-2 block">
                    搜索知识点
                  </label>
                  <div className="relative">
                    <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-surface-400" />
                    <Input
                      placeholder="输入关键词..."
                      value={localSearchTerm}
                      onChange={(e) => setLocalSearchTerm(e.target.value)}
                      className="pl-10"
                    />
                  </div>
                </div>

                <div>
                  <label className="text-sm font-medium text-surface-700 dark:text-surface-300 mb-2 block">
                    章节
                  </label>
                  <Select
                    options={chapterOptions}
                    value={filters.chapter || ''}
                    onChange={handleChapterChange}
                  />
                </div>

                <div>
                  <label className="text-sm font-medium text-surface-700 dark:text-surface-300 mb-2 block">
                    类型
                  </label>
                  <Select
                    options={typeOptions}
                    value={filters.type || ''}
                    onChange={handleTypeChange}
                  />
                </div>
              </CardContent>
            </Card>

            {/* 图例 */}
            <Card>
              <CardHeader>
                <CardTitle className="text-lg">图例</CardTitle>
              </CardHeader>
              <CardContent>
                <KnowledgeGraphLegend />
              </CardContent>
            </Card>

            {/* 统计信息 */}
            <Card>
              <CardHeader>
                <CardTitle className="text-lg">学习统计</CardTitle>
              </CardHeader>
              <CardContent className="space-y-4">
                <div>
                  <div className="flex justify-between text-sm mb-1">
                    <span className="text-surface-600 dark:text-surface-400">整体掌握度</span>
                    <span className="font-medium text-surface-900 dark:text-surface-100">
                      {statistics ? `${Math.round(statistics.overall_mastery * 100)}%` : '0%'}
                    </span>
                  </div>
                  <Progress value={statistics ? statistics.overall_mastery * 100 : 0} />
                </div>
                <div className="grid grid-cols-2 gap-4 text-center">
                  <div>
                    <div className="text-2xl font-bold text-primary-600 dark:text-primary-400">
                      {statistics?.total_nodes || 0}
                    </div>
                    <div className="text-xs text-surface-500 dark:text-surface-400">知识点总数</div>
                  </div>
                  <div>
                    <div className="text-2xl font-bold text-emerald-600 dark:text-emerald-400">
                      {statistics?.mastered_nodes || 0}
                    </div>
                    <div className="text-xs text-surface-500 dark:text-surface-400">已掌握</div>
                  </div>
                </div>
              </CardContent>
            </Card>
          </div>

          {/* 右侧主内容区 */}
          <div className="lg:col-span-3 space-y-6">
            {/* 知识图谱可视化区域 */}
            <Card className="min-h-[400px]">
              <CardContent className="p-4 h-full">
                {nodes.length > 0 ? (
                  <KnowledgeGraph
                    nodes={nodes}
                    edges={edges}
                    onNodeClick={handleNodeClick}
                    height={400}
                  />
                ) : (
                  <div className="flex items-center justify-center h-[400px] text-surface-500 dark:text-surface-400">
                    没有匹配的知识点
                  </div>
                )}
              </CardContent>
            </Card>

            {/* 知识点列表 */}
            <Card>
              <CardHeader>
                <CardTitle className="text-lg">知识点列表</CardTitle>
              </CardHeader>
              <CardContent>
                <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                  {nodes.map((node) => (
                    <div
                      key={node.id}
                      onClick={() => dispatch(selectNode(node.id))}
                      className={`p-4 rounded-lg border cursor-pointer transition-all ${
                        selectedNode?.id === node.id
                          ? 'border-primary-500 bg-primary-50 dark:bg-primary-900/20'
                          : 'border-surface-200 dark:border-surface-700 hover:border-primary-300 dark:hover:border-primary-700'
                      }`}
                    >
                      <div className="flex items-start justify-between mb-3">
                        <div className="flex items-center gap-2">
                          {getNodeTypeIcon(node.type)}
                          <span className="font-medium text-surface-900 dark:text-surface-100">
                            {node.label}
                          </span>
                        </div>
                        {getNodeTypeBadge(node.type)}
                      </div>
                      <div className="text-xs text-surface-500 dark:text-surface-400 mb-2">
                        {node.chapter}
                      </div>
                      <div className="flex items-center gap-2">
                        <Progress
                          value={node.mastery * 100}
                          variant={getMasteryColor(node.mastery)}
                          size="sm"
                          className="flex-1"
                        />
                        <span className="text-sm font-medium text-surface-700 dark:text-surface-300">
                          {(node.mastery * 100).toFixed(0)}%
                        </span>
                      </div>
                    </div>
                  ))}
                </div>
              </CardContent>
            </Card>

            {/* 选中节点详情 */}
            {selectedNode && (
              <Card>
                <CardHeader>
                  <CardTitle className="text-lg flex items-center gap-2">
                    {getNodeTypeIcon(selectedNode.type)}
                    {selectedNode.label}
                    {getNodeTypeBadge(selectedNode.type)}
                  </CardTitle>
                </CardHeader>
                <CardContent className="space-y-4">
                  <div>
                    <h4 className="text-sm font-medium text-surface-700 dark:text-surface-300 mb-2">
                      掌握度
                    </h4>
                    <Progress
                      value={selectedNode.mastery * 100}
                      variant={getMasteryColor(selectedNode.mastery)}
                      showLabel
                    />
                  </div>
                  <div>
                    <h4 className="text-sm font-medium text-surface-700 dark:text-surface-300 mb-2">
                      先修知识点
                    </h4>
                    <div className="flex flex-wrap gap-2">
                      {prerequisiteEdges.map((edge) => {
                          const sourceNode = graphIndex.nodesById.get(edge.source);
                          return sourceNode ? (
                            <Badge key={edge.source} variant="outline">
                              {sourceNode.label}
                            </Badge>
                          ) : null;
                        })}
                      {prerequisiteEdges.length === 0 && (
                        <span className="text-sm text-surface-500 dark:text-surface-400">无</span>
                      )}
                    </div>
                  </div>
                  <div>
                    <h4 className="text-sm font-medium text-surface-700 dark:text-surface-300 mb-2">
                      后续知识点
                    </h4>
                    <div className="flex flex-wrap gap-2">
                      {successorEdges.map((edge) => {
                          const targetNode = graphIndex.nodesById.get(edge.target);
                          return targetNode ? (
                            <Badge key={edge.target} variant="outline">
                              {targetNode.label}
                            </Badge>
                          ) : null;
                        })}
                      {successorEdges.length === 0 && (
                        <span className="text-sm text-surface-500 dark:text-surface-400">无</span>
                      )}
                    </div>
                  </div>
                  <div className="flex gap-2 pt-4">
                    <Button className="flex-1">开始学习</Button>
                    <Button variant="outline" className="flex-1">查看习题</Button>
                  </div>
                </CardContent>
              </Card>
            )}
          </div>
        </div>
      </div>
    </MainLayout>
  );
};
