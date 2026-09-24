package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"

	"github.com/echo-vcs/echo/internal/index"
	"github.com/spf13/cobra"
)

func init() {
	RootCmd.AddCommand(mcpCmd)
}

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Start the Model Context Protocol (MCP) server over stdio",
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

		// Simple JSON-RPC read loop over stdio
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			line := scanner.Bytes()
			
			var req map[string]interface{}
			if err := json.Unmarshal(line, &req); err != nil {
				sendError(nil, -32700, "Parse error")
				continue
			}

			id := req["id"]
			method, _ := req["method"].(string)
			params, _ := req["params"].(map[string]interface{})

			switch method {
			case "tools/list":
				sendResult(id, map[string]interface{}{
					"tools": []map[string]interface{}{
						{
							"name": "find_symbol",
							"description": "Locate a function or class across the temporal graph",
							"inputSchema": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"name": map[string]interface{}{"type": "string"},
								},
								"required": []string{"name"},
							},
						},
						{
							"name": "history",
							"description": "Get the temporal history (previous checkpoints) for a specific symbol",
							"inputSchema": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"name": map[string]interface{}{"type": "string"},
									"limit": map[string]interface{}{"type": "number"},
								},
								"required": []string{"name"},
							},
						},
					},
				})
			case "tools/call":
				toolName, _ := params["name"].(string)
				toolArgs, _ := params["arguments"].(map[string]interface{})

				if toolName == "find_symbol" {
					symName, _ := toolArgs["name"].(string)
					records, err := idx.FindSymbol(symName, 1)
					if err != nil {
						sendError(id, -32603, err.Error())
						continue
					}

					resBytes, _ := json.Marshal(records)
					sendResult(id, map[string]interface{}{
						"content": []map[string]interface{}{
							{
								"type": "text",
								"text": string(resBytes),
							},
						},
					})
				} else if toolName == "history" {
					symName, _ := toolArgs["name"].(string)
					
					limitFloat, ok := toolArgs["limit"].(float64)
					limit := 5
					if ok {
						limit = int(limitFloat)
					}
					
					records, err := idx.FindSymbol(symName, limit)
					if err != nil {
						sendError(id, -32603, err.Error())
						continue
					}

					resBytes, _ := json.Marshal(records)
					sendResult(id, map[string]interface{}{
						"content": []map[string]interface{}{
							{
								"type": "text",
								"text": string(resBytes),
							},
						},
					})
				} else {
					sendError(id, -32601, "Method not found")
				}

			default:
				// ignore unsupported or lifecycle methods (e.g. initialize)
				sendResult(id, map[string]interface{}{"serverInfo": map[string]string{"name": "atcg", "version": "1.0.0"}, "capabilities": map[string]interface{}{}})
			}
		}
	},
}

func sendResult(id interface{}, result interface{}) {
	res := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"result":  result,
	}
	b, _ := json.Marshal(res)
	fmt.Println(string(b))
}

func sendError(id interface{}, code int, message string) {
	res := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"error": map[string]interface{}{
			"code":    code,
			"message": message,
		},
	}
	b, _ := json.Marshal(res)
	fmt.Println(string(b))
}
