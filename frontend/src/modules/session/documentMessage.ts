const CHAT_DOCUMENT_MESSAGE_TYPE = 'msp.document-context.v1';

export interface ChatDocumentContext {
  filename: string;
  content: string;
}

interface ChatDocumentMessage {
  type: typeof CHAT_DOCUMENT_MESSAGE_TYPE;
  documents: ChatDocumentContext[];
  message: string;
}

/** Persist model context in a stable envelope while keeping UI projection separate. */
export function formatDocumentsForChat(
  documents: readonly ChatDocumentContext[],
  message: string
): string {
  if (documents.length === 0) return message;

  const payload: ChatDocumentMessage = {
    type: CHAT_DOCUMENT_MESSAGE_TYPE,
    documents: documents.map(({ filename, content }) => ({ filename, content })),
    message,
  };
  return JSON.stringify(payload);
}

function isChatDocumentContext(value: unknown): value is ChatDocumentContext {
  return typeof value === 'object'
    && value !== null
    && !Array.isArray(value)
    && 'filename' in value
    && typeof value.filename === 'string'
    && 'content' in value
    && typeof value.content === 'string';
}

/** Project a persisted document envelope to the same compact text shown on first send. */
export function formatChatMessageForDisplay(content: string): string {
  try {
    const parsed: unknown = JSON.parse(content);
    if (
      typeof parsed !== 'object' ||
      parsed === null ||
      Array.isArray(parsed) ||
      !('type' in parsed) ||
      parsed.type !== CHAT_DOCUMENT_MESSAGE_TYPE ||
      !('message' in parsed) ||
      typeof parsed.message !== 'string' ||
      !('documents' in parsed) ||
      !Array.isArray(parsed.documents) ||
      parsed.documents.length === 0 ||
      !parsed.documents.every(isChatDocumentContext)
    ) {
      return content;
    }

    return `${parsed.message}\n\n📎 ${parsed.documents
      .map((document) => document.filename)
      .join(', ')}`;
  } catch {
    return content;
  }
}
