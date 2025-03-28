package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/parakeet-nest/parakeet/embeddings"
	"github.com/parakeet-nest/parakeet/llm"
)

var globalStore embeddings.BboltVectorStore

// StartMCPServer initializes and starts the MCP server
func StartMCPServer() error {
	err := globalStore.Initialize(dbPath)
	if err != nil {
		return fmt.Errorf("error initializing vector store: %v", err)
	}

	// Create MCP server
	s := server.NewMCPServer(
		"Nostr NIPs RAG System",
		"1.0.0",
		server.WithLogging(),
	)

	// Add query tool
	queryTool := mcp.NewTool("query_nips",
		mcp.WithDescription("Search Nostr NIPs documentation with semantic search"),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("The query text to search for in the NIPs documentation"),
		),
		mcp.WithNumber("similarity",
			mcp.Description("The similarity threshold for retrieving documents (0.0 to 1.0)"),
		),
		mcp.WithNumber("num_results",
			mcp.Description("The number of similar documents to retrieve"),
		),
	)

	// Add tool handler
	s.AddTool(queryTool, queryNipsHandler)

	// Start the stdio server
	fmt.Println("Starting MCP server for Nostr NIPs RAG system...")
	return server.ServeStdio(s)
}

// queryNipsHandler handles the query_nips tool requests
func queryNipsHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Extract parameters
	query, ok := request.Params.Arguments["query"].(string)
	if !ok || query == "" {
		return nil, errors.New("query must be a non-empty string")
	}

	// Extract optional parameters with defaults
	similarity := 0.6
	if sim, ok := request.Params.Arguments["similarity"].(float64); ok {
		similarity = sim
	}

	numResults := 3
	if num, ok := request.Params.Arguments["num_results"].(float64); ok {
		numResults = int(num)
	}

	// Create embedding from the query
	queryWithPrefix := fmt.Sprintf("search_query: %s", query)
	queryEmbedding, err := embeddings.CreateEmbedding(
		ollamaURL,
		llm.Query4Embedding{
			Model:  embeddingModel,
			Prompt: queryWithPrefix,
		},
		"query",
	)
	if err != nil {
		return nil, fmt.Errorf("error creating embedding: %v", err)
	}

	// Search for similar documents
	similarities, err := globalStore.SearchTopNSimilarities(queryEmbedding, similarity, numResults)
	if err != nil {
		return nil, fmt.Errorf("error searching for similarities: %v", err)
	}

	if len(similarities) == 0 {
		return mcp.NewToolResultText("No similar documents found"), nil
	}

	// Generate context from similarities
	context := embeddings.GenerateContextFromSimilarities(similarities)

	// Return the results
	return mcp.NewToolResultText(context), nil
}
