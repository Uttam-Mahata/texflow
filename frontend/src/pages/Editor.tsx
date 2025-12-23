import React, { useState, useEffect, useRef, useCallback } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { ArrowLeft, Save, Settings, Users } from 'lucide-react';
import { api } from '@/services/api';
import { useEditor } from '@/contexts/EditorContext';
import { FileTree } from '@/components/FileTree';
import { MonacoEditor } from '@/components/MonacoEditor';
import { PDFViewer } from '@/components/PDFViewer';
import { CompilationPanel } from '@/components/CompilationPanel';
import type { Project, FileItem } from '@/types';

export const Editor: React.FC = () => {
  const { projectId } = useParams<{ projectId: string }>();
  const navigate = useNavigate();
  const { currentFile, openFile, setCurrentFile, isDirty, setDirty } = useEditor();

  const [project, setProject] = useState<Project | null>(null);
  const [files, setFiles] = useState<FileItem[]>([]);
  const [pdfUrl, setPdfUrl] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isSaving, setIsSaving] = useState(false);
  const [showSettings, setShowSettings] = useState(false);
  const [fileContent, setFileContent] = useState<Map<string, string>>(new Map());
  const autoSaveTimeoutRef = useRef<NodeJS.Timeout | null>(null);

  useEffect(() => {
    if (projectId) {
      loadProject();
      loadFiles();
    }
  }, [projectId]);

  const loadProject = async () => {
    if (!projectId) return;

    try {
      const data = await api.getProject(projectId);
      setProject(data);
    } catch (err) {
      console.error('Failed to load project:', err);
      navigate('/projects');
    }
  };

  const loadFiles = async () => {
    if (!projectId) return;

    try {
      setIsLoading(true);
      const data = await api.getProjectFiles(projectId);
      setFiles(data);

      // Auto-open main.tex if it exists
      const mainTex = data.find((f) => f.name === 'main.tex');
      if (mainTex && !currentFile) {
        await handleFileSelect(mainTex);
      }
    } catch (err) {
      console.error('Failed to load files:', err);
    } finally {
      setIsLoading(false);
    }
  };

  // Cleanup auto-save timeout on unmount or file change
  useEffect(() => {
    return () => {
      if (autoSaveTimeoutRef.current) {
        clearTimeout(autoSaveTimeoutRef.current);
      }
    };
  }, [currentFile?.id]);

  // Handle content changes with auto-save (5 second delay)
  const handleContentChange = useCallback((newContent: string) => {
    if (!currentFile) return;

    // Update local state
    setFileContent((prev) => new Map(prev).set(currentFile.id, newContent));
    setDirty(true);

    // Clear existing auto-save timeout
    if (autoSaveTimeoutRef.current) {
      clearTimeout(autoSaveTimeoutRef.current);
    }

    // Set new auto-save timeout (5 seconds)
    autoSaveTimeoutRef.current = setTimeout(() => {
      handleFileSave(newContent);
    }, 5000);
  }, [currentFile, setDirty]);

  const handleFileSelect = async (file: FileItem) => {
    if (!projectId) return;

    try {
      // Check if content is already loaded
      if (!fileContent.has(file.id)) {
        const content = await api.getFileContent(projectId, file.id);
        setFileContent((prev) => new Map(prev).set(file.id, content));
      }

      openFile(file);
    } catch (err) {
      console.error('Failed to load file content:', err);
    }
  };

  const handleFileSave = async (content: string) => {
    if (!projectId || !currentFile) return;

    try {
      setIsSaving(true);
      await api.updateFile(projectId, currentFile.id, { content });
      setFileContent((prev) => new Map(prev).set(currentFile.id, content));
      setDirty(false);
    } catch (err) {
      console.error('Failed to save file:', err);
    } finally {
      setIsSaving(false);
    }
  };

  const handleFileCreate = async (path: string, name: string) => {
    if (!projectId) return;

    try {
      const newFile = await api.createFile(projectId, {
        name,
        path,
        content: '',
        content_type: 'text/plain',
      });

      setFiles([...files, newFile]);
      setFileContent((prev) => new Map(prev).set(newFile.id, ''));
      openFile(newFile);
    } catch (err) {
      console.error('Failed to create file:', err);
    }
  };

  const handleFileDelete = async (fileId: string) => {
    if (!projectId) return;

    try {
      await api.deleteFile(projectId, fileId);
      setFiles(files.filter((f) => f.id !== fileId));
      setFileContent((prev) => {
        const newMap = new Map(prev);
        newMap.delete(fileId);
        return newMap;
      });

      if (currentFile?.id === fileId) {
        setCurrentFile(null);
      }
    } catch (err) {
      console.error('Failed to delete file:', err);
    }
  };

  const handleCompilationComplete = (outputUrl: string) => {
    console.log('🎬 Editor: handleCompilationComplete called with URL:', outputUrl);
    setPdfUrl(outputUrl);
    console.log('🎬 Editor: pdfUrl state updated to:', outputUrl);
  };

  if (!projectId) {
    return null;
  }

  return (
    <div className="h-screen flex flex-col bg-gray-100">
      {/* Header */}
      <header className="flex items-center justify-between px-4 py-2 bg-white border-b border-gray-200">
        <div className="flex items-center space-x-4">
          <button
            onClick={() => navigate('/projects')}
            className="p-2 hover:bg-gray-100 rounded"
            title="Back to projects"
          >
            <ArrowLeft className="w-5 h-5 text-gray-600" />
          </button>

          <div>
            <h1 className="text-lg font-semibold text-gray-900">
              {project?.name || 'Loading...'}
            </h1>
            {project?.description && (
              <p className="text-xs text-gray-500">{project.description}</p>
            )}
          </div>

          {isDirty && (
            <span className="inline-flex items-center px-2 py-1 rounded-full text-xs font-medium bg-orange-100 text-orange-800">
              Unsaved
            </span>
          )}
        </div>

        <div className="flex items-center space-x-2">
          {currentFile && (
            <button
              onClick={() => currentFile && handleFileSave(fileContent.get(currentFile.id) || '')}
              disabled={isSaving || !isDirty}
              className="inline-flex items-center px-3 py-1.5 border border-transparent text-sm font-medium rounded-md text-white bg-primary-600 hover:bg-primary-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-primary-500 disabled:opacity-50 disabled:cursor-not-allowed"
            >
              <Save className="w-4 h-4 mr-2" />
              {isSaving ? 'Saving...' : 'Save'}
            </button>
          )}

          <button
            onClick={() => setShowSettings(!showSettings)}
            className="p-2 hover:bg-gray-100 rounded"
            title="Settings"
          >
            <Settings className="w-5 h-5 text-gray-600" />
          </button>

          {project && project.collaborators && project.collaborators.length > 1 && (
            <div className="flex items-center px-3 py-1.5 bg-gray-100 rounded-md">
              <Users className="w-4 h-4 mr-2 text-gray-600" />
              <span className="text-sm text-gray-700">{project.collaborators.length}</span>
            </div>
          )}
        </div>
      </header>

      {/* Main Editor Area */}
      <div className="flex-1 flex overflow-hidden">
        {/* File Tree */}
        <div className="w-64 flex-shrink-0">
          <FileTree
            files={files}
            currentFileId={currentFile?.id || null}
            onFileSelect={handleFileSelect}
            onFileCreate={handleFileCreate}
            onFileDelete={handleFileDelete}
          />
        </div>

        {/* Editor */}
        <div className="flex-1 min-w-0">
          {currentFile ? (
            <MonacoEditor
              key={currentFile.id}
              file={currentFile}
              projectId={projectId}
              content={fileContent.get(currentFile.id) || ''}
              onChange={handleContentChange}
              onSave={handleFileSave}
            />
          ) : (
            <div className="h-full flex items-center justify-center bg-gray-50">
              <div className="text-center">
                <div className="text-gray-400 mb-2">
                  <svg className="mx-auto h-12 w-12" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
                  </svg>
                </div>
                <p className="text-sm text-gray-500">No file selected</p>
                <p className="text-xs text-gray-400 mt-1">Select a file from the tree to start editing</p>
              </div>
            </div>
          )}
        </div>

        {/* PDF Viewer */}
        <div className="w-1/3 flex-shrink-0">
          <PDFViewer url={pdfUrl} />
        </div>

        {/* Compilation Panel */}
        <div className="w-80 flex-shrink-0">
          <CompilationPanel
            projectId={projectId}
            onCompilationComplete={handleCompilationComplete}
          />
        </div>
      </div>
    </div>
  );
};
