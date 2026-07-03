import { waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cancelTask, createSSEConnection, type SSEHandlers } from '@/libs/http/sseClient';

const encoder = new TextEncoder();

function streamResponse(chunks: string[]): Response {
  return new Response(
    new ReadableStream<Uint8Array>({
      start(controller) {
        for (const chunk of chunks) {
          controller.enqueue(encoder.encode(chunk));
        }
        controller.close();
      },
    }),
    {
      status: 200,
      headers: { 'Content-Type': 'text/event-stream' },
    }
  );
}

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
}

function mockHandlers(): Required<SSEHandlers> {
  return {
    onChunk: vi.fn(),
    onDone: vi.fn(),
    onError: vi.fn(),
    onCancelled: vi.fn(),
    onTaskInfo: vi.fn(),
    onOpen: vi.fn(),
    onClose: vi.fn(),
  };
}

describe('sseClient', () => {
  const fetchMock = vi.fn<typeof fetch>();

  beforeEach(() => {
    vi.restoreAllMocks();
    vi.stubGlobal('fetch', fetchMock);
    sessionStorage.clear();
    localStorage.clear();
    fetchMock.mockReset();
    vi.spyOn(console, 'debug').mockImplementation(() => undefined);
    vi.spyOn(console, 'warn').mockImplementation(() => undefined);
    vi.spyOn(console, 'error').mockImplementation(() => undefined);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it('parses task, chunk and done events across stream chunks', async () => {
    fetchMock.mockResolvedValueOnce(
      streamResponse([
        'event: task_info\ndata: {"task_id":"task-1"}\n\n',
        'event: message\ndata: {"type":"chunk","content":"Hel',
        'lo","agent":"tutor","message_id":"msg-1"}\n\n',
        'event: message\ndata: {"type":"done","message_id":"msg-1","agent":"tutor"}\n\n',
      ])
    );
    const handlers = mockHandlers();

    const controller = createSSEConnection('/stream', { message: 'hi' }, handlers);

    await waitFor(() => expect(handlers.onDone).toHaveBeenCalledWith('msg-1', 'tutor'));
    expect(handlers.onOpen).toHaveBeenCalledOnce();
    expect(handlers.onTaskInfo).toHaveBeenCalledWith('task-1');
    expect(handlers.onChunk).toHaveBeenCalledWith('Hello', 'tutor', 'msg-1');
    expect(controller.getTaskId()).toBe('task-1');
    expect(handlers.onClose).toHaveBeenCalledOnce();
  });

  it('does not log raw event data when JSON parsing fails', async () => {
    fetchMock.mockResolvedValueOnce(
      streamResponse([
        'event: message\ndata: {"type":"error","message":"secret-token"\n\n',
      ])
    );
    const handlers = mockHandlers();

    createSSEConnection('/stream', { message: 'hi' }, handlers);

    await waitFor(() => expect(handlers.onClose).toHaveBeenCalledOnce());
    expect(handlers.onError).not.toHaveBeenCalled();
    expect(JSON.stringify(vi.mocked(console.warn).mock.calls)).not.toContain('secret-token');
  });

  it('closes the stream with a connection error when one event is too large', async () => {
    const oversizedContent = 'x'.repeat(1024 * 1024 + 1);
    fetchMock.mockResolvedValueOnce(
      streamResponse([
        `event: message\ndata: ${JSON.stringify({
          type: 'chunk',
          content: oversizedContent,
        })}\n\n`,
      ])
    );
    const handlers = mockHandlers();

    createSSEConnection('/stream', { message: 'hi' }, handlers);

    await waitFor(() =>
      expect(handlers.onError).toHaveBeenCalledWith(
        expect.objectContaining({
          code: 'CONNECTION_ERROR',
          message: 'SSE event exceeded size limit',
        })
      )
    );
    expect(handlers.onClose).toHaveBeenCalledOnce();
  });

  it('falls back to safe string values for malformed event fields', async () => {
    fetchMock.mockResolvedValueOnce(
      streamResponse([
        `event: message\ndata: ${JSON.stringify({
          type: 'chunk',
          content: { text: 'bad' },
          agent: 123,
          message_id: false,
        })}\n\n`,
      ])
    );
    const handlers = mockHandlers();

    createSSEConnection('/stream', { message: 'hi' }, handlers);

    await waitFor(() => expect(handlers.onClose).toHaveBeenCalledOnce());
    expect(handlers.onChunk).toHaveBeenCalledWith('', null, '');
  });

  it('encodes task ids before building cancel URLs', async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse({ success: true }));

    await expect(cancelTask(' task/../?secret=1 ')).resolves.toBe(true);

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/session/task/task%2F..%2F%3Fsecret%3D1/cancel',
      expect.objectContaining({ method: 'POST' })
    );
  });

  it('does not send cancel requests for blank task ids', async () => {
    await expect(cancelTask('   ')).resolves.toBe(false);

    expect(fetchMock).not.toHaveBeenCalled();
  });
});
