export { sessionService } from './services/sessionService';
export type { ExerciseTutorResponse } from './services/sessionService';
export { formatChatMessageForDisplay, formatDocumentsForChat } from './documentMessage';
export { isDefinitiveDraftIdentityError, isSessionNotFoundError } from './errors';
export { normalizeSessionKnowledge } from './knowledge';
export type { SessionKnowledge } from './knowledge';
export { MAX_CHAT_DOCUMENTS, MAX_CHAT_IMAGES, MAX_CHAT_MESSAGE_BYTES, MAX_CHAT_MESSAGE_KIB } from './limits';
export type { StudyAction, StudyProgress, StudyTurnInput } from './study';
export type { ChatMode, ChatSessionListItem, DraftSessionIdentity, SessionMode } from './types';
