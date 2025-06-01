# MCP 框架商店系統範例專案

## 專案說明

這是一個使用 **MCP (Model Control Protocol)** 框架結合 **OpenAI API** 的範例專案，模擬一個簡單的商店查詢系統。專案展示如何透過 MCP 框架建立 Client-Server 架構，並整合 OpenAI API 進行自然語言處理。

## 系統架構

### Server 端實現 (`cmd/server/main.go`)

基於 [mark3labs/mcp-go](https://github.com/mark3labs/mcp-go) 框架開發，使用 `stdio` 進行程序間通訊：

```go
// 建立 MCP Server
s := server.NewMCPServer(
    "Product Price Server",
    "1.0.0", 
    server.WithToolCapabilities(false),
)

// 啟動 stdio 服務
if err := server.ServeStdio(s); err != nil {
    fmt.Printf("Server error: %v\n", err)
    os.Exit(1)
}
```

### Client 端實現 (`cmd/client/main.go`)

自動啟動 Server 進程並建立連線：

```go
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
    
    return &MCPServer{
        cmd:     cmd,
        stdin:   stdin,
        stdout:  stdout,
        scanner: bufio.NewScanner(stdout),
    }, nil
}
```

## 功能實現

### 1. 商品價格查詢

Server 端定義商品資料：

```go
var defaultProducts = []Product{
    {ID: "1", Name: "Laptop", Price: 1000.0},
    {ID: "2", Name: "Smartphone", Price: 500.0},
    {ID: "3", Name: "Tablet", Price: 300.0},
}
```

工具定義和處理函數：

```go
func getPriceHandler(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
    args := req.GetArguments()
    productID, ok := args["product_id"].(string)
    if !ok {
        return nil, fmt.Errorf("product_id is not a string")
    }

    for _, product := range defaultProducts {
        if product.ID == productID {
            result := map[string]interface{}{
                "success":      true,
                "product_id":   product.ID,
                "product_name": product.Name,
                "price":        product.Price,
                "message":      fmt.Sprintf("The price of %s is $%.2f", product.Name, product.Price),
            }
            resultJSON, _ := json.Marshal(result)
            return &mcp.CallToolResult{
                Content: []mcp.Content{mcp.NewTextContent(string(resultJSON))},
            }, nil
        }
    }
    // 回傳錯誤結果...
}
```

### 2. 多商品總價計算

處理陣列格式的商品清單：


```go
func calculateTotalHandler(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
    args := req.GetArguments()
    itemsInterface, ok := args["items"]
    if !ok {
        return nil, fmt.Errorf("missing items")
    }
    items, ok := itemsInterface.([]interface{})
    if !ok {
        return nil, fmt.Errorf("items is not an array")
    }

    total := 0.0
    var itemDetails []map[string]interface{}

    for _, itemInterface := range items {
        item := itemInterface.(map[string]interface{})
        productID := item["product_id"].(string)
        quantity := int(item["quantity"].(float64))
        
        for _, p := range defaultProducts {
            if p.ID == productID {
                itemTotal := p.Price * float64(quantity)
                total += itemTotal
                
                itemDetails = append(itemDetails, map[string]interface{}{
                    "product_id":   productID,
                    "product_name": p.Name,
                    "price":        p.Price,
                    "quantity":     quantity,
                    "item_total":   itemTotal,
                })
                break
            }
        }
    }

    result := map[string]interface{}{
        "success":     true,
        "total_price": total,
        "items":       itemDetails,
        "item_count":  len(itemDetails),
        "message":     fmt.Sprintf("Total price is $%.2f", total),
    }
    // 回傳結果...
}
```

### 3. 折扣計算

實現中文「打X折」邏輯：

```go
func applyDiscountHandler(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
    args := req.GetArguments()
    totalPrice, ok := args["total_price"].(float64)
    if !ok {
        return nil, fmt.Errorf("missing total_price")
    }
    discountPercentage, ok := args["discount_percentage"].(float64)
    if !ok {
        return nil, fmt.Errorf("missing discount_percentage")
    }

    // 中文「打X折」= 支付原價的X%
    discountedPrice := totalPrice * (discountPercentage / 100)
    originalPrice := totalPrice
    savedAmount := originalPrice - discountedPrice

    result := map[string]interface{}{
        "success":             true,
        "original_price":      originalPrice,
        "discount_percentage": discountPercentage,
        "discounted_price":    discountedPrice,
        "saved_amount":        savedAmount,
        "message":             fmt.Sprintf("Original price: $%.2f, After %.0f%% discount: $%.2f (You save: $%.2f)", originalPrice, discountPercentage, discountedPrice, savedAmount),
    }
    // 回傳結果...
}
```

## OpenAI API 整合

### 工具清單轉換

將 MCP 工具轉換為 OpenAI 格式：

```go
func (s *MCPServer) ListTools() ([]openai.Tool, error) {
    // 發送 tools/list 請求給 MCP Server
    listToolsRequest := map[string]interface{}{
        "jsonrpc": "2.0",
        "id":      2,
        "method":  "tools/list",
    }

    // 解析回應並轉換格式
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
```

### 自然語言處理設定

使用 System Prompt 定義中文處理規則：

```go
Messages: []openai.ChatCompletionMessage{
    {
        Role: openai.ChatMessageRoleSystem,
        Content: `你是一個購物助手，專門處理中文購物查詢。

## 商品對應表
- 筆電/筆記型電腦/電腦/laptop → product_id: "1" (價格: $1000)
- 智慧型手機/手機/smartphone → product_id: "2" (價格: $500)  
- 平板/平板電腦/tablet → product_id: "3" (價格: $300)

## 中文數字轉換
- 一/1 → 1, 二/2 → 2, 三/3 → 3, 四/4 → 4, 五/5 → 5
- 六/6 → 6, 七/7 → 7, 八/8 → 8, 九/9 → 9, 十/10 → 10

## 折扣處理
- "打X折" = discount_percentage: X
- 例如：打三折 = 30, 打八折 = 80`,
    },
},
```

### 複合查詢處理

自動傳遞前一個工具的結果：

```go
for _, toolCall := range message.ToolCalls {
    var arguments map[string]interface{}
    if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &arguments); err != nil {
        continue
    }

    // 如果是 apply_discount 且有前一步結果
    if toolCall.Function.Name == "apply_discount" && lastStructuredResult != nil {
        if totalPrice, exists := lastStructuredResult["total_price"]; exists {
            if price, ok := totalPrice.(float64); ok {
                arguments["total_price"] = price
            }
        }
    }

    // 調用 MCP Server
    response, err := server.CallTool(toolCall.Function.Name, arguments)
    
    // 儲存結果供下一個工具使用
    lastStructuredResult = structuredResult
}
```

## 資料格式

所有工具回傳統一的 JSON 格式：

```json
{
  "success": true,
  "total_price": 6500.0,
  "items": [
    {
      "product_id": "1",
      "product_name": "Laptop", 
      "price": 1000.0,
      "quantity": 5,
      "item_total": 5000.0
    }
  ],
  "item_count": 2,
  "message": "Total price is $6500.00"
}
```

## 錯誤處理

實現完整的參數驗證：

```go
// 驗證數量
quantity, ok := item["quantity"].(float64)
if !ok {
    return &mcp.CallToolResult{
        IsError: true,
        Content: []mcp.Content{mcp.NewTextContent("Invalid quantity format")},
    }, nil
}

// 檢查是否為整數
if quantity != float64(int(quantity)) {
    return &mcp.CallToolResult{
        IsError: true,
        Content: []mcp.Content{mcp.NewTextContent("Quantity must be an integer")},
    }, nil
}

// 檢查範圍
if quantity <= 0 || quantity > 1000 {
    return &mcp.CallToolResult{
        IsError: true,
        Content: []mcp.Content{mcp.NewTextContent("Quantity must be between 1 and 1000")},
    }, nil
}
```

---

## 本地測試環境架設

### 前置需求

1. **Go 開發環境**
   ```bash
   # 確認 Go 版本 (建議 1.19 以上)
   go version
   ```

2. **OpenAI API Key**
   - 前往 [OpenAI Platform](https://platform.openai.com/) 申請 API Key
   - 確保帳戶有足夠的額度

### 安裝步驟

1. **複製專案**
   ```bash
   git clone <your-repo-url>
   cd mcp-store-example
   ```

2. **安裝相依套件**
   ```bash
   make deps
   ```

3. **設定環境變數**
   ```bash
   export OPENAI_API_KEY="your_openai_api_key_here"
   ```
   
   或者建立 `.env` 檔案：
   ```bash
   echo "OPENAI_API_KEY=your_openai_api_key_here" > .env
   source .env
   ```

4. **編譯程式**
   ```bash
   make build
   ```

5. **執行系統**
   ```bash
   make run
   ```

### 測試範例

啟動後，您可以嘗試以下查詢：

```
📱 基本查詢：
"筆電多少錢？"
"手機的價格是多少？"

🛒 多商品計算：
"我要買 2 台筆電和 1 支手機"
"五台平板加上三台智慧型手機多少錢？"

💰 折扣計算：
"總價 2500 元打 9 折"
"1000 元打 3 折後是多少？"

🔄 複合查詢：
"三台筆電加上五支手機再打八折"
"買十台平板打五折後的價格"
```

### 常見問題排除

**問題：出現 "Please set the OPENAI_API_KEY environment variable" 錯誤**
解決：請確認已正確設定 OpenAI API Key 環境變數

**問題：Server 啟動失敗**
解決：檢查 `./bin/product-server` 檔案是否存在，執行 `make build` 重新編譯

**問題：API 回應速度較慢**
解決：這是正常現象，OpenAI API 需要一定的處理時間，系統會顯示實際回應時間

### 開發工具指令

```bash
# 格式化程式碼
make fmt

# 執行測試
make test

# 清理編譯檔案
make clean

# 只編譯 Server
make build-server

# 只編譯 Client  
make build-client
``` 