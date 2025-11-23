import React, { useState, useEffect } from 'react';
import { Play, AlertCircle, CheckCircle, Clock, X } from 'lucide-react';
import { api } from '@/services/api';
import type { Compilation, CompilationStatus } from '@/types';

interface CompilationPanelProps {
  projectId: string;
  onCompilationComplete?: (outputUrl: string) => void;
}

export const CompilationPanel: React.FC<CompilationPanelProps> = ({
  projectId,
  onCompilationComplete,
}) => {
  const [isCompiling, setIsCompiling] = useState(false);
  const [currentCompilation, setCurrentCompilation] = useState<Compilation | null>(null);
  const [compiler, setCompiler] = useState<'pdflatex' | 'xelatex' | 'lualatex'>('pdflatex');
  const [mainFile, setMainFile] = useState('main.tex');
  const [showLog, setShowLog] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    // Poll for compilation status if compiling
    if (currentCompilation && (currentCompilation.status === 'queued' || currentCompilation.status === 'running')) {
      const interval = setInterval(async () => {
        try {
          const updated = await api.getCompilation(currentCompilation.id);
          setCurrentCompilation(updated);

          if (updated.status === 'completed') {
            setIsCompiling(false);
            if (updated.output_url && onCompilationComplete) {
              onCompilationComplete(updated.output_url);
            }
          } else if (updated.status === 'failed' || updated.status === 'timeout') {
            setIsCompiling(false);
            setError(updated.error || 'Compilation failed');
          }
        } catch (err) {
          console.error('Failed to check compilation status:', err);
        }
      }, 2000); // Poll every 2 seconds

      return () => clearInterval(interval);
    }
  }, [currentCompilation, onCompilationComplete]);

  const handleCompile = async () => {
    setError(null);
    setIsCompiling(true);
    setShowLog(false);

    try {
      const compilation = await api.compile({
        project_id: projectId,
        compiler,
        main_file: mainFile,
      });

      setCurrentCompilation(compilation);
    } catch (err: any) {
      setError(err.response?.data?.error || 'Failed to start compilation');
      setIsCompiling(false);
    }
  };

  const getStatusIcon = (status: CompilationStatus) => {
    switch (status) {
      case 'completed':
        return <CheckCircle className="w-5 h-5 text-green-600" />;
      case 'failed':
      case 'timeout':
        return <AlertCircle className="w-5 h-5 text-red-600" />;
      case 'running':
      case 'queued':
        return <Clock className="w-5 h-5 text-blue-600 animate-pulse" />;
      default:
        return null;
    }
  };

  const getStatusText = (status: CompilationStatus) => {
    switch (status) {
      case 'queued':
        return 'Queued';
      case 'running':
        return 'Compiling...';
      case 'completed':
        return 'Success';
      case 'failed':
        return 'Failed';
      case 'timeout':
        return 'Timeout';
      default:
        return status;
    }
  };

  return (
    <div className="h-full flex flex-col bg-white border-l border-gray-200">
      {/* Header */}
      <div className="p-4 border-b border-gray-200">
        <h3 className="text-sm font-medium text-gray-700 mb-3">Compilation</h3>

        <div className="space-y-3">
          {/* Compiler Selection */}
          <div>
            <label className="block text-xs font-medium text-gray-600 mb-1">
              Compiler
            </label>
            <select
              value={compiler}
              onChange={(e) => setCompiler(e.target.value as any)}
              disabled={isCompiling}
              className="w-full px-3 py-1.5 text-sm border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-primary-500 disabled:opacity-50"
            >
              <option value="pdflatex">pdfLaTeX</option>
              <option value="xelatex">XeLaTeX</option>
              <option value="lualatex">LuaLaTeX</option>
            </select>
          </div>

          {/* Main File */}
          <div>
            <label className="block text-xs font-medium text-gray-600 mb-1">
              Main File
            </label>
            <input
              type="text"
              value={mainFile}
              onChange={(e) => setMainFile(e.target.value)}
              disabled={isCompiling}
              className="w-full px-3 py-1.5 text-sm border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-primary-500 disabled:opacity-50"
              placeholder="main.tex"
            />
          </div>

          {/* Compile Button */}
          <button
            onClick={handleCompile}
            disabled={isCompiling || !mainFile}
            className="w-full inline-flex items-center justify-center px-4 py-2 border border-transparent text-sm font-medium rounded-md text-white bg-primary-600 hover:bg-primary-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-primary-500 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            <Play className="w-4 h-4 mr-2" />
            {isCompiling ? 'Compiling...' : 'Compile'}
          </button>
        </div>
      </div>

      {/* Status */}
      {currentCompilation && (
        <div className="p-4 border-b border-gray-200">
          <div className="flex items-center space-x-2 mb-2">
            {getStatusIcon(currentCompilation.status)}
            <span className="text-sm font-medium text-gray-700">
              {getStatusText(currentCompilation.status)}
            </span>
          </div>

          {currentCompilation.duration_ms && (
            <p className="text-xs text-gray-500">
              Duration: {(currentCompilation.duration_ms / 1000).toFixed(2)}s
            </p>
          )}

          {currentCompilation.log && (
            <button
              onClick={() => setShowLog(!showLog)}
              className="mt-2 text-xs text-primary-600 hover:text-primary-700"
            >
              {showLog ? 'Hide' : 'Show'} Log
            </button>
          )}
        </div>
      )}

      {/* Error */}
      {error && (
        <div className="p-4 bg-red-50 border-b border-red-200">
          <div className="flex items-start">
            <AlertCircle className="w-4 h-4 text-red-600 mt-0.5 mr-2 flex-shrink-0" />
            <p className="text-xs text-red-800">{error}</p>
          </div>
        </div>
      )}

      {/* Log Viewer */}
      {showLog && currentCompilation?.log && (
        <div className="flex-1 overflow-hidden flex flex-col">
          <div className="flex items-center justify-between px-4 py-2 bg-gray-50 border-b border-gray-200">
            <h4 className="text-xs font-medium text-gray-700">Compilation Log</h4>
            <button
              onClick={() => setShowLog(false)}
              className="p-1 hover:bg-gray-200 rounded"
            >
              <X className="w-4 h-4 text-gray-600" />
            </button>
          </div>
          <div className="flex-1 overflow-auto p-4 bg-gray-900">
            <pre className="text-xs text-gray-100 font-mono whitespace-pre-wrap">
              {currentCompilation.log}
            </pre>
          </div>
        </div>
      )}

      {/* Recent Compilations (when not showing log) */}
      {!showLog && (
        <div className="flex-1 overflow-auto">
          <div className="p-4">
            <h4 className="text-xs font-medium text-gray-600 mb-2">Recent</h4>
            {/* This would show recent compilations - simplified for now */}
            <p className="text-xs text-gray-500">
              {currentCompilation ? 'Latest compilation shown above' : 'No compilations yet'}
            </p>
          </div>
        </div>
      )}
    </div>
  );
};
