/** AI 学习会话请求的客户端边界。后端独立执行相同的安全校验。 */
export const MAX_CHAT_IMAGES = 5;
export const MAX_CHAT_DOCUMENTS = 5;
export const MAX_CHAT_MESSAGE_KIB = 12;
export const MAX_CHAT_MESSAGE_BYTES = MAX_CHAT_MESSAGE_KIB * 1024;
