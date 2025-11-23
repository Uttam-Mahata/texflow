# TexFlow Frontend

Modern, collaborative LaTeX editor frontend built with React, TypeScript, and Monaco Editor.

## Features

- **Monaco Editor Integration**: Professional code editing experience with syntax highlighting for LaTeX
- **Real-time Collaboration**: Multiple users can edit documents simultaneously using Yjs CRDT
- **Live PDF Preview**: Instant PDF rendering and preview with zoom controls
- **Project Management**: Create, organize, and share LaTeX projects
- **File Management**: Hierarchical file tree with create/delete operations
- **Compilation**: Compile LaTeX documents with pdfLaTeX, XeLaTeX, or LuaLaTeX
- **User Presence**: See who's online and editing with cursor/selection tracking
- **Responsive Design**: Clean, modern UI with Tailwind CSS

## Tech Stack

- **React 18** - UI framework
- **TypeScript** - Type safety
- **Vite** - Build tool and dev server
- **React Router** - Client-side routing
- **Monaco Editor** - Code editor (VS Code editor)
- **Yjs** - CRDT for real-time collaboration
- **Axios** - HTTP client
- **Tailwind CSS** - Utility-first CSS framework
- **Lucide React** - Icon library
- **React PDF** - PDF rendering

## Project Structure

```
frontend/
├── src/
│   ├── components/          # Reusable React components
│   │   ├── MonacoEditor.tsx  # Monaco Editor with Yjs binding
│   │   ├── FileTree.tsx      # File explorer
│   │   ├── PDFViewer.tsx     # PDF preview
│   │   └── CompilationPanel.tsx
│   ├── pages/                # Page components
│   │   ├── Login.tsx
│   │   ├── Register.tsx
│   │   ├── Projects.tsx      # Project list
│   │   └── Editor.tsx        # Main editor page
│   ├── contexts/             # React contexts
│   │   ├── AuthContext.tsx   # Authentication state
│   │   └── EditorContext.tsx # Editor state
│   ├── services/             # API and WebSocket services
│   │   ├── api.ts            # REST API client
│   │   └── websocket.ts      # WebSocket client
│   ├── types/                # TypeScript type definitions
│   │   └── index.ts
│   ├── utils/                # Utility functions
│   ├── App.tsx               # Main app component
│   ├── main.tsx              # Entry point
│   └── index.css             # Global styles
├── public/                   # Static assets
├── index.html                # HTML template
├── vite.config.ts            # Vite configuration
├── tsconfig.json             # TypeScript configuration
├── tailwind.config.js        # Tailwind CSS configuration
├── Dockerfile                # Docker build configuration
└── package.json              # Dependencies and scripts
```

## Development

### Prerequisites

- Node.js 18+
- npm or yarn

### Installation

```bash
# Install dependencies
npm install

# Copy environment variables
cp .env.example .env

# Update .env with your API endpoints
VITE_API_URL=http://localhost:8080
VITE_WS_URL=ws://localhost:8082
```

### Run Development Server

```bash
npm run dev
```

The application will be available at `http://localhost:3000`.

### Build for Production

```bash
npm run build
```

The built files will be in the `dist/` directory.

### Preview Production Build

```bash
npm run preview
```

### Linting

```bash
npm run lint
```

## Key Components

### MonacoEditor

Monaco Editor component with Yjs collaboration integration.

**Features:**
- LaTeX syntax highlighting
- Real-time collaborative editing
- Cursor and selection synchronization
- Auto-save functionality
- Customizable settings (theme, font size, etc.)

**Usage:**
```tsx
<MonacoEditor
  file={currentFile}
  projectId={projectId}
  onSave={handleSave}
/>
```

### FileTree

Hierarchical file explorer with folder expansion.

**Features:**
- File and folder navigation
- Create/delete files
- Visual indication of selected file
- Collapsible folders

### PDFViewer

PDF preview with controls.

**Features:**
- Page navigation
- Zoom in/out
- Download PDF
- Responsive layout

### CompilationPanel

Compilation control panel.

**Features:**
- Compiler selection (pdfLaTeX, XeLaTeX, LuaLaTeX)
- Main file configuration
- Real-time compilation status
- Compilation log viewer
- Error messages

## Contexts

### AuthContext

