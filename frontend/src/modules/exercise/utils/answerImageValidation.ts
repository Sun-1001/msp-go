import { uploadService } from '@/modules/upload/services/uploadService';

export const answerImageAccept = 'image/jpeg,image/png,image/gif';

const answerImageTypes = new Set(answerImageAccept.split(','));

export const validateAnswerImageFile = (file: File): { valid: boolean; error?: string } => {
  if (!answerImageTypes.has(file.type)) {
    return {
      valid: false,
      error: '答案图片仅支持 JPEG、PNG 或 GIF 格式',
    };
  }

  return uploadService.validateImageFile(file);
};
