import React, { useMemo } from 'react';
import { MarkdownContent } from '@/components/chat/MarkdownContent';
import type { AnnouncementContentFormat } from './types';

interface AnnouncementContentProps {
  content: string;
  format: AnnouncementContentFormat;
  className?: string;
  title?: string;
}

const htmlDocumentPrefix = `<!doctype html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<meta http-equiv="Content-Security-Policy" content="default-src 'none'; img-src data:; font-src data:; style-src 'unsafe-inline'; form-action 'none'; base-uri 'none'">
<style>
  :root { color-scheme: light; }
  * { box-sizing: border-box; }
  body { margin: 0; padding: 20px; color: #1f2937; background: #fff; font: 15px/1.7 ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; overflow-wrap: anywhere; }
  h1, h2, h3, h4 { margin: 1.2em 0 .55em; color: #111827; line-height: 1.3; }
  h1 { margin-top: 0; font-size: 1.5rem; } h2 { font-size: 1.25rem; } h3 { font-size: 1.1rem; }
  p, ul, ol, pre, blockquote, table { margin: .75em 0; }
  img { max-width: 100%; height: auto; }
  pre, code { font-family: ui-monospace, SFMono-Regular, Consolas, monospace; }
  pre { overflow: auto; padding: 12px; border-radius: 6px; background: #f3f4f6; }
  code { padding: .1em .3em; border-radius: 4px; background: #f3f4f6; }
  pre code { padding: 0; background: transparent; }
  blockquote { margin-left: 0; padding-left: 14px; border-left: 3px solid #d1d5db; color: #4b5563; }
  table { width: 100%; border-collapse: collapse; }
  th, td { padding: 8px 10px; border: 1px solid #d1d5db; text-align: left; }
  a { color: #2563eb; pointer-events: none; text-decoration: underline; }
</style>
</head>
<body>`;

function buildSandboxedDocument(content: string): string {
  return `${htmlDocumentPrefix}${content}</body></html>`;
}

export const AnnouncementContent = React.memo<AnnouncementContentProps>(({
  content,
  format,
  className = '',
  title = 'HTML 公告内容',
}) => {
  const sourceDocument = useMemo(
    () => (format === 'html' ? buildSandboxedDocument(content) : ''),
    [content, format]
  );

  if (format === 'html') {
    return (
      <iframe
        title={title}
        sandbox=""
        referrerPolicy="no-referrer"
        srcDoc={sourceDocument}
        className={`h-[min(50vh,480px)] min-h-60 w-full border-0 bg-white ${className}`}
      />
    );
  }

  return (
    <div className={`min-w-0 [overflow-wrap:anywhere] text-surface-800 dark:text-surface-200 ${className}`}>
      <MarkdownContent content={content} />
    </div>
  );
});

AnnouncementContent.displayName = 'AnnouncementContent';
