export interface NoteSummary { id: string; title: string; updated_at: string }
export interface Note extends NoteSummary {
  owner_id: string;
  body: string;
  material_id: string | null;
  source_info: string;
  revision: number;
  created_at: string;
}
export interface NoteInput { title: string; body: string; material_id: string | null; source_info: string; revision: number }
export interface NoteLink { source_id: string; target_id: string; owner_id: string; source_type: 'markdown'; created_at: string }
export interface NoteNetwork { nodes: NoteSummary[]; links: NoteLink[] }
export interface NoteMaterial { id: string; filename: string; content_type: string; byte_size: number; created_at: string }
