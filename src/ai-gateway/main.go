package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
	"os"

	"github.com/mark3labs/mcp-go/mcp"
)

// --- 异步 MCP Client ---
type SimpleClient struct {
	sseURL      string
	postURL     string
	httpClient  *http.Client
	mu          sync.Mutex
	ready       bool
	pendingReqs sync.Map 
}

type RPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func NewSimpleClient(url string) *SimpleClient {
	return &SimpleClient{
		sseURL:     url,
		httpClient: &http.Client{Timeout: 0}, 
	}
}

func (c *SimpleClient) Start(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", c.sseURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "text/event-stream")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	
	go func() {
		defer resp.Body.Close()
		scanner := bufio.NewScanner(resp.Body)
		var currentEvent string

		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				currentEvent = ""
				continue
			}

			if strings.HasPrefix(line, "event: ") {
				currentEvent = strings.TrimPrefix(line, "event: ")
			} else if strings.HasPrefix(line, "data: ") {
				data := strings.TrimPrefix(line, "data: ")
				if currentEvent == "endpoint" {
					c.mu.Lock()
					baseURL := c.sseURL
					if strings.HasSuffix(baseURL, "/sse") {
						baseURL = strings.TrimSuffix(baseURL, "/sse")
					}
					if strings.HasPrefix(data, "/") {
						c.postURL = baseURL + data
					} else {
						c.postURL = data
					}
					log.Printf("MCP Session Established. POST URL: %s", c.postURL)
					c.ready = true
					c.mu.Unlock()
				} else if currentEvent == "message" {
				        // --- 新增日志：记录收到的 SSE 原始数据 ---
                                        log.Printf("⬅️ [MCP-IN] From SSE: %s", data)
                                        // -------------------------------------
					var rpcResp RPCResponse
					if err := json.Unmarshal([]byte(data), &rpcResp); err == nil {
						idStr := fmt.Sprintf("%v", rpcResp.ID)
						if ch, ok := c.pendingReqs.Load(idStr); ok {
							ch.(chan *RPCResponse) <- &rpcResp
							c.pendingReqs.Delete(idStr)
						}
					}
				}
			}
		}
	}()

	for i := 0; i < 50; i++ {
		c.mu.Lock()
		if c.ready {
			c.mu.Unlock()
			return nil
		}
		c.mu.Unlock()
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for endpoint event")
}

func (c *SimpleClient) SendRequest(method string, params interface{}, result interface{}) error {
	c.mu.Lock()
	if !c.ready {
		c.mu.Unlock()
		return fmt.Errorf("client not connected")
	}
	url := c.postURL
	c.mu.Unlock()

	reqID := fmt.Sprintf("%d", time.Now().UnixNano())
	respChan := make(chan *RPCResponse, 1)
	c.pendingReqs.Store(reqID, respChan)
	defer c.pendingReqs.Delete(reqID)

	reqBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      reqID,
		"method":  method,
		"params":  params,
	}
	jsonBytes, _ := json.Marshal(reqBody)

        // --- 新增日志：记录发出的 JSON-RPC 请求 ---
        log.Printf("➡️ [MCP-OUT] To Server: %s", string(jsonBytes))
        // ----------------------------------------	

	resp, err := c.httpClient.Post(url, "application/json", bytes.NewReader(jsonBytes))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 && resp.StatusCode != 202 {
		return fmt.Errorf("server returned status: %s", resp.Status)
	}

	select {
	case rpcResp := <-respChan:
		if rpcResp.Error != nil {
			return fmt.Errorf("RPC Error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
		}
		if result != nil {
			return json.Unmarshal(rpcResp.Result, result)
		}
		return nil
	case <-time.After(10 * time.Second):
		return fmt.Errorf("request timeout")
	}
}

// --- 业务代码 ---
type ListToolsResult struct {
	Tools []mcp.Tool `json:"tools"`
}

type CallToolResult struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text,omitempty"`
	} `json:"content"`
}

