package cli

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/echo-vcs/echo/internal/core"
	"github.com/echo-vcs/echo/internal/index"
	"github.com/echo-vcs/echo/internal/merkle"
	"github.com/spf13/cobra"
)

func init() {
	RootCmd.AddCommand(uiCmd)
}

var uiCmd = &cobra.Command{
	Use:   "ui",
	Short: "Start a local web-based dashboard and IDE for the Temporal Graph",
	Run: func(cmd *cobra.Command, args []string) {
		ws, err := GetWorkspace()
		if err != nil {
			HandleError(err)
		}

		dbPath := ws.EchoDir + "/index.db"
		idx, err := index.OpenIndex(dbPath)
		if err != nil {
			HandleError(fmt.Errorf("failed to open index: %w", err))
		}
		defer idx.Close()

		port := "8080"
		fmt.Printf("Starting ATCG Dashboard on http://localhost:%s\n", port)

		http.HandleFunc("/", serveIndex)
		
		http.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
			branch, _ := core.ReadHead(ws)
			headID, _ := core.GetRef(ws, branch)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"branch": branch,
				"head":   headID,
			})
		})

		http.HandleFunc("/api/files", func(w http.ResponseWriter, r *http.Request) {
			branch, _ := core.ReadHead(ws)
			headID, _ := core.GetRef(ws, branch)
			if headID == "" {
				json.NewEncoder(w).Encode([]string{})
				return
			}
			
			cp, err := core.LoadCheckpoint(ws, headID)
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			
			treeData, err := ws.Store.ReadTree(cp.TreeHash)
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			
			treeNode, err := merkle.DeserializeTree(treeData)
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			
			files, err := merkle.FlattenTree(treeNode, ws.Store)
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			
			var fileList []map[string]interface{}
			for p, f := range files {
				fileList = append(fileList, map[string]interface{}{
					"path": p,
					"size": f.Size,
					"hash": f.Hash,
				})
			}
			json.NewEncoder(w).Encode(fileList)
		})

		http.HandleFunc("/api/file", func(w http.ResponseWriter, r *http.Request) {
			hash := r.URL.Query().Get("hash")
			if hash == "" {
				http.Error(w, "missing hash", 400)
				return
			}
			
			blob, err := ws.Store.ReadBlob(hash)
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			w.Header().Set("Content-Type", "text/plain")
			w.Write(blob)
		})

		if err := http.ListenAndServe(":"+port, nil); err != nil {
			HandleError(err)
		}
	},
}

func serveIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	
	html := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>ATCG Studio</title>
    <script src="https://cdn.tailwindcss.com"></script>
    <style>
        /* Custom scrollbars like VSCode */
        ::-webkit-scrollbar { width: 10px; height: 10px; }
        ::-webkit-scrollbar-track { background: #1e1e1e; }
        ::-webkit-scrollbar-thumb { background: #424242; border-radius: 5px; }
        ::-webkit-scrollbar-thumb:hover { background: #4f4f4f; }
        
        .tree-line { border-left: 1px solid #404040; margin-left: 0.5rem; padding-left: 0.5rem; }
    </style>
</head>
<body class="bg-[#1e1e1e] text-[#cccccc] font-sans h-screen flex flex-col">
    <!-- Navbar -->
    <header class="bg-[#333333] border-b border-[#2d2d2d] px-4 py-2 flex justify-between items-center select-none">
        <div class="flex items-center gap-3">
            <svg class="w-5 h-5 text-blue-400" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19.428 15.428a2 2 0 00-1.022-.547l-2.387-.477a6 6 0 00-3.86.517l-.318.158a6 6 0 01-3.86.517L6.05 15.21a2 2 0 00-1.806.547M8 4h8l-1 1v5.172a2 2 0 00.586 1.414l5 5c1.26 1.26.367 3.414-1.415 3.414H4.828c-1.782 0-2.674-2.154-1.414-3.414l5-5A2 2 0 009 10.172V5L8 4z"></path></svg>
            <h1 class="text-sm font-semibold tracking-wide">ATCG Studio</h1>
        </div>
        <div id="status-bar" class="text-xs bg-[#252526] text-[#9cdcfe] px-3 py-1 rounded border border-[#3c3c3c]">Loading status...</div>
    </header>

    <!-- Main Content -->
    <div class="flex flex-1 overflow-hidden">
        <!-- Sidebar (File Tree) -->
        <aside class="w-64 bg-[#252526] border-r border-[#333333] flex flex-col">
            <div class="px-4 py-2 text-xs font-semibold tracking-wider text-[#bbbbbb] uppercase select-none">Explorer</div>
            <div id="file-list" class="flex-1 overflow-y-auto text-sm px-2 pb-4">
                <div class="text-[#808080] p-2 text-xs">Loading tree...</div>
            </div>
        </aside>

        <!-- Editor/Viewer -->
        <main class="flex-1 flex flex-col bg-[#1e1e1e]">
            <!-- Tabs -->
            <div id="editor-tabs" class="flex bg-[#2d2d2d] overflow-x-auto select-none">
                <div id="active-tab" class="hidden px-4 py-2 text-sm bg-[#1e1e1e] border-t-2 border-blue-500 text-[#d4d4d4] flex items-center gap-2">
                    <span id="editor-header">Welcome</span>
                </div>
            </div>
            
            <!-- Code Area -->
            <div class="flex-1 relative">
                <pre id="editor-content" class="absolute inset-0 p-4 overflow-auto text-[13px] leading-relaxed font-mono text-[#d4d4d4]">Select a file from the explorer to view its contents.</pre>
            </div>
            
            <!-- Footer Status -->
            <footer class="bg-[#007acc] text-white text-xs px-3 py-1 flex justify-between select-none">
                <div id="footer-left">Agent-Native Temporal Code Graph</div>
                <div id="footer-right">UTF-8</div>
            </footer>
        </main>
    </div>

    <script>
        async function fetchStatus() {
            const res = await fetch('/api/status');
            const data = await res.json();
            document.getElementById('status-bar').innerText = '🌱 ' + data.branch + '  @  ' + (data.head ? data.head.substring(0, 16) : 'None');
        }

        async function fetchFiles() {
            const res = await fetch('/api/files');
            const files = await res.json();
            
            // Build tree
            const tree = { type: 'dir', children: {}, name: 'root' };
            files.forEach(f => {
                const parts = f.path.split('/');
                let current = tree;
                for (let i = 0; i < parts.length - 1; i++) {
                    if (!current.children[parts[i]]) {
                        current.children[parts[i]] = { type: 'dir', children: {}, name: parts[i], expanded: true };
                    }
                    current = current.children[parts[i]];
                }
                current.children[parts[parts.length - 1]] = { type: 'file', hash: f.hash, path: f.path, name: parts[parts.length - 1] };
            });

            const list = document.getElementById('file-list');
            list.innerHTML = '';
            list.appendChild(renderNode(tree, true));
        }
        
        function renderNode(node, isRoot = false) {
            const container = document.createElement('div');
            if (!isRoot) {
                container.className = 'tree-line';
            }
            
            const sortedKeys = Object.keys(node.children).sort((a, b) => {
                const childA = node.children[a];
                const childB = node.children[b];
                if (childA.type !== childB.type) return childA.type === 'dir' ? -1 : 1;
                return a.localeCompare(b);
            });

            sortedKeys.forEach(key => {
                const child = node.children[key];
                const item = document.createElement('div');
                
                const header = document.createElement('div');
                header.className = 'flex items-center gap-1.5 py-1 px-1 cursor-pointer hover:bg-[#2a2d2e] rounded text-[#cccccc] select-none';
                
                if (child.type === 'dir') {
                    const icon = document.createElement('span');
                    icon.innerHTML = '📂';
                    icon.className = 'text-[10px] opacity-80';
                    header.appendChild(icon);
                    
                    const label = document.createElement('span');
                    label.innerText = child.name;
                    header.appendChild(label);
                    
                    const childrenContainer = renderNode(child);
                    header.onclick = () => {
                        child.expanded = !child.expanded;
                        childrenContainer.style.display = child.expanded ? 'block' : 'none';
                        icon.innerHTML = child.expanded ? '📂' : '📁';
                    };
                    
                    item.appendChild(header);
                    item.appendChild(childrenContainer);
                } else {
                    const icon = document.createElement('span');
                    icon.innerHTML = '📄';
                    icon.className = 'text-[10px] opacity-60';
                    header.appendChild(icon);
                    
                    const label = document.createElement('span');
                    label.innerText = child.name;
                    header.appendChild(label);
                    
                    header.onclick = () => loadFile(child.path, child.hash, header);
                    item.appendChild(header);
                }
                
                container.appendChild(item);
            });
            return container;
        }

        let activeHeader = null;

        async function loadFile(path, hash, element) {
            if (activeHeader) activeHeader.classList.remove('bg-[#37373d]', 'text-white');
            element.classList.add('bg-[#37373d]', 'text-white');
            activeHeader = element;

            document.getElementById('active-tab').classList.remove('hidden');
            document.getElementById('editor-header').innerText = path.split('/').pop();
            document.getElementById('editor-content').innerText = 'Loading...';
            
            const res = await fetch('/api/file?hash=' + hash);
            const content = await res.text();
            document.getElementById('editor-content').innerText = content;
        }

        fetchStatus();
        fetchFiles();
    </script>
</body>
</html>`
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}
