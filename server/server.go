package main

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type Product struct {
	ID    string  `json:"id"`
	Name  string  `json:"name"`
	Price float64 `json:"price"`
}

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
			return &mcp.CallToolResult{
				Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("The price of %s is $%.2f", product.Name, product.Price))},
			}, nil
		}
	}
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{mcp.NewTextContent("Product not found")},
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
	total := 0.0
	for _, itemInterface := range items {
		item, ok := itemInterface.(map[string]interface{})
		if !ok {
			continue
		}
		productID, ok := item["product_id"].(string)
		if !ok {
			continue
		}
		quantityFloat, ok := item["quantity"].(float64)
		if !ok {
			continue
		}
		quantity := int(quantityFloat)
		for _, p := range defaultProducts {
			if p.ID == productID {
				total += p.Price * float64(quantity)
				break
			}
		}
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Total price is $%.2f", total))},
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
	discount := totalPrice * (discountPercentage / 100)
	discountedPrice := totalPrice - discount
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Discounted price is $%.2f", discountedPrice))},
	}, nil
}

func main() {
	// Create a new MCP server instance
	s := server.NewMCPServer(
		"Product Price Server",
		"1.0.0",
		server.WithToolCapabilities(false),
	)

	// Define the get_price tool based on the schema comment above getPriceHandler
	// Schema: {"type": "object", "properties": {"product_id": {"type": "string"}}, "required": ["product_id"]}
	getPriceTool := mcp.NewTool("get_price",
		mcp.WithDescription("Get the price of a product"),
		mcp.WithString("product_id",
			mcp.Required(),
			mcp.Description("The ID of the product to get the price of"),
		),
	)

	// Add the get_price tool with its handler
	s.AddTool(getPriceTool, getPriceHandler)

	// Define the calculate_total tool based on the schema comment above calculateTotalHandler
	// Schema: complex object with items array containing product_id and quantity
	calculateTotalTool := mcp.NewTool("calculate_total",
		mcp.WithDescription("Calculate the total price for multiple items"),
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

	// Define the apply_discount tool based on the schema comment above applyDiscountHandler
	// Schema: {"type": "object", "properties": {"total_price": {"type": "number"}, "discount_percentage": {"type": "number"}}, "required": ["total_price", "discount_percentage"]}
	applyDiscountTool := mcp.NewTool("apply_discount",
		mcp.WithDescription("Apply a discount to the total price"),
		mcp.WithNumber("total_price", mcp.Required(), mcp.Description("The total price to apply the discount to")),
		mcp.WithNumber("discount_percentage", mcp.Required(), mcp.Description("The discount percentage to apply")),
	)

	// Add the apply_discount tool with its handler
	s.AddTool(applyDiscountTool, applyDiscountHandler)
	// Start the server using stdio transport
	if err := server.ServeStdio(s); err != nil {
		fmt.Printf("Server error: %v\n", err)
	}
}
