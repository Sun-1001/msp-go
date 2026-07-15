import { type ChangeEvent, useEffect, useId, useRef, useState } from 'react';
import { ImagePlus, RefreshCw, X } from 'lucide-react';
import { Button } from '@/components/ui/Button';
import {
  answerImageAccept,
  validateAnswerImageFile,
} from '../utils/answerImageValidation';

export interface AnswerImageInputProps {
  file: File | null;
  disabled?: boolean;
  onChange: (file: File | null) => void;
}

export const AnswerImageInput = ({ file, disabled = false, onChange }: AnswerImageInputProps) => {
  const inputId = useId();
  const inputRef = useRef<HTMLInputElement>(null);
  const [previewUrl, setPreviewUrl] = useState<string | null>(null);
  const previewUrlRef = useRef<string | null>(null);
  const [validationError, setValidationError] = useState<string | null>(null);

  useEffect(() => {
    return () => {
      if (previewUrlRef.current) {
        URL.revokeObjectURL(previewUrlRef.current);
        previewUrlRef.current = null;
      }
    };
  }, []);

  const openFilePicker = () => inputRef.current?.click();

  const handleFileChange = (event: ChangeEvent<HTMLInputElement>) => {
    const nextFile = event.target.files?.[0];
    event.target.value = '';
    if (!nextFile) return;

    const validation = validateAnswerImageFile(nextFile);
    if (!validation.valid) {
      setValidationError(validation.error ?? '答案图片不符合上传要求');
      return;
    }

    setValidationError(null);
    const nextPreviewUrl = URL.createObjectURL(nextFile);
    if (previewUrlRef.current) {
      URL.revokeObjectURL(previewUrlRef.current);
    }
    previewUrlRef.current = nextPreviewUrl;
    setPreviewUrl(nextPreviewUrl);
    onChange(nextFile);
  };

  const removeFile = () => {
    setValidationError(null);
    if (previewUrlRef.current) {
      URL.revokeObjectURL(previewUrlRef.current);
      previewUrlRef.current = null;
    }
    setPreviewUrl(null);
    onChange(null);
  };

  return (
    <div className="space-y-2">
      <input
        id={inputId}
        ref={inputRef}
        type="file"
        accept={answerImageAccept}
        aria-label="选择答案图片"
        aria-describedby={validationError ? `${inputId}-error` : undefined}
        onChange={handleFileChange}
        disabled={disabled}
        className="hidden"
      />

      {file && previewUrl ? (
        <div className="flex max-w-md items-center gap-3 rounded-md border border-surface-200 bg-surface-50 p-3 dark:border-surface-700 dark:bg-surface-800">
          <img
            src={previewUrl}
            alt={`答案图片预览：${file.name}`}
            className="h-24 w-24 shrink-0 rounded-md border border-surface-200 bg-white object-contain dark:border-surface-700 dark:bg-surface-900"
          />
          <div className="min-w-0 flex-1">
            <p className="truncate text-sm font-medium text-surface-800 dark:text-surface-200">
              {file.name}
            </p>
            <p className="mt-1 text-xs text-surface-500">
              {(file.size / 1024).toFixed(1)} KB
            </p>
            <Button
              type="button"
              variant="ghost"
              size="sm"
              onClick={openFilePicker}
              disabled={disabled}
              className="mt-2 px-2"
            >
              <RefreshCw className="mr-1.5 h-4 w-4" aria-hidden="true" />
              更换图片
            </Button>
          </div>
          <Button
            type="button"
            variant="ghost"
            size="icon"
            onClick={removeFile}
            disabled={disabled}
            aria-label="移除答案图片"
            title="移除答案图片"
            className="shrink-0 text-surface-500 hover:text-red-600"
          >
            <X className="h-4 w-4" aria-hidden="true" />
          </Button>
        </div>
      ) : (
        <Button
          type="button"
          variant="outline"
          onClick={openFilePicker}
          disabled={disabled}
          className="w-full sm:w-auto"
        >
          <ImagePlus className="mr-2 h-4 w-4" aria-hidden="true" />
          上传答案图片
        </Button>
      )}

      {validationError ? (
        <p id={`${inputId}-error`} role="alert" className="text-sm text-red-600 dark:text-red-400">
          {validationError}
        </p>
      ) : null}
    </div>
  );
};