// --- HTML 模板 (包含 CSS 和 JS Markdown 渲染器) ---
const htmlHead = `<html>
<head><title>MCP HR Agent</title>
<!-- 引入 marked.js 用于渲染 Markdown -->
<script src="https://cdn.jsdelivr.net/npm/marked/marked.min.js"></script>
<style>
	body { font-family: 'Segoe UI', sans-serif; max-width: 100%; margin: 20px; padding: 0 20px; background-color: #f4f6f9; color: #333; }
	h2 { color: #2c3e50; border-bottom: 2px solid #3498db; padding-bottom: 10px; width: fit-content; }
	h3 { color: #34495e; margin-top: 25px; border-left: 4px solid #3498db; padding-left: 10px; }
	form { background: white; padding: 20px; border-radius: 8px; box-shadow: 0 2px 5px rgba(0,0,0,0.05); }
	textarea { width: 100%; padding: 10px; border: 1px solid #ddd; border-radius: 4px; font-size: 16px; }
	input[type=submit] { background-color: #3498db; color: white; border: none; padding: 10px 25px; border-radius: 4px; cursor: pointer; margin-top: 10px; }
	
	/* 消息气泡 */
	.message { padding: 15px; margin: 10px 0; border-radius: 6px; background: white; border-left: 5px solid #00b894; box-shadow: 0 1px 3px rgba(0,0,0,0.1); }
	.tool { background-color: #fff8e1; font-family: monospace; padding: 10px; margin: 5px 0; border-radius: 4px; color: #5d4037; border: 1px solid #ffe0b2; }
	
	/* 表格样式 (Markdown 渲染后的表格会用到这些样式) */
	table { border-collapse: collapse; margin: 15px 0; width: 100%; max-width: 800px; box-shadow: 0 0 10px rgba(0,0,0,0.05); }
	th { background-color: #009879; color: white; text-align: left; padding: 10px; }
	td { padding: 10px; border-bottom: 1px solid #ddd; }
	tr:nth-child(even) { background-color: #f3f3f3; }
	
	/* SQL 代码块 */
	pre { background: #f4f4f4; padding: 10px; border-radius: 4px; overflow-x: auto; }
</style>
</head>
<body>`

func main() {
	serverURL := "http://mcp-server-service.ai-services:8080/sse"
	log.Printf("Connecting to MCP Server at %s...", serverURL)

	client := NewSimpleClient(serverURL)
	ctx := context.Background()
	if err := client.Start(ctx); err != nil {
		log.Printf("Warning: Failed to connect to MCP Server: %v", err)
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		handleHome(w, r)
	})
	
	http.HandleFunc("/chat", func(w http.ResponseWriter, r *http.Request) {
		handleChat(w, r, client)
	})
	
	log.Println("AI Gateway running on :3000")
	if err := http.ListenAndServe(":3000", nil); err != nil {
		log.Fatal(err)
	}
}

func handleHome(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, htmlHead)
	body := `
		<h2>HR 智能助手 (基于 MCP + Qwen 3)</h2>
		<form action="/chat" method="POST">
			<textarea name="prompt" rows="5" placeholder="例如：查询销售部门的所有员工信息..."></textarea><br>
			<input type="submit" value="发送查询">
		</form>
	</body></html>`
	fmt.Fprint(w, body)
}

