import React, { createContext, useContext, useState, useCallback, ReactNode } from 'react';
import type { FileItem, EditorSettings } from '@/types';

interface EditorContextType {
  currentFile: FileItem | null;
  openFiles: FileItem[];
  activeFileId: string | null;
  isDirty: boolean;
  settings: EditorSettings;
  setCurrentFile: (file: FileItem | null) => void;
  openFile: (file: FileItem) => void;
  closeFile: (fileId: string) => void;
  setActiveFile: (fileId: string) => void;
  setDirty: (dirty: boolean) => void;
  updateSettings: (settings: Partial<EditorSettings>) => void;
}

const EditorContext = createContext<EditorContextType | undefined>(undefined);

export const useEditor = () => {
  const context = useContext(EditorContext);
  if (!context) {
    throw new Error('useEditor must be used within EditorProvider');
  }
  return context;
};

interface EditorProviderProps {
  children: ReactNode;
}

const DEFAULT_SETTINGS: EditorSettings = {
  fontSize: 14,
  tabSize: 2,
  wordWrap: true,
  minimap: false,
  lineNumbers: true,
  autoSave: true,
  autoSaveDelay: 2000,
  theme: 'vs-dark',
};

export const EditorProvider: React.FC<EditorProviderProps> = ({ children }) => {
  const [currentFile, setCurrentFile] = useState<FileItem | null>(null);
  const [openFiles, setOpenFiles] = useState<FileItem[]>([]);
  const [activeFileId, setActiveFileId] = useState<string | null>(null);
  const [isDirty, setIsDirty] = useState(false);
  const [settings, setSettings] = useState<EditorSettings>(() => {
    const stored = localStorage.getItem('editor_settings');
    return stored ? { ...DEFAULT_SETTINGS, ...JSON.parse(stored) } : DEFAULT_SETTINGS;
  });

  const openFile = useCallback((file: FileItem) => {
    setOpenFiles((prev) => {
      // Check if file is already open
      if (prev.some((f) => f.id === file.id)) {
        return prev;
      }
      return [...prev, file];
    });
    setCurrentFile(file);
    setActiveFileId(file.id);
    setIsDirty(false);
  }, []);

  const closeFile = useCallback((fileId: string) => {
    setOpenFiles((prev) => {
      const newFiles = prev.filter((f) => f.id !== fileId);

      // If closing the active file, switch to another file
      if (activeFileId === fileId) {
        if (newFiles.length > 0) {
          const nextFile = newFiles[newFiles.length - 1];
          setCurrentFile(nextFile);
          setActiveFileId(nextFile.id);
        } else {
          setCurrentFile(null);
          setActiveFileId(null);
        }
      }

      return newFiles;
    });
    setIsDirty(false);
  }, [activeFileId]);

  const setActiveFile = useCallback((fileId: string) => {
    const file = openFiles.find((f) => f.id === fileId);
    if (file) {
      setCurrentFile(file);
      setActiveFileId(fileId);
      setIsDirty(false);
    }
  }, [openFiles]);

  const setDirty = useCallback((dirty: boolean) => {
    setIsDirty(dirty);
  }, []);

  const updateSettings = useCallback((newSettings: Partial<EditorSettings>) => {
    setSettings((prev) => {
      const updated = { ...prev, ...newSettings };
      localStorage.setItem('editor_settings', JSON.stringify(updated));
      return updated;
    });
  }, []);

  const value: EditorContextType = {
    currentFile,
    openFiles,
    activeFileId,
    isDirty,
    settings,
    setCurrentFile,
    openFile,
    closeFile,
    setActiveFile,
    setDirty,
    updateSettings,
  };

  return <EditorContext.Provider value={value}>{children}</EditorContext.Provider>;
};
