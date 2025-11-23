// User types
export interface User {
  id: string;
  email: string;
  name: string;
  created_at: string;
}

export interface AuthTokens {
  access_token: string;
  refresh_token: string;
  expires_at: string;
}

export interface LoginRequest {
  email: string;
  password: string;
}

export interface RegisterRequest {
  email: string;
  password: string;
  name: string;
}

export interface AuthResponse {
  user: User;
  access_token: string;
  refresh_token: string;
  expires_at: string;
}

// Project types
export interface Project {
  id: string;
  name: string;
  description: string;
  owner_id: string;
  collaborators: Collaborator[];
  created_at: string;
  updated_at: string;
}

export interface Collaborator {
  user_id: string;
  email: string;
  name: string;
  role: 'owner' | 'editor' | 'viewer';
  added_at: string;
}

export interface CreateProjectRequest {
  name: string;
  description: string;
}

export interface UpdateProjectRequest {
  name?: string;
  description?: string;
}

export interface ShareProjectRequest {
  collaborator_email: string;
  role: 'editor' | 'viewer';
}

// File types
export interface FileItem {
  id: string;
  project_id: string;
  name: string;
  path: string;
  content_type: string;
  size: number;
  is_binary: boolean;
  storage_key: string;
  created_at: string;
  updated_at: string;
}

export interface CreateFileRequest {
  name: string;
  path: string;
  content: string;
  content_type?: string;
}

export interface UpdateFileRequest {
  content: string;
}

export interface FileNode {
  name: string;
  path: string;
  type: 'file' | 'folder';
  children?: FileNode[];
  file?: FileItem;
}

// Compilation types
export type CompilationStatus = 'queued' | 'running' | 'completed' | 'failed' | 'timeout';

export interface Compilation {
  id: string;
  project_id: string;
  user_id: string;
  status: CompilationStatus;
  compiler: 'pdflatex' | 'xelatex' | 'lualatex';
  main_file: string;
  output_url?: string;
  log?: string;
  error?: string;
  duration_ms?: number;
  created_at: string;
  started_at?: string;
  completed_at?: string;
}

export interface CompileRequest {
  project_id: string;
  compiler: 'pdflatex' | 'xelatex' | 'lualatex';
  main_file: string;
}

export interface CompilationStats {
  total_compilations: number;
  successful_compilations: number;
  failed_compilations: number;
  avg_duration_ms: number;
  cache_hit_rate: number;
}

export interface QueueStats {
  queue_length: number;
  pending_count: number;
  active_workers: number;
}

// WebSocket types
export type MessageType =
  | 'user_joined'
  | 'user_left'
  | 'yjs_update'
  | 'cursor_update'
  | 'selection_update'
  | 'awareness_update';

export interface WebSocketMessage {
  type: MessageType;
  payload: any;
  user_id: string;
  user_name: string;
  timestamp: string;
}

export interface UserPresence {
  user_id: string;
  user_name: string;
  color: string;
  cursor?: CursorPosition;
  selection?: Selection;
}

export interface CursorPosition {
  line: number;
  column: number;
}

export interface Selection {
  start: CursorPosition;
  end: CursorPosition;
}

// Collaboration types
export interface YjsUpdate {
  id: string;
  project_id: string;
  document_name: string;
  update: string; // base64 encoded
  version: number;
  user_id: string;
  clock: number;
  created_at: string;
}

export interface DocumentState {
  snapshot?: string; // base64 encoded
  updates: YjsUpdate[];
  current_version: number;
}

export interface CollaborationMetrics {
  total_updates: number;
  unique_users: number;
  last_update: string;
  document_size: number;
}

// Editor types
export interface EditorSettings {
  fontSize: number;
  tabSize: number;
  wordWrap: boolean;
  minimap: boolean;
  lineNumbers: boolean;
  autoSave: boolean;
  autoSaveDelay: number;
  theme: 'vs-dark' | 'vs-light';
}

export interface EditorState {
  currentFile: FileItem | null;
  openFiles: FileItem[];
  activeFileId: string | null;
  isDirty: boolean;
}

// API Error types
export interface APIError {
  error: string;
  message?: string;
  status?: number;
}
