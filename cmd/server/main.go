package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Product represents a product in the store
type Product struct {
	ID    string  `json:"id"`
	Name  string  `json:"name"`
	Price float64 `json:"price"`
}

// Default products available in the store
var defaultProducts = []Product{
	{ID: "1", Name: "Laptop", Price: 1000.0},
	{ID: "2", Name: "Smartphone", Price: 500.0},
	{ID: "3", Name: "Tablet", Price: 300.0},
}

/*
	{
	  "type": "object",
	  "properties": {
	    "product_id": {"type": "string"}
	  },
	  "required": ["product_id"]
	}
*/
func getPriceHandler(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	if args == nil {
		return nil, fmt.Errorf("no arguments provided")
	}

	productID, ok := args["product_id"].(string)
	if !ok {
		return nil, fmt.Errorf("product_id is not a string")
	}

	for _, product := range defaultProducts {
		if product.ID == productID {
			// Return structured data
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

	// Return structured error
	errorResult := map[string]interface{}{
		"success":    false,
		"error":      "Product not found",
		"product_id": productID,
	}
	errorJSON, _ := json.Marshal(errorResult)
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{mcp.NewTextContent(string(errorJSON))},
	}, nil
}

/*
	{
	  "type": "object",
	  "properties": {
	    "items": {
	      "type": "array",
	      "items": {
	        "type": "object",
	        "properties": {
	          "product_id": {"type": "string"},
	          "quantity": {"type": "integer"}
	        },
	        "required": ["product_id", "quantity"]
	      }
	    }
	  },
	  "required": ["items"]
	}
*/
func calculateTotalHandler(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	if args == nil {
		return nil, fmt.Errorf("invalid arguments")
	}
	itemsInterface, ok := args["items"]
	if !ok {
		return nil, fmt.Errorf("missing items")
	}
	items, ok := itemsInterface.([]interface{})
	if !ok {
		return nil, fmt.Errorf("items is not an array")
	}

	// Validate product quantity
	for _, itemInterface := range items {
		item, ok := itemInterface.(map[string]interface{})
		if !ok {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{mcp.NewTextContent("Invalid item format")},
			}, nil
		}

		// Validate product ID
		productID, ok := item["product_id"].(string)
		if !ok {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{mcp.NewTextContent("Invalid product ID format")},
			}, nil
		}

		// Validate product existence
		productExists := false
		for _, p := range defaultProducts {
			if p.ID == productID {
				productExists = true
				break
			}
		}
		if !productExists {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Product with ID %s not found", productID))},
			}, nil
		}

		// Validate quantity
		quantity, ok := item["quantity"].(float64)
		if !ok {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{mcp.NewTextContent("Invalid quantity format")},
			}, nil
		}

		// Check if quantity is an integer
		if quantity != float64(int(quantity)) {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{mcp.NewTextContent("Quantity must be an integer")},
			}, nil
		}

		// Check if quantity is positive
		if quantity <= 0 {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{mcp.NewTextContent("Quantity must be greater than 0")},
			}, nil
		}

		// Check if quantity is within reasonable range
		if quantity > 1000 {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{mcp.NewTextContent("Quantity cannot exceed 1000")},
			}, nil
		}
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

				// Add item details
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

	// Return structured data
	result := map[string]interface{}{
		"success":     true,
		"total_price": total,
		"items":       itemDetails,
		"item_count":  len(itemDetails),
		"message":     fmt.Sprintf("Total price is $%.2f", total),
	}
	resultJSON, _ := json.Marshal(result)
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(string(resultJSON))},
	}, nil
}

/*
	{
	  "type": "object",
	  "properties": {
	    "total_price": {"type": "number"},
	    "discount_percentage": {"type": "number"}
	  },
	  "required": ["total_price", "discount_percentage"]
	}
*/
func applyDiscountHandler(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	if args == nil {
		return nil, fmt.Errorf("invalid arguments")
	}
	totalPrice, ok := args["total_price"].(float64)
	if !ok {
		return nil, fmt.Errorf("missing total_price")
	}
	discountPercentage, ok := args["discount_percentage"].(float64)
	if !ok {
		return nil, fmt.Errorf("missing discount_percentage")
	}

	// In Chinese, "打X折" means paying X% of the original price
	// So discount_percentage represents the percentage to keep, not to subtract
	discountedPrice := totalPrice * (discountPercentage / 100)
	originalPrice := totalPrice
	savedAmount := originalPrice - discountedPrice

	// Return structured data
	result := map[string]interface{}{
		"success":             true,
		"original_price":      originalPrice,
		"discount_percentage": discountPercentage,
		"discounted_price":    discountedPrice,
		"saved_amount":        savedAmount,
		"message":             fmt.Sprintf("Original price: $%.2f, After %.0f%% discount: $%.2f (You save: $%.2f)", originalPrice, discountPercentage, discountedPrice, savedAmount),
	}
	resultJSON, _ := json.Marshal(result)
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(string(resultJSON))},
	}, nil
}

