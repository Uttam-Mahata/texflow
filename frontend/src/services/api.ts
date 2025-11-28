import axios, { AxiosInstance, AxiosError } from 'axios';
import type {
  AuthResponse,
  LoginRequest,
  RegisterRequest,
  Project,
  CreateProjectRequest,
  UpdateProjectRequest,
  ShareProjectRequest,
  FileItem,
  CreateFileRequest,
  UpdateFileRequest,
  Compilation,
  CompileRequest,
  CompilationStats,
  QueueStats,
  DocumentState,
  CollaborationMetrics,
} from '@/types';

const API_BASE_URL = import.meta.env.VITE_API_URL || 'http://localhost:8000';

class APIClient {
  private client: AxiosInstance;

  constructor() {
    this.client = axios.create({
      baseURL: API_BASE_URL,
      timeout: 30000,
      headers: {
        'Content-Type': 'application/json',
      },
    });

    // Request interceptor to add auth token and user ID
    this.client.interceptors.request.use(
      (config) => {
        const token = localStorage.getItem('access_token');
        if (token) {
          config.headers.Authorization = `Bearer ${token}`;

          // Extract user ID from JWT token and add as header
          try {
            const payload = JSON.parse(atob(token.split('.')[1]));
            // Auth service uses 'user_id' claim in access tokens
            if (payload.user_id) {
              config.headers['X-User-ID'] = payload.user_id;
            }
          } catch (e) {
            console.error('Failed to decode JWT token:', e);
          }
        }
        return config;
      },
      (error) => Promise.reject(error)
    );

    // Response interceptor to handle token refresh
    this.client.interceptors.response.use(
      (response) => response,
      async (error: AxiosError) => {
        const originalRequest = error.config as any;

        // Skip refresh logic for auth endpoints to prevent infinite loops
        const isAuthEndpoint = originalRequest?.url?.includes('/api/v1/auth/');

        if (error.response?.status === 401 && !originalRequest._retry && !isAuthEndpoint) {
          originalRequest._retry = true;

          try {
            const refreshToken = localStorage.getItem('refresh_token');
            if (!refreshToken) {
              throw new Error('No refresh token');
            }

            // Use axios directly to avoid interceptor loop
            const response = await axios.post<AuthResponse>(
              `${API_BASE_URL}/api/v1/auth/refresh`,
              { refresh_token: refreshToken },
              { headers: { 'Content-Type': 'application/json' } }
            );

            localStorage.setItem('access_token', response.data.access_token);
            localStorage.setItem('refresh_token', response.data.refresh_token);

            originalRequest.headers.Authorization = `Bearer ${response.data.access_token}`;
            return this.client(originalRequest);
          } catch (refreshError) {
            // Refresh failed, logout user
            localStorage.removeItem('access_token');
            localStorage.removeItem('refresh_token');
            localStorage.removeItem('user');
            localStorage.removeItem('expires_at');
            window.location.href = '/login';
            return Promise.reject(refreshError);
          }
        }

        return Promise.reject(error);
      }
    );
  }

  // Auth API
  async login(data: LoginRequest): Promise<AuthResponse> {
    const response = await this.client.post<AuthResponse>('/api/v1/auth/login', data);
    return response.data;
  }

  async register(data: RegisterRequest): Promise<AuthResponse> {
    const response = await this.client.post<AuthResponse>('/api/v1/auth/register', data);
    return response.data;
  }

  async refreshToken(refreshToken: string): Promise<AuthResponse> {
    const response = await this.client.post<AuthResponse>('/api/v1/auth/refresh', {
      refresh_token: refreshToken,
    });
    return response.data;
  }

  async logout(): Promise<void> {
    await this.client.post('/api/v1/auth/logout');
  }

  // Project API
  async getProjects(): Promise<Project[]> {
    const response = await this.client.get<{ data: Project[] }>('/api/v1/projects');
    return response.data.data || [];
  }

