package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

// MCPServer represents a connection to the MCP server
type MCPServer struct {
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	stdout  io.ReadCloser
	scanner *bufio.Scanner
}

// NewMCPServer creates a new connection to the MCP server
func NewMCPServer() (*MCPServer, error) {
	cmd := exec.Command("./bin/product-server")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get stdin pipe: %v", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get stdout pipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start server: %v", err)
	}

	scanner := bufio.NewScanner(stdout)
	return &MCPServer{
		cmd:     cmd,
		stdin:   stdin,
		stdout:  stdout,
		scanner: scanner,
	}, nil
}

// Close closes the connection to the server
func (s *MCPServer) Close() error {
	if err := s.stdin.Close(); err != nil {
		return fmt.Errorf("failed to close stdin: %v", err)
	}
	return s.cmd.Wait()
}

// Initialize sends the initialization request to the MCP server
func (s *MCPServer) Initialize() error {
	initRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]interface{}{},
			"clientInfo": map[string]interface{}{
				"name":    "interactive-client",
				"version": "1.0.0",
			},
		},
	}

	reqBytes, _ := json.Marshal(initRequest)
	if _, err := fmt.Fprintf(s.stdin, "%s\n", reqBytes); err != nil {
		return fmt.Errorf("failed to send initialization request: %v", err)
	}

	if s.scanner.Scan() {
		responseText := s.scanner.Text()

		// Parse the initialization response
		var response map[string]interface{}
		if err := json.Unmarshal([]byte(responseText), &response); err != nil {
			return fmt.Errorf("failed to parse initialization response: %v", err)
		}

		// Check if initialization was successful
		if result, ok := response["result"].(map[string]interface{}); ok {
			if serverInfo, ok := result["serverInfo"].(map[string]interface{}); ok {
				fmt.Printf("Connected to: %s v%s\n", serverInfo["name"], serverInfo["version"])
			}
		}
		return nil
	}
	return fmt.Errorf("failed to initialize server")
}

// ListTools retrieves the list of available tools from the MCP server
func (s *MCPServer) ListTools() ([]openai.Tool, error) {
	listToolsRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/list",
	}

	reqBytes, _ := json.Marshal(listToolsRequest)
	if _, err := fmt.Fprintf(s.stdin, "%s\n", reqBytes); err != nil {
		return nil, fmt.Errorf("failed to send tools list request: %v", err)
	}

	if !s.scanner.Scan() {
		return nil, fmt.Errorf("failed to get tools list")
	}

	responseText := s.scanner.Text()

	var response map[string]interface{}
	if err := json.Unmarshal([]byte(responseText), &response); err != nil {
		return nil, fmt.Errorf("failed to parse tools list response: %v", err)
	}

	result, ok := response["result"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response format - no result field")
	}

	tools, ok := result["tools"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid tools format - tools field not found or not an array")
	}

	var openaiTools []openai.Tool
	for _, tool := range tools {
		toolMap, ok := tool.(map[string]interface{})
		if !ok {
			continue
		}

		name, _ := toolMap["name"].(string)
		description, _ := toolMap["description"].(string)
		inputSchema, _ := toolMap["inputSchema"].(map[string]interface{})

		openaiTool := openai.Tool{
			Type: "function",
			Function: &openai.FunctionDefinition{
				Name:        name,
				Description: description,
				Parameters:  inputSchema,
			},
		}
		openaiTools = append(openaiTools, openaiTool)
	}

	return openaiTools, nil
}

// CallTool sends a tool call request to the MCP server
func (s *MCPServer) CallTool(name string, arguments map[string]interface{}) (string, error) {
	toolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      3,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      name,
			"arguments": arguments,
		},
	}

	reqBytes, _ := json.Marshal(toolRequest)
	if _, err := fmt.Fprintf(s.stdin, "%s\n", reqBytes); err != nil {
		return "", fmt.Errorf("failed to send tool call request: %v", err)
	}

	if s.scanner.Scan() {
		return s.scanner.Text(), nil
	}
	return "", fmt.Errorf("failed to get response")
}

