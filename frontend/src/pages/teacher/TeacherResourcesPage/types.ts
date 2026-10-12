import type { Resource } from '@/modules/resource/api';

export type FilterType = 'all' | 'video' | 'document';
export type ViewMode = 'grid' | 'list';

export interface ResourcePageState {
  searchTerm: string;
  selectedType: FilterType;
  viewMode: ViewMode;
  deleteConfirmId: string | null;
  viewingResource: Resource | null;
  selectionMode: boolean;
  selectedResourceIds: Set<string>;
  showBatchDeleteConfirm: boolean;
  editingResource: Resource | null;
  showBatchImportModal: boolean;
}

export type ResourcePageAction =
  | { type: 'SET_SEARCH_TERM'; payload: string }
  | { type: 'SET_SELECTED_TYPE'; payload: FilterType }
  | { type: 'SET_VIEW_MODE'; payload: ViewMode }
  | { type: 'OPEN_DELETE_CONFIRM'; payload: string }
  | { type: 'CLOSE_DELETE_CONFIRM' }
  | { type: 'SET_VIEWING_RESOURCE'; payload: Resource | null }
  | { type: 'CLOSE_VIEWING_RESOURCE' }
  | { type: 'TOGGLE_SELECTION_MODE' }
  | { type: 'EXIT_SELECTION_MODE' }
  | { type: 'TOGGLE_RESOURCE_SELECTION'; payload: string }
  | { type: 'SELECT_ALL'; payload: string[] }
  | { type: 'CLEAR_SELECTION' }
  | { type: 'OPEN_BATCH_DELETE_CONFIRM' }
  | { type: 'CLOSE_BATCH_DELETE_CONFIRM' }
  | { type: 'OPEN_EDIT'; payload: Resource }
  | { type: 'CLOSE_EDIT' }
  | { type: 'OPEN_BATCH_IMPORT' }
  | { type: 'CLOSE_BATCH_IMPORT' };
