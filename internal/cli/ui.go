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
    <title>ATCG Dashboard</title>
    <script src="https://cdn.tailwindcss.com"></script>
</head>
<body class="bg-gray-900 text-gray-100 font-sans h-screen flex flex-col">
    <!-- Navbar -->
    <header class="bg-gray-800 border-b border-gray-700 p-4 flex justify-between items-center">
        <h1 class="text-xl font-bold text-blue-400">ATCG Temporal Graph</h1>
        <div id="status-bar" class="text-sm bg-gray-700 px-3 py-1 rounded">Loading status...</div>
    </header>

    <!-- Main Content -->
    <div class="flex flex-1 overflow-hidden">
        <!-- Sidebar (File Tree) -->
        <aside class="w-1/4 bg-gray-800 border-r border-gray-700 p-4 overflow-y-auto">
            <h2 class="text-sm font-semibold uppercase text-gray-400 mb-4">Files</h2>
            <ul id="file-list" class="space-y-1 text-sm">
                <li class="text-gray-500">Loading files...</li>
            </ul>
        </aside>

        <!-- Editor/Viewer -->
        <main class="w-3/4 flex flex-col bg-gray-950">
            <div id="editor-header" class="bg-gray-800 p-2 border-b border-gray-700 text-sm text-gray-400">
                Select a file to view
            </div>
            <pre id="editor-content" class="p-4 overflow-auto text-sm font-mono text-gray-300 h-full"></pre>
        </main>
    </div>

    <script>
        async function fetchStatus() {
            const res = await fetch('/api/status');
            const data = await res.json();
            document.getElementById('status-bar').innerText = 'Branch: ' + data.branch + ' | HEAD: ' + (data.head ? data.head.substring(0, 16) : 'None');
        }

        async function fetchFiles() {
            const res = await fetch('/api/files');
            const files = await res.json();
            const list = document.getElementById('file-list');
            list.innerHTML = '';
            
            files.sort((a, b) => a.path.localeCompare(b.path)).forEach(f => {
                const li = document.createElement('li');
                li.className = 'cursor-pointer hover:text-blue-400 truncate py-1';
                li.innerText = f.path;
                li.onclick = () => loadFile(f.path, f.hash);
                list.appendChild(li);
            });
        }

        async function loadFile(path, hash) {
            document.getElementById('editor-header').innerText = path;
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