// extractContentFromResponse extracts the text content from a JSON response
func extractContentFromResponse(response string) string {
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(response), &result); err != nil {
		return response
	}

	if resultObj, ok := result["result"].(map[string]interface{}); ok {
		if contentArray, ok := resultObj["content"].([]interface{}); ok {
			for _, content := range contentArray {
				if contentMap, ok := content.(map[string]interface{}); ok {
					if text, ok := contentMap["text"].(string); ok {
						return text
					}
				}
			}
		}
	}
	return response
}

// parseStructuredResponse parses structured JSON response from MCP server
func parseStructuredResponse(response string) (map[string]interface{}, error) {
	content := extractContentFromResponse(response)

	var structuredData map[string]interface{}
	if err := json.Unmarshal([]byte(content), &structuredData); err != nil {
		// If it's not JSON, return the content as message
		return map[string]interface{}{
			"success": true,
			"message": content,
		}, nil
	}

	return structuredData, nil
}

func main() {
	// Connect to MCP server
	server, err := NewMCPServer()
	if err != nil {
		fmt.Printf("Failed to connect to server: %v\n", err)
		return
	}
	defer server.Close()

	// Initialize server connection
	if err := server.Initialize(); err != nil {
		fmt.Printf("Failed to initialize server: %v\n", err)
		return
	}

	// Get tools list from server
	tools, err := server.ListTools()
	if err != nil {
		fmt.Printf("Failed to get tools list: %v\n", err)
		return
	}
	fmt.Println("\nAvailable tools:")
	for _, tool := range tools {
		fmt.Printf("- %s: %s\n", tool.Function.Name, tool.Function.Description)
	}

	// Check OpenAI API key
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		fmt.Println("Please set the OPENAI_API_KEY environment variable to continue with interactive mode")
		return
	}

	// Initialize OpenAI client
	client := openai.NewClient(apiKey)

	// Interactive conversation
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("\nWelcome to the Interactive Product Query System!")
	fmt.Println("You can ask about product prices, calculate totals, or apply discounts.")
	fmt.Println("Type 'exit' to quit.")
	fmt.Println("Type 'help' for supported operations.")

	for {
		fmt.Print("\nPlease enter your question: ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		if input == "exit" {
			break
		}

		// Use OpenAI to parse user input
		start := time.Now()
		resp, err := client.CreateChatCompletion(
			context.Background(),
			openai.ChatCompletionRequest{
				Model: openai.GPT4TurboPreview,
				Messages: []openai.ChatCompletionMessage{
					{
						Role: openai.ChatMessageRoleSystem,
						Content: `你是一個智能購物助手，專門處理複雜的中文購物查詢。你能夠理解中文表達並將其轉換為正確的工具調用。

## 商品對應表
- 筆電/筆記型電腦/電腦/laptop → product_id: "1" (價格: $1000)
- 智慧型手機/手機/smartphone → product_id: "2" (價格: $500)  
- 平板/平板電腦/tablet → product_id: "3" (價格: $300)

## 中文數字轉換
- 一/1 → 1, 二/2 → 2, 三/3 → 3, 四/4 → 4, 五/5 → 5
- 六/6 → 6, 七/7 → 7, 八/8 → 8, 九/9 → 9, 十/10 → 10
- 二十/20 → 20, 三十/30 → 30, 四十/40 → 40, 五十/50 → 50
- 其他數字：直接使用阿拉伯數字

## 折扣處理
- "打X折" = discount_percentage: X
- 例如：打三折 = 30, 打八折 = 80, 打五折 = 50

## 工具使用規則

### 1. 簡單價格查詢
用戶問："筆電多少錢？" → 使用 get_price
參數：{"product_id": "1"}

### 2. 多商品總價計算  
用戶問："五台筆電加上三台智慧型手機多少錢？" → 使用 calculate_total
參數：{"items": [{"product_id": "1", "quantity": 5}, {"product_id": "2", "quantity": 3}]}

### 3. 折扣應用
用戶問："$2000打八折是多少？" → 使用 apply_discount
參數：{"total_price": 2000, "discount_percentage": 80}

### 4. 複合查詢（重要！）
用戶問："五台筆電加上三十台智慧型手機再打三折"
需要按順序調用：
1. calculate_total: {"items": [{"product_id": "1", "quantity": 5}, {"product_id": "2", "quantity": 30}]}
2. apply_discount: {"total_price": [從第一步結果中提取], "discount_percentage": 30}

## 參數提取注意事項
- product_id 必須是字符串 "1", "2", "3"
- quantity 必須是正整數
- discount_percentage 必須是 1-99 之間的數字
- total_price 必須是正數

## 錯誤處理
如果無法完全理解用戶查詢，嘗試部分解析並說明需要更多資訊。

請根據用戶的中文查詢選擇合適的工具並正確提取參數。`,
					},
					{
						Role:    openai.ChatMessageRoleUser,
						Content: input,
					},
				},
				Tools: tools,
			},
		)
		elapsed := time.Since(start)
		fmt.Printf("OpenAI API response time: %v\n", elapsed)

		if err != nil {
			fmt.Printf("OpenAI API error: %v\n", err)
			continue
		}

		// Process OpenAI response
		message := resp.Choices[0].Message
		if message.ToolCalls != nil {
			var lastResult string
			var lastStructuredResult map[string]interface{}

			for _, toolCall := range message.ToolCalls {
				var arguments map[string]interface{}
				if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &arguments); err != nil {
					fmt.Printf("Error parsing arguments: %v\n", err)
					continue
				}

				// If it's apply_discount and we have a previous structured result with total_price
				if toolCall.Function.Name == "apply_discount" && lastStructuredResult != nil {
					if totalPrice, exists := lastStructuredResult["total_price"]; exists {
						if price, ok := totalPrice.(float64); ok {
							arguments["total_price"] = price
							fmt.Printf("自動使用前一步的總價: $%.2f\n", price)
						}
					}
				}

				// Call MCP server
				response, err := server.CallTool(toolCall.Function.Name, arguments)
				if err != nil {
					fmt.Printf("Error calling tool: %v\n", err)
					continue
				}

				// Parse structured response
				structuredResult, err := parseStructuredResponse(response)
				if err != nil {
					fmt.Printf("Error parsing structured response: %v\n", err)
					continue
				}

				// Store for potential use in next tool call
				lastStructuredResult = structuredResult

				// Display structured result
				if message, exists := structuredResult["message"]; exists {
					lastResult = message.(string)
					fmt.Printf("\n%s\n", lastResult)
				}

				// Also display structured data for debugging/testing
				if success, exists := structuredResult["success"]; exists && success.(bool) {
					fmt.Printf("結構化數據: %+v\n", structuredResult)
				}
			}

			// Use LLM to polish the final response
			polishedResp, err := client.CreateChatCompletion(
				context.Background(),
				openai.ChatCompletionRequest{
					Model: openai.GPT4TurboPreview,
					Messages: []openai.ChatCompletionMessage{
						{
							Role: openai.ChatMessageRoleSystem,
							Content: `You are a friendly store assistant. Please convert the system's response into a more friendly and natural conversation format.
If the response is an error message, please tell the user about the problem in a more friendly way and provide suggestions.
Please maintain a professional but friendly tone and respond in Traditional Chinese.
If the response includes discount calculations, please clearly explain the original price and the discounted price.`,
						},
						{
							Role:    openai.ChatMessageRoleUser,
							Content: fmt.Sprintf("User question: %s\nSystem response: %s", input, lastResult),
						},
					},
				},
			)

			if err != nil {
				fmt.Printf("Error polishing response: %v\n", err)
			} else {
				fmt.Printf("\n%s\n", polishedResp.Choices[0].Message.Content)
			}
		} else {
			fmt.Printf("\n%s\n", message.Content)
		}
	}
}
