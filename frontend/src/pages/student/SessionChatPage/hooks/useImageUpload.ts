import { useState, useCallback, useRef, useEffect } from 'react';
import { uploadService } from '@/modules/upload/services/uploadService';

export const useImageUpload = () => {
  const [selectedImages, setSelectedImages] = useState<File[]>([]);
  const [previewUrls, setPreviewUrls] = useState<string[]>([]);
  const [isUploading, setIsUploading] = useState(false);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const previewUrlsRef = useRef<string[]>([]);

  // 处理图片选择
  const handleImageSelect = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
    const files = e.target.files;
    if (!files || files.length === 0) return;

    const newFiles: File[] = [];
    const newUrls: string[] = [];

    for (let i = 0; i < files.length; i++) {
      const file = files[i];
      const validation = uploadService.validateImageFile(file);
      if (validation.valid) {
        newFiles.push(file);
        newUrls.push(URL.createObjectURL(file));
      } else {
        console.error(validation.error);
      }
    }

    setSelectedImages((prev) => [...prev, ...newFiles]);
    setPreviewUrls((prev) => {
      const next = [...prev, ...newUrls];
      previewUrlsRef.current = next;
      return next;
    });

    // 重置 input 以允许重复选择同一文件
    if (fileInputRef.current) {
      fileInputRef.current.value = '';
    }
  }, []);

  // 移除选中的图片
  const handleRemoveImage = useCallback((index: number) => {
    setSelectedImages((prev) => prev.filter((_, i) => i !== index));
    setPreviewUrls((prev) => {
      const url = prev[index];
      if (url) URL.revokeObjectURL(url);
      const next = prev.filter((_, i) => i !== index);
      previewUrlsRef.current = next;
      return next;
    });
  }, []);

  // 清空图片
  const clearImages = useCallback(() => {
    previewUrlsRef.current.forEach((url) => URL.revokeObjectURL(url));
    previewUrlsRef.current = [];
    setSelectedImages([]);
    setPreviewUrls([]);
  }, []);

  // 清理预览 URL
  useEffect(() => {
    return () => {
      previewUrlsRef.current.forEach((url) => URL.revokeObjectURL(url));
      previewUrlsRef.current = [];
    };
  }, []);

  return {
    selectedImages,
    previewUrls,
    isUploading,
    setIsUploading,
    fileInputRef,
    handleImageSelect,
    handleRemoveImage,
    clearImages,
  };
};
