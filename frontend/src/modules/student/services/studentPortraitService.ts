/**
 * 学生画像 API 服务
 *
 * 提供学生画像的获取、生成和清除功能
 */

import { apiClient } from '@/libs/http/apiClient';
import { logger } from '@/libs/utils/logger';
import type {
  StudentPortrait,
  GeneratePortraitResponse,
  ClearPortraitResponse,
  PortraitInsights,
  PortraitRangeType,
  PortraitActionStartResponse,
} from '@/modules/student/types/studentPortrait';

const portraitLogger = logger.createContextLogger('StudentPortrait');

const BASE_PATH = '/portrait';

export const studentPortraitService = {

  /** 获取与学习统计时间范围一致的结构化画像洞察。 */
  async getInsights(range: PortraitRangeType, signal?: AbortSignal): Promise<PortraitInsights> {
    try {
      const response = await apiClient.get<PortraitInsights>('/progress/portrait-insights', {
        params: { range },
        signal,
      });
      return response.data;
    } catch (error) {
      portraitLogger.error('获取学生画像洞察失败', error);
      throw error;
    }
  },

  /** 显式开始画像行动；练习提交模块无需感知画像状态。 */
  async startAction(conceptId: string): Promise<PortraitActionStartResponse> {
    try {
      const response = await apiClient.post<PortraitActionStartResponse>(
        `/progress/portrait-actions/${encodeURIComponent(conceptId)}/start`
      );
      return response.data;
    } catch (error) {
      portraitLogger.error('开始画像行动失败', error);
      throw error;
    }
  },

  /**
   * 获取学生画像
   */
  async getPortrait(): Promise<StudentPortrait> {
    try {
      const response = await apiClient.get<StudentPortrait>(BASE_PATH);
      portraitLogger.debug('获取学生画像成功', {
        has_content: response.data.has_content,
      });
      return response.data;
    } catch (error) {
      portraitLogger.error('获取学生画像失败', error);
      throw error;
    }
  },

  /**
   * 生成/重新生成学生画像
   */
  async generatePortrait(range: PortraitRangeType): Promise<GeneratePortraitResponse> {
    try {
      const response = await apiClient.post<GeneratePortraitResponse>(
        `${BASE_PATH}/generate`,
        undefined,
        { params: { range }, timeout: 60_000 }
      );
      portraitLogger.info('生成学生画像成功', {
        version: response.data.portrait_version,
      });
      return response.data;
    } catch (error) {
      portraitLogger.error('生成学生画像失败', error);
      throw error;
    }
  },

  /**
   * 清除学生画像
   */
  async clearPortrait(): Promise<ClearPortraitResponse> {
    try {
      const response =
        await apiClient.delete<ClearPortraitResponse>(BASE_PATH);
      portraitLogger.info('清除学生画像成功');
      return response.data;
    } catch (error) {
      portraitLogger.error('清除学生画像失败', error);
      throw error;
    }
  },
};

export default studentPortraitService;