  async getProject(projectId: string): Promise<Project> {
    const response = await this.client.get<Project>(`/api/v1/projects/${projectId}`);
    return response.data;
  }

  async createProject(data: CreateProjectRequest): Promise<Project> {
    const response = await this.client.post<Project>('/api/v1/projects', data);
    return response.data;
  }

  async updateProject(projectId: string, data: UpdateProjectRequest): Promise<Project> {
    const response = await this.client.put<Project>(`/api/v1/projects/${projectId}`, data);
    return response.data;
  }

  async deleteProject(projectId: string): Promise<void> {
    await this.client.delete(`/api/v1/projects/${projectId}`);
  }

  async shareProject(projectId: string, data: ShareProjectRequest): Promise<void> {
    await this.client.post(`/api/v1/projects/${projectId}/share`, data);
  }

  async removeCollaborator(projectId: string, userId: string): Promise<void> {
    await this.client.delete(`/api/v1/projects/${projectId}/collaborators/${userId}`);
  }

  // File API
  async getProjectFiles(projectId: string): Promise<FileItem[]> {
    const response = await this.client.get<FileItem[]>(`/api/v1/projects/${projectId}/files`);
    return response.data || [];
  }

  async getFile(projectId: string, fileId: string): Promise<FileItem> {
    const response = await this.client.get<FileItem>(
      `/api/v1/projects/${projectId}/files/${fileId}`
    );
    return response.data;
  }

  async createFile(projectId: string, data: CreateFileRequest): Promise<FileItem> {
    const response = await this.client.post<FileItem>(
      `/api/v1/projects/${projectId}/files`,
      data
    );
    return response.data;
  }

  async updateFile(projectId: string, fileId: string, data: UpdateFileRequest): Promise<FileItem> {
    const response = await this.client.put<FileItem>(
      `/api/v1/projects/${projectId}/files/${fileId}`,
      data
    );
    return response.data;
  }

  async deleteFile(projectId: string, fileId: string): Promise<void> {
    await this.client.delete(`/api/v1/projects/${projectId}/files/${fileId}`);
  }

  async getFileContent(projectId: string, fileId: string): Promise<string> {
    const response = await this.client.get<{ content: string }>(
      `/api/v1/projects/${projectId}/files/${fileId}/content`
    );
    return response.data.content;
  }

  // Compilation API
  async compile(data: CompileRequest): Promise<Compilation> {
    const response = await this.client.post<Compilation>('/api/v1/compilation/compile', data);
    return response.data;
  }

  async getCompilation(compilationId: string): Promise<Compilation> {
    const response = await this.client.get<Compilation>(`/api/v1/compilation/${compilationId}`);
    return response.data;
  }

  async getProjectCompilations(projectId: string, limit = 20): Promise<Compilation[]> {
    const response = await this.client.get<Compilation[]>(
      `/api/v1/compilation/project/${projectId}?limit=${limit}`
    );
    return response.data;
  }

  async getCompilationStats(): Promise<CompilationStats> {
    const response = await this.client.get<CompilationStats>('/api/v1/compilation/stats');
    return response.data;
  }

  async getQueueStats(): Promise<QueueStats> {
    const response = await this.client.get<QueueStats>('/api/v1/compilation/queue');
    return response.data;
  }

  // Collaboration API
  async getDocumentState(
    projectId: string,
    documentName: string,
    sinceVersion = 0
  ): Promise<DocumentState> {
    const response = await this.client.get<DocumentState>(
      `/api/v1/collaboration/state/${projectId}/${documentName}?since_version=${sinceVersion}`
    );
    return response.data;
  }

  async getCollaborationMetrics(
    projectId: string,
    documentName: string
  ): Promise<CollaborationMetrics> {
    const response = await this.client.get<CollaborationMetrics>(
      `/api/v1/collaboration/metrics/${projectId}/${documentName}`
    );
    return response.data;
  }
}

export const api = new APIClient();
export default api;