func helpHandler(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	helpText := `Available tools:

1. get_price - Get the price of a product by ID
   Parameters: product_id (string)
   Example: {"product_id": "1"}

2. calculate_total - Calculate total price for multiple items
   Parameters: items (array of {product_id, quantity})
   Example: {"items": [{"product_id": "1", "quantity": 2}]}

3. apply_discount - Apply discount to a total price
   Parameters: total_price (number), discount_percentage (number)
   Example: {"total_price": 1000, "discount_percentage": 30}

Product IDs:
- "1": Laptop ($1000)
- "2": Smartphone ($500)
- "3": Tablet ($300)

Note: discount_percentage represents the percentage to keep (e.g., 30 for 30% of original price)`

	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(helpText)},
	}, nil
}

func main() {
	// Create a new MCP server instance
	s := server.NewMCPServer(
		"Product Price Server",
		"1.0.0",
		server.WithToolCapabilities(false),
	)

	// Define the help tool
	helpTool := mcp.NewTool("help",
		mcp.WithDescription("Show all supported operations and examples"),
	)

	// Add the help tool with its handler
	s.AddTool(helpTool, helpHandler)

	// Define the get_price tool
	getPriceTool := mcp.NewTool("get_price",
		mcp.WithDescription(`Get the price of a product by its ID.
Product mapping:
- Laptop -> ID: "1", Price: $1000.0
- Smartphone -> ID: "2", Price: $500.0
- Tablet -> ID: "3", Price: $300.0`),
		mcp.WithString("product_id",
			mcp.Required(),
			mcp.Description("The ID of the product to get the price of"),
		),
	)

	// Add the get_price tool with its handler
	s.AddTool(getPriceTool, getPriceHandler)

	// Define the calculate_total tool
	calculateTotalTool := mcp.NewTool("calculate_total",
		mcp.WithDescription(`Calculate the total price for multiple items.
Product mapping:
- Laptop -> ID: "1", Price: $1000.0
- Smartphone -> ID: "2", Price: $500.0
- Tablet -> ID: "3", Price: $300.0`),
		mcp.WithArray("items",
			mcp.Required(),
			mcp.Description("Array of items with product_id and quantity"),
			mcp.Items(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"product_id": map[string]any{
						"type":        "string",
						"description": "The ID of the product",
					},
					"quantity": map[string]any{
						"type":        "integer",
						"description": "The quantity of the product",
					},
				},
				"required": []string{"product_id", "quantity"},
			}),
		),
	)

	// Add the calculate_total tool with its handler
	s.AddTool(calculateTotalTool, calculateTotalHandler)

	// Define the apply_discount tool
	applyDiscountTool := mcp.NewTool("apply_discount",
		mcp.WithDescription(`Apply a discount to the total price.
In Chinese context, "打X折" means paying X% of the original price.
For example:
- "打3折" (30% discount) means paying 30% of original price, saving 70%
- "打8折" (80% discount) means paying 80% of original price, saving 20%`),
		mcp.WithNumber("total_price", mcp.Required(), mcp.Description("The total price to apply the discount to")),
		mcp.WithNumber("discount_percentage", mcp.Required(), mcp.Description("The percentage to keep (e.g., 30 for 打3折, 80 for 打8折)")),
	)

	// Add the apply_discount tool with its handler
	s.AddTool(applyDiscountTool, applyDiscountHandler)

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		os.Exit(0)
	}()

	// Start the server using stdio
	if err := server.ServeStdio(s); err != nil {
		fmt.Printf("Server error: %v\n", err)
		os.Exit(1)
	}
}
