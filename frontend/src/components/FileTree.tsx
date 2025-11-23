import React, { useState } from 'react';
import { File, Folder, FolderOpen, Plus, Trash2, ChevronRight, ChevronDown } from 'lucide-react';
import type { FileItem, FileNode } from '@/types';

interface FileTreeProps {
  files: FileItem[];
  currentFileId: string | null;
  onFileSelect: (file: FileItem) => void;
  onFileCreate?: (path: string, name: string) => void;
  onFileDelete?: (fileId: string) => void;
}

export const FileTree: React.FC<FileTreeProps> = ({
  files,
  currentFileId,
  onFileSelect,
  onFileCreate,
  onFileDelete,
}) => {
  const [expandedFolders, setExpandedFolders] = useState<Set<string>>(new Set(['/']));
  const [showNewFileModal, setShowNewFileModal] = useState(false);
  const [newFileName, setNewFileName] = useState('');
  const [newFilePath, setNewFilePath] = useState('/');

  // Build file tree structure from flat file list
  const buildFileTree = (files: FileItem[]): FileNode[] => {
    const root: FileNode[] = [];
    const pathMap = new Map<string, FileNode>();

    files.forEach((file) => {
      const parts = file.path.split('/').filter(Boolean);
      let currentPath = '';

      parts.forEach((part, index) => {
        const parentPath = currentPath;
        currentPath = currentPath ? `${currentPath}/${part}` : part;

        if (!pathMap.has(currentPath)) {
          const isFile = index === parts.length - 1;
          const node: FileNode = {
            name: part,
            path: currentPath,
            type: isFile ? 'file' : 'folder',
            children: isFile ? undefined : [],
            file: isFile ? file : undefined,
          };

          pathMap.set(currentPath, node);

          if (parentPath) {
            const parent = pathMap.get(parentPath);
            if (parent && parent.children) {
              parent.children.push(node);
            }
          } else {
            root.push(node);
          }
        }
      });
    });

    return root;
  };

  const toggleFolder = (path: string) => {
    setExpandedFolders((prev) => {
      const newSet = new Set(prev);
      if (newSet.has(path)) {
        newSet.delete(path);
      } else {
        newSet.add(path);
      }
      return newSet;
    });
  };

  const handleCreateFile = () => {
    if (newFileName && onFileCreate) {
      const fullPath = newFilePath === '/' ? `/${newFileName}` : `${newFilePath}/${newFileName}`;
      onFileCreate(fullPath, newFileName);
      setShowNewFileModal(false);
      setNewFileName('');
      setNewFilePath('/');
    }
  };

  const renderNode = (node: FileNode, depth: number = 0): React.ReactNode => {
    const isExpanded = expandedFolders.has(node.path);
    const isSelected = node.file?.id === currentFileId;

    if (node.type === 'folder') {
      return (
        <div key={node.path}>
          <div
            className={`flex items-center px-2 py-1.5 text-sm cursor-pointer hover:bg-gray-100 ${
              isExpanded ? 'bg-gray-50' : ''
            }`}
            style={{ paddingLeft: `${depth * 12 + 8}px` }}
            onClick={() => toggleFolder(node.path)}
          >
            {isExpanded ? (
              <ChevronDown className="w-4 h-4 mr-1 text-gray-600" />
            ) : (
              <ChevronRight className="w-4 h-4 mr-1 text-gray-600" />
            )}
            {isExpanded ? (
              <FolderOpen className="w-4 h-4 mr-2 text-blue-500" />
            ) : (
              <Folder className="w-4 h-4 mr-2 text-blue-500" />
            )}
            <span className="text-gray-700">{node.name}</span>
          </div>
          {isExpanded && node.children && (
            <div>
              {node.children
                .sort((a, b) => {
                  if (a.type !== b.type) return a.type === 'folder' ? -1 : 1;
                  return a.name.localeCompare(b.name);
                })
                .map((child) => renderNode(child, depth + 1))}
            </div>
          )}
        </div>
      );
    }

    return (
      <div
        key={node.path}
        className={`flex items-center justify-between px-2 py-1.5 text-sm cursor-pointer hover:bg-gray-100 group ${
          isSelected ? 'bg-primary-50 text-primary-700' : 'text-gray-700'
        }`}
        style={{ paddingLeft: `${depth * 12 + 28}px` }}
        onClick={() => node.file && onFileSelect(node.file)}
      >
        <div className="flex items-center flex-1 min-w-0">
          <File className={`w-4 h-4 mr-2 flex-shrink-0 ${isSelected ? 'text-primary-600' : 'text-gray-400'}`} />
          <span className="truncate">{node.name}</span>
        </div>
        {onFileDelete && (
          <button
            onClick={(e) => {
              e.stopPropagation();
              if (node.file && confirm(`Delete ${node.name}?`)) {
                onFileDelete(node.file.id);
              }
            }}
            className="opacity-0 group-hover:opacity-100 p-1 hover:bg-red-100 rounded"
          >
            <Trash2 className="w-3 h-3 text-red-600" />
          </button>
        )}
      </div>
    );
  };

  const fileTree = buildFileTree(files);

  return (
    <div className="h-full flex flex-col bg-white border-r border-gray-200">
      <div className="flex items-center justify-between p-3 border-b border-gray-200">
        <h3 className="text-sm font-medium text-gray-700">Files</h3>
        {onFileCreate && (
          <button
            onClick={() => setShowNewFileModal(true)}
            className="p-1 hover:bg-gray-100 rounded"
            title="New file"
          >
            <Plus className="w-4 h-4 text-gray-600" />
          </button>
        )}
      </div>

      <div className="flex-1 overflow-y-auto">
        {files.length === 0 ? (
          <div className="p-4 text-center text-sm text-gray-500">
            No files yet
          </div>
        ) : (
          <div className="py-2">
            {fileTree
              .sort((a, b) => {
                if (a.type !== b.type) return a.type === 'folder' ? -1 : 1;
                return a.name.localeCompare(b.name);
              })
              .map((node) => renderNode(node))}
          </div>
        )}
      </div>

      {/* New File Modal */}
      {showNewFileModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black bg-opacity-50">
          <div className="bg-white rounded-lg p-6 w-96">
            <h3 className="text-lg font-medium mb-4">Create New File</h3>
            <div className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  File Name
                </label>
                <input
                  type="text"
                  value={newFileName}
                  onChange={(e) => setNewFileName(e.target.value)}
                  placeholder="main.tex"
                  className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-primary-500"
                  autoFocus
                />
              </div>
              <div className="flex justify-end space-x-2">
                <button
                  onClick={() => setShowNewFileModal(false)}
                  className="px-4 py-2 text-sm text-gray-700 hover:bg-gray-100 rounded"
                >
                  Cancel
                </button>
                <button
                  onClick={handleCreateFile}
                  disabled={!newFileName}
                  className="px-4 py-2 text-sm text-white bg-primary-600 hover:bg-primary-700 rounded disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  Create
                </button>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};
