// echo MCP server: reads newline-delimited JSON-RPC from stdin, replies with
// either an empty result (request) or echoes notifications. Writes a banner
// to stderr so stderr-capture tests can assert on it. Terminates on EOF.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
)

type rpc struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id,omitempty"`
	Method  string           `json:"method"`
	Params  json.RawMessage  `json:"params,omitempty"`
	Result  json.RawMessage  `json:"result,omitempty"`
	Error   json.RawMessage  `json:"error,omitempty"`
}

func main() {
	fmt.Fprintln(os.Stderr, "echo-mcp-server: ready")
	in := bufio.NewScanner(os.Stdin)
	in.Buffer(make([]byte, 1024*1024), 16*1024*1024)
	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()
	for in.Scan() {
		var req rpc
		if err := json.Unmarshal(in.Bytes(), &req); err != nil {
			fmt.Fprintf(os.Stderr, "parse error: %v\n", err)
			continue
		}
		if req.ID == nil {
			fmt.Fprintf(os.Stderr, "notif: %s\n", req.Method)
			continue
		}
		var resultPayload json.RawMessage
		switch req.Method {
		case "tools/list":
			resultPayload = json.RawMessage(`{"tools":[{"name":"echo","description":"Echoes input back","inputSchema":{"type":"object","properties":{"text":{"type":"string"}},"required":["text"]}}]}`)
		default:
			resultPayload = json.RawMessage(`{}`)
		}
		resp := rpc{JSONRPC: "2.0", ID: req.ID, Result: resultPayload}
		b, _ := json.Marshal(&resp)
		out.Write(b)
		out.WriteByte('\n')
		out.Flush()
	}
}
