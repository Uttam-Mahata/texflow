import React, { useEffect, useRef, useState } from 'react';
import Editor, { OnMount } from '@monaco-editor/react';
import { editor } from 'monaco-editor';
import * as Y from 'yjs';
import { MonacoBinding } from 'y-monaco';
import { websocketService } from '@/services/websocket';
import { useEditor } from '@/contexts/EditorContext';
import type { FileItem, UserPresence } from '@/types';

interface MonacoEditorProps {
  file: FileItem;
  projectId: string;
  onSave?: (content: string) => void;
}

export const MonacoEditor: React.FC<MonacoEditorProps> = ({ file, projectId, onSave }) => {
  const { settings, setDirty } = useEditor();
  const editorRef = useRef<editor.IStandaloneCodeEditor | null>(null);
  const ydocRef = useRef<Y.Doc | null>(null);
  const bindingRef = useRef<MonacoBinding | null>(null);
  const [connectedUsers, setConnectedUsers] = useState<Map<string, UserPresence>>(new Map());

  useEffect(() => {
    // Initialize Yjs document
    ydocRef.current = new Y.Doc();
    const ytext = ydocRef.current.getText('monaco');

    // Connect to WebSocket for collaboration
    if (!websocketService.isConnected()) {
      websocketService.connect(projectId).catch((error) => {
        console.error('Failed to connect to WebSocket:', error);
      });
    }

    // Subscribe to presence updates
    const unsubscribe = websocketService.onPresence((users) => {
      setConnectedUsers(users);
    });

    // Subscribe to Yjs updates from WebSocket
    const unsubscribeMessages = websocketService.onMessage((message) => {
      if (message.type === 'yjs_update' && ydocRef.current) {
        const update = Uint8Array.from(atob(message.payload.update), (c) => c.charCodeAt(0));
        Y.applyUpdate(ydocRef.current, update);
      }
    });

    // Send Yjs updates to WebSocket
    const updateHandler = (update: Uint8Array) => {
      const base64Update = btoa(String.fromCharCode.apply(null, Array.from(update)));
      websocketService.send({
        type: 'yjs_update',
        payload: {
          document_name: file.path,
          update: base64Update,
        },
      });
    };

    ydocRef.current.on('update', updateHandler);

    return () => {
      unsubscribe();
      unsubscribeMessages();
      if (ydocRef.current) {
        ydocRef.current.off('update', updateHandler);
        ydocRef.current.destroy();
      }
      if (bindingRef.current) {
        bindingRef.current.destroy();
      }
    };
  }, [projectId, file.path]);

  const handleEditorDidMount: OnMount = (editor, monaco) => {
    editorRef.current = editor;

    // Create Monaco binding with Yjs
    if (ydocRef.current) {
      const ytext = ydocRef.current.getText('monaco');
      const model = editor.getModel();

      if (model) {
        bindingRef.current = new MonacoBinding(
          ytext,
          model,
          new Set([editor]),
          // Awareness for showing cursor positions
          null
        );
      }
    }

    // Add save command (Ctrl+S / Cmd+S)
    editor.addCommand(monaco.KeyMod.CtrlCmd | monaco.KeyCode.KeyS, () => {
      if (onSave) {
        const content = editor.getValue();
        onSave(content);
      }
    });

    // Track dirty state
    editor.onDidChangeModelContent(() => {
      setDirty(true);
    });

    // Send cursor updates
    editor.onDidChangeCursorPosition((e) => {
      websocketService.send({
        type: 'cursor_update',
        payload: {
          cursor: {
            line: e.position.lineNumber,
            column: e.position.column,
          },
        },
      });
    });

    // Send selection updates
    editor.onDidChangeCursorSelection((e) => {
      const selection = e.selection;
      websocketService.send({
        type: 'selection_update',
        payload: {
          selection: {
            start: {
              line: selection.startLineNumber,
              column: selection.startColumn,
            },
            end: {
              line: selection.endLineNumber,
              column: selection.endColumn,
            },
          },
        },
      });
    });
  };

  // Get file language from extension
  const getLanguage = (filename: string): string => {
    if (filename.endsWith('.tex')) return 'latex';
    if (filename.endsWith('.bib')) return 'bibtex';
    if (filename.endsWith('.md')) return 'markdown';
    if (filename.endsWith('.json')) return 'json';
    if (filename.endsWith('.yaml') || filename.endsWith('.yml')) return 'yaml';
    return 'plaintext';
  };

  return (
    <div className="relative h-full">
      {/* Connected Users Indicator */}
      {connectedUsers.size > 0 && (
        <div className="absolute top-2 right-2 z-10 flex items-center space-x-2 bg-white rounded-md shadow-sm px-3 py-1.5">
          <div className="flex -space-x-2">
            {Array.from(connectedUsers.values()).map((user) => (
              <div
                key={user.user_id}
                className="w-8 h-8 rounded-full flex items-center justify-center text-white text-xs font-medium ring-2 ring-white"
                style={{ backgroundColor: user.color }}
                title={user.user_name}
              >
                {user.user_name.charAt(0).toUpperCase()}
              </div>
            ))}
          </div>
          <span className="text-sm text-gray-600">{connectedUsers.size} online</span>
        </div>
      )}

      <Editor
        height="100%"
        language={getLanguage(file.name)}
        theme={settings.theme}
        options={{
          fontSize: settings.fontSize,
          tabSize: settings.tabSize,
          wordWrap: settings.wordWrap ? 'on' : 'off',
          minimap: { enabled: settings.minimap },
          lineNumbers: settings.lineNumbers ? 'on' : 'off',
          automaticLayout: true,
          scrollBeyondLastLine: false,
          renderWhitespace: 'selection',
          bracketPairColorization: { enabled: true },
          guides: {
            bracketPairs: true,
            indentation: true,
          },
        }}
        onMount={handleEditorDidMount}
      />
    </div>
  );
};
