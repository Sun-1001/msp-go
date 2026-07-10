/**
 * 资源工具函数
 *
 * 提供 URL/文件名解析、资源类型识别等功能
 */

import type { ResourceType } from '@/modules/resource/types/resource';
import { replaceAsciiControlCharacters } from '@/libs/utils/controlCharacters';
import { hasUnsafeUrlCharacters, normalizeSafeHttpUrl } from '@/libs/utils/safeUrl';

const LOCAL_RESOURCE_PATH_PATTERN = /^\/uploads\/(?:documents|videos)\/[A-Za-z0-9._~!$&'()*+,;=:@/-]+$/;
const MAX_RESOURCE_TITLE_LENGTH = 100;
const MAX_LOCATION_SEARCH_LENGTH = 4096;
const MAX_INITIAL_SEARCH_LENGTH = 100;

function safeDecodeUrlComponent(value: string): string {
  try {
    return decodeURIComponent(value);
  } catch {
    return value;
  }
}

function normalizeDisplayText(value: string, maxLength: number): string {
  return replaceAsciiControlCharacters(value, '').trim().slice(0, maxLength);
}

/**
 * 从 URL 提取标题
 */
export function extractTitleFromUrl(url: string): string {
  try {
    const urlObj = new URL(url);

    // 尝试从路径提取
    const segments = urlObj.pathname.split('/').filter(Boolean);
    if (segments.length > 0) {
      const lastSegment = segments[segments.length - 1];
      // 移除文件扩展名和查询参数
      const title = normalizeDisplayText(
        safeDecodeUrlComponent(lastSegment.replace(/\.[^/.]+$/, '')),
        MAX_RESOURCE_TITLE_LENGTH
      );
      if (title && title.length > 0) {
        return title;
      }
    }

    // 回退到主机名
    return urlObj.hostname.replace('www.', '');
  } catch {
    // URL 解析失败，截取前50个字符
    return url.slice(0, 50);
  }
}

/**
 * 从 URL 识别资源类型
 */
export function detectResourceTypeFromUrl(url: string): ResourceType {
  const lowerUrl = url.toLowerCase();

  // 视频平台
  if (
    lowerUrl.includes('bilibili.com') ||
    lowerUrl.includes('youtube.com') ||
    lowerUrl.includes('youtu.be') ||
    lowerUrl.includes('vimeo.com') ||
    lowerUrl.includes('douyin.com') ||
    lowerUrl.includes('ixigua.com')
  ) {
    return 'video';
  }

  // 文件扩展名检测
  try {
    const urlObj = new URL(url);
    const pathname = urlObj.pathname.toLowerCase();
    const ext = pathname.split('.').pop()?.split('?')[0];

    if (ext) {
      // 视频扩展名
      if (['mp4', 'avi', 'mov', 'mkv', 'webm', 'flv', 'wmv'].includes(ext)) {
        return 'video';
      }
      // 文档扩展名
      if (['pdf', 'doc', 'docx', 'ppt', 'pptx', 'xls', 'xlsx', 'txt', 'md'].includes(ext)) {
        return 'document';
      }
    }
  } catch {
    // URL 解析失败，忽略
  }

  // 外部链接默认视频类型
  return 'video';
}

/**
 * 从 URL 提取来源
 */
export function extractSourceFromUrl(url: string): string {
  try {
    const hostname = new URL(url).hostname.replace('www.', '');

    // 常见平台映射
    const sourceMap: Record<string, string> = {
      'bilibili.com': 'Bilibili',
      'youtube.com': 'YouTube',
      'youtu.be': 'YouTube',
      'vimeo.com': 'Vimeo',
      'douyin.com': '抖音',
      'ixigua.com': '西瓜视频',
      'zhihu.com': '知乎',
      'jianshu.com': '简书',
      'csdn.net': 'CSDN',
      'github.com': 'GitHub',
      'gitee.com': 'Gitee',
      'pan.baidu.com': '百度网盘',
      'drive.google.com': 'Google Drive',
      'docs.google.com': 'Google Docs',
    };

    // 查找匹配的平台
    for (const [domain, source] of Object.entries(sourceMap)) {
      if (hostname.includes(domain)) {
        return source;
      }
    }

    // 返回主机名作为来源
    return hostname;
  } catch {
    return '';
  }
}

/**
 * 从文件名提取标题
 */
export function extractTitleFromFilename(filename: string): string {
  // 移除扩展名
  return filename.replace(/\.[^/.]+$/, '');
}

/**
 * 从文件扩展名识别资源类型
 */
export function detectResourceTypeFromFile(filename: string): ResourceType {
  const ext = filename.split('.').pop()?.toLowerCase();

  if (!ext) {
    return 'document';
  }

  // 视频扩展名
  const videoExts = ['mp4', 'avi', 'mov', 'mkv', 'webm', 'flv', 'wmv', 'm4v'];
  if (videoExts.includes(ext)) {
    return 'video';
  }

  // 文档扩展名
  const docExts = ['pdf', 'doc', 'docx', 'ppt', 'pptx', 'xls', 'xlsx', 'txt', 'md', 'rtf'];
  if (docExts.includes(ext)) {
    return 'document';
  }

  // 默认文档类型
  return 'document';
}

/**
 * 解析多行链接文本
 * 返回去重后的有效 URL 列表
 */
export function parseLinksFromText(text: string): string[] {
  const lines = text
    .split(/[\n\r]+/)
    .map((line) => line.trim())
    .filter(Boolean);

  const validUrls: string[] = [];
  const seen = new Set<string>();

  for (const line of lines) {
    const normalized = normalizeSafeHttpUrl(line);
    if (normalized && !seen.has(normalized)) {
      seen.add(normalized);
      validUrls.push(normalized);
    }
  }

  return validUrls;
}

/**
 * 规范化可打开的资源 URL，拒绝危险协议和异常本地路径
 */
export function normalizeOpenableResourceUrl(rawUrl: string | null | undefined): string | null {
  const value = rawUrl?.trim();
  if (!value || hasUnsafeUrlCharacters(value)) {
    return null;
  }
  if (value.startsWith('/')) {
    if (!LOCAL_RESOURCE_PATH_PATTERN.test(value)) {
      return null;
    }
    if (value.split('/').some((part) => part === '..' || part === '.')) {
      return null;
    }
    try {
      return new URL(value, window.location.origin).href;
    } catch {
      return null;
    }
  }
  if (value.startsWith('//')) {
    return null;
  }
  return normalizeSafeHttpUrl(value);
}

/**
 * 安全打开资源链接
 */
export function openResourceUrl(rawUrl: string | null | undefined): boolean {
  const url = normalizeOpenableResourceUrl(rawUrl);
  if (!url) {
    return false;
  }
  window.open(url, '_blank', 'noopener,noreferrer');
  return true;
}

/**
 * 从资源中心页面 URL 查询串读取初始搜索词
 */
export function getInitialResourceSearch(locationSearch: string): string {
  if (locationSearch.length > MAX_LOCATION_SEARCH_LENGTH) {
    return '';
  }
  const search = new URLSearchParams(locationSearch).get('search');
  return search ? normalizeDisplayText(search, MAX_INITIAL_SEARCH_LENGTH) : '';
}

/**
 * 生成简单的唯一 ID
 */
export function generateTempId(): string {
  return `temp_${Date.now()}_${Math.random().toString(36).slice(2, 9)}`;
}
