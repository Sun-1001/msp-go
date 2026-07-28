/**
 * 图片上传服务
 *
 * 提供图片上传功能
 */

import { apiClient } from '@/libs/http/apiClient';

export interface UploadResponse {
  file_id: string;
  url: string;
  filename: string;
  content_type: string;
  size: number;
}

/**
 * 上传图片
 *
 * @param file 图片文件
 * @returns 上传结果
 */
export async function uploadImage(file: File, onProgress?: (percent: number) => void): Promise<UploadResponse> {
  const formData = new FormData();
  formData.append('file', file);

  // 图片上传超时时间：最大 10MB，给 60 秒足够了
  const response = await apiClient.post<UploadResponse>('/upload/image', formData, {
    headers: {
      'Content-Type': 'multipart/form-data',
    },
    timeout: 60000, // 60 秒
    onUploadProgress: onProgress
      ? (progressEvent) => {
          const total = progressEvent.total ?? file.size;
          onProgress(Math.min(100, Math.round((progressEvent.loaded * 100) / total)));
        }
      : undefined,
  });

  return response.data;
}

/**
 * 验证图片文件
 *
 * @param file 文件
 * @returns 验证结果
 */
export function validateImageFile(file: File): { valid: boolean; error?: string } {
  const allowedTypes = ['image/jpeg', 'image/png', 'image/gif', 'image/webp'];
  const maxSize = 10 * 1024 * 1024; // 10MB

  if (!allowedTypes.includes(file.type)) {
    return {
      valid: false,
      error: `不支持的文件类型: ${file.type}。支持的类型: JPEG, PNG, GIF, WebP`,
    };
  }

  if (file.size > maxSize) {
    return {
      valid: false,
      error: `文件大小超过限制: ${(file.size / 1024 / 1024).toFixed(2)}MB > 10MB`,
    };
  }

  return { valid: true };
}

/**
 * 上传资源文件（视频/文档）
 *
 * @param file 资源文件
 * @param onProgress 上传进度回调（0-100）
 * @returns 上传结果
 */
export async function uploadResourceFile(
  file: File,
  onProgress?: (percent: number) => void
): Promise<UploadResponse> {
  const formData = new FormData();
  formData.append('file', file);

  // 根据文件大小动态计算超时时间
  // 假设上传速度为 1MB/s，加上处理时间，至少给 2 倍时间
  const fileSizeMB = file.size / (1024 * 1024);
  const estimatedSeconds = Math.max(60, fileSizeMB * 2); // 最少 60 秒
  const timeout = Math.min(estimatedSeconds * 1000, 600000); // 最多 10 分钟

  const response = await apiClient.post<UploadResponse>('/upload/resource', formData, {
    headers: {
      'Content-Type': 'multipart/form-data',
    },
    timeout, // 动态超时时间
    onUploadProgress: onProgress
      ? (progressEvent) => {
          const total = progressEvent.total ?? file.size;
          const percent = Math.round((progressEvent.loaded * 100) / total);
          onProgress(percent);
        }
      : undefined,
  });

  return response.data;
}

/**
 * 验证资源文件类型
 */
export function validateResourceFile(file: File): { valid: boolean; error?: string } {
  const allowedTypes = [
    'video/mp4', 'video/avi', 'video/quicktime', 'video/x-matroska', 'video/webm',
    'application/pdf', 'application/msword',
    'application/vnd.openxmlformats-officedocument.wordprocessingml.document',
    'application/vnd.ms-powerpoint',
    'application/vnd.openxmlformats-officedocument.presentationml.presentation',
    'text/plain', 'text/markdown',
  ];
  const maxSize = 500 * 1024 * 1024; // 500MB

  if (!allowedTypes.includes(file.type)) {
    return {
      valid: false,
      error: `不支持的文件类型: ${file.type}`,
    };
  }

  if (file.size > maxSize) {
    return {
      valid: false,
      error: `文件大小超过限制: ${(file.size / 1024 / 1024).toFixed(2)}MB > 500MB`,
    };
  }

  return { valid: true };
}

/** 上传消息中心文档附件（最大 50MB）。 */
export async function uploadMessageFile(file: File, onProgress?: (percent: number) => void): Promise<UploadResponse> {
  const formData = new FormData();
  formData.append('file', file);
  const response = await apiClient.post<UploadResponse>('/upload/message-file', formData, {
    headers: { 'Content-Type': 'multipart/form-data' },
    timeout: 120000,
    onUploadProgress: onProgress
      ? (progressEvent) => {
          const total = progressEvent.total ?? file.size;
          onProgress(Math.min(100, Math.round((progressEvent.loaded * 100) / total)));
        }
      : undefined,
  });
  return response.data;
}

/** 验证消息中心可用的文档附件，不接受视频。 */
export function validateDocumentFile(file: File): { valid: boolean; error?: string } {
  if (file.type.startsWith('video/')) {
    return { valid: false, error: '消息附件暂不支持视频文件' };
  }
	const maxSize = 50 * 1024 * 1024;
	if (file.size > maxSize) {
		return { valid: false, error: `文件大小超过限制: ${(file.size / 1024 / 1024).toFixed(2)}MB > 50MB` };
	}
  return validateResourceFile(file);
}

export const uploadService = {
  uploadImage,
  validateImageFile,
  uploadResourceFile,
  uploadMessageFile,
  validateResourceFile,
  validateDocumentFile,
};

export default uploadService;