Manages user authentication state.

**API:**
```tsx
const {
  user,              // Current user
  isAuthenticated,   // Authentication status
  isLoading,         // Loading state
  login,             // Login function
  register,          // Register function
  logout,            // Logout function
} = useAuth();
```

### EditorContext

Manages editor state and settings.

**API:**
```tsx
const {
  currentFile,       // Currently open file
  openFiles,         // List of open files
  activeFileId,      // Active file ID
  isDirty,           // Unsaved changes
  settings,          // Editor settings
  openFile,          // Open a file
  closeFile,         // Close a file
  setActiveFile,     // Set active file
  updateSettings,    // Update editor settings
} = useEditor();
```

## API Integration

The frontend communicates with backend services through:

1. **REST API** (`/api/v1/*`)
   - Authentication
   - Project management
   - File operations
   - Compilation requests

2. **WebSocket** (`/ws`)
   - Real-time collaboration
   - Yjs updates
   - User presence
   - Cursor/selection updates

### API Client

```tsx
import { api } from '@/services/api';

// Authentication
await api.login({ email, password });
await api.register({ name, email, password });
await api.logout();

// Projects
const projects = await api.getProjects();
const project = await api.getProject(projectId);
await api.createProject({ name, description });
await api.deleteProject(projectId);

// Files
const files = await api.getProjectFiles(projectId);
const file = await api.getFile(projectId, fileId);
const content = await api.getFileContent(projectId, fileId);
await api.createFile(projectId, { name, path, content });
await api.updateFile(projectId, fileId, { content });

// Compilation
const compilation = await api.compile({
  project_id: projectId,
  compiler: 'pdflatex',
  main_file: 'main.tex'
});
```

### WebSocket Client

```tsx
import { websocketService } from '@/services/websocket';

// Connect to project
await websocketService.connect(projectId);

// Listen for messages
websocketService.onMessage((message) => {
  if (message.type === 'yjs_update') {
    // Handle Yjs update
  }
});

// Listen for presence
websocketService.onPresence((users) => {
  // Update connected users
});

// Send message
websocketService.send({
  type: 'cursor_update',
  payload: { cursor: { line: 10, column: 5 } }
});

// Disconnect
websocketService.disconnect();
```

## Styling

The application uses Tailwind CSS for styling with a custom color palette:

```js
primary: {
  50: '#f0f9ff',
  500: '#0ea5e9',
  600: '#0284c7',
  700: '#0369a1',
}
```

### Custom Utilities

- `.scrollbar-thin` - Thin scrollbar styling
- `.line-clamp-2` - Limit text to 2 lines

## Docker Deployment

### Build Docker Image

```bash
docker build -t texflow/frontend:latest .
```

### Run Container

```bash
docker run -p 3000:80 \
  -e VITE_API_URL=http://your-api-url \
  -e VITE_WS_URL=ws://your-ws-url \
  texflow/frontend:latest
```

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `VITE_API_URL` | Backend API URL | `http://localhost:8080` |
| `VITE_WS_URL` | WebSocket URL | `ws://localhost:8082` |
| `VITE_ENV` | Environment | `development` |

## Browser Support

- Chrome/Edge (latest)
- Firefox (latest)
- Safari (latest)

## Performance Optimizations

1. **Code Splitting**: Automatic route-based code splitting
2. **Lazy Loading**: Components loaded on demand
3. **Bundle Optimization**: Vendor chunks for better caching
4. **Asset Optimization**: Gzip compression in production
5. **Monaco Editor**: Lazy loaded with dynamic imports

## Troubleshooting

### Monaco Editor Not Loading

Ensure the Monaco Editor worker is properly configured:
```tsx
import { loader } from '@monaco-editor/react';
loader.config({ paths: { vs: '...' } });
```

### WebSocket Connection Failed

Check:
- WebSocket URL in `.env`
- CORS settings on backend
- Network connectivity
- JWT token validity

### PDF Not Rendering

Ensure:
- PDF.js worker is loaded
- PDF URL is accessible
- CORS headers allow PDF loading

## Contributing

1. Create a feature branch
2. Make changes
3. Run linter: `npm run lint`
4. Build: `npm run build`
5. Test locally
6. Submit pull request

## License

Part of the TexFlow platform.