func handleChat(w http.ResponseWriter, r *http.Request, client *SimpleClient) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, htmlHead)

	userPrompt := r.FormValue("prompt")
	fmt.Fprintf(w, "<h3>思考过程：</h3>")

	var toolsRes ListToolsResult
	err := client.SendRequest("tools/list", map[string]interface{}{}, &toolsRes)
	if err != nil {
		fmt.Fprintf(w, "<div style='color:red'>无法获取工具列表: %v</div>", err)
		return
	}
	ollamaTools := convertToOllamaTools(toolsRes.Tools)

	messages := []Message{
		{
			Role: "system",
			// 保持你之前已经改好的简化版 Prompt
			Content: `你是一个专业的数据库管理员。你的目标是根据用户问题，自动查询数据库并给出最终答案。
            执行原则：
            1. 必须使用工具：先调用 read_schema 了解结构，再调用 execute_query 执行 SQL。
            2. 自主规划：根据表结构自动编写 JOIN 等查询语句，不要问用户怎么查。
            3. 结果展示：使用 Markdown 格式清晰地展示数据（例如使用 Markdown 表格）。`,
		},
		{Role: "user", Content: userPrompt},
	}
	
	fmt.Fprintf(w, "<div>正在询问 AI...</div>")

	var finalContent string // <--- 新增：用于保存最终回答

	maxTurns := 5
	for i := 0; i < maxTurns; i++ {
		resp, err := callOllama(messages, ollamaTools)
		if err != nil {
			fmt.Fprintf(w, "<div style='color:red'>Ollama Error: %v</div>", err)
			return
		}

		messages = append(messages, resp.Message)

		if len(resp.Message.ToolCalls) == 0 {
			// 找到了最终回答，保存并跳出循环
			finalContent = resp.Message.Content
			break
		}

		for _, tc := range resp.Message.ToolCalls {
			if tc.Function.Name == "execute_query" {
				if sql, ok := tc.Function.Arguments["query"].(string); ok {
					fmt.Fprintf(w, "<div style='background:#e8f4fd; padding:10px; border-left: 4px solid #007bff; margin: 5px 0; font-family: monospace;'><strong>🤖 生成 SQL:</strong><br>%s</div>", sql)
				}
			}

			fmt.Fprintf(w, "<div class='tool'>调用工具: <strong>%s</strong></div>", tc.Function.Name)
			
			var toolRes CallToolResult
			err := client.SendRequest("tools/call", map[string]interface{}{
				"name": tc.Function.Name,
				"arguments": tc.Function.Arguments,
			}, &toolRes)

			output := "Error"
			if err != nil {
				output = err.Error()
			} else if len(toolRes.Content) > 0 {
				output = toolRes.Content[0].Text
			}
			
			displayOutput := output
			if len(displayOutput) > 200 { displayOutput = displayOutput[:200] + "..." }
			fmt.Fprintf(w, "<div class='tool'>结果: %s</div>", displayOutput)
			
			messages = append(messages, Message{Role: "tool", Content: output})
		}
	}
	
	// --- 输出最终结果和渲染脚本 ---
	fmt.Fprintf(w, `<h3>AI 回答：</h3>
	<script type="text/template" id="ai-raw">%s</script>
	<div id="ai-render" class="ai message">正在渲染...</div>
	<script>
		var rawMd = document.getElementById('ai-raw').innerHTML;
		var targetDiv = document.getElementById('ai-render');
		
		var lines = rawMd.split('\n');
		var html = '';
		var inTable = false;
		
		for (var i = 0; i < lines.length; i++) {
			var line = lines[i].trim();
			if (line.startsWith('|') && line.endsWith('|')) {
				if (!inTable) { html += '<table>'; inTable = true; }
				if (line.includes('---')) { continue; }
				var cells = line.split('|').filter(function(c) { return c.trim() !== ''; });
				html += '<tr>';
				for (var j = 0; j < cells.length; j++) {
					var tag = (html.endsWith('<table>')) ? 'th' : 'td';
					html += '<' + tag + '>' + cells[j].trim() + '</' + tag + '>';
				}
				html += '</tr>';
			} else {
				if (inTable) { html += '</table>'; inTable = false; }
				if (line.length > 0) html += '<p>' + line + '</p>';
			}
		}
		if (inTable) html += '</table>';
		targetDiv.innerHTML = html;
	</script>
	<br><a href='/'>返回首页</a></body></html>`, finalContent) // <--- 这里传入 finalContent
}

// 辅助结构体和函数保持不变
type OllamaRequest struct {
	Model string `json:"model"` 
	Messages []Message `json:"messages"`
	Stream bool `json:"stream"`
	Tools []OllamaTool `json:"tools,omitempty"`
}
type Message struct { Role string `json:"role"`; Content string `json:"content"`; ToolCalls []ToolCall `json:"tool_calls,omitempty"` }
type ToolCall struct { Function FunctionCall `json:"function"` }
type FunctionCall struct { Name string `json:"name"`; Arguments map[string]interface{} `json:"arguments"` }
type OllamaTool struct { Type string `json:"type"`; Function ToolFunction `json:"function"` }
type ToolFunction struct { Name string `json:"name"`; Description string `json:"description"`; Parameters interface{} `json:"parameters"` }
type OllamaResponse struct { Message Message `json:"message"` }

func callOllama(msgs []Message, tools []OllamaTool) (*OllamaResponse, error) {
    ollamaAPI := os.Getenv("OLLAMA_URL")
	if ollamaAPI == "" {
		ollamaAPI = "http://172.20.10.5:11434/api/chat"
	}

	ollamaModel := os.Getenv("OLLAMA_MODEL")
	if ollamaModel == "" {
		ollamaModel = "qwen3:4b"
	}
	
	req := OllamaRequest{
		Model:    ollamaModel, // 默认qwen3:4b，有很好的工具调用功能以及中文处理和SQL书写能力
		Messages: msgs,
		Stream:   false,
		Tools:    tools,
	}
	jsonData, _ := json.Marshal(req)
	// 修改为统一格式
	log.Printf("🤖 [LLM-REQ] To Ollama: %s", string(jsonData))
	resp, err := http.Post(ollamaAPI, "application/json", strings.NewReader(string(jsonData))) //主要修改
	if err != nil { return nil, err }
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(resp.Body)
	// 修改为统一格式
	log.Printf("🤖 [LLM-RES] From Ollama (Status %s): %s", resp.Status, string(bodyBytes))
	resp.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	var result OllamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil { return nil, err }
	return &result, nil
}

func convertToOllamaTools(mcpTools []mcp.Tool) []OllamaTool {
	var out []OllamaTool
	for _, t := range mcpTools {
		out = append(out, OllamaTool{Type: "function", Function: ToolFunction{Name: t.Name, Description: t.Description, Parameters: t.InputSchema}})
	}
	return out
}
