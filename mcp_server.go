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

func StartMCPServer() error {
	err := globalStore.Initialize(dbPath)
	if err != nil {
		return fmt.Errorf("error initializing vector store: %v", err)
	}

	s := server.NewMCPServer(
		"Beating Heart Nostr RAG System",
		"1.0.0",
		server.WithLogging(),
	)

	queryTool := mcp.NewTool("query_nostr_data",
		mcp.WithDescription("Searches the Nostr documentation for documents semantically similar to the input query."),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("The query text to search for in the Nostr documentation"),
		),
		mcp.WithNumber("similarity",
			mcp.Description("The similarity threshold for retrieving documents (0.0 to 1.0)"),
		),
		mcp.WithNumber("num_results",
			mcp.Description("The number of similar documents to retrieve"),
		),
	)

	s.AddTool(queryTool, queryNostrDataHandler)

	fmt.Println("Starting MCP server for Nostr RAG system...")
	return server.ServeStdio(s)
}

func queryNostrDataHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	query, ok := request.Params.Arguments["query"].(string)
	if !ok || query == "" {
		return nil, errors.New("query must be a non-empty string")
	}

	similarity := 0.6
	if sim, ok := request.Params.Arguments["similarity"].(float64); ok {
		similarity = sim
	}

	numResults := 3
	if num, ok := request.Params.Arguments["num_results"].(float64); ok {
		numResults = int(num)
	}

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

	similarities, err := globalStore.SearchTopNSimilarities(queryEmbedding, similarity, numResults)
	if err != nil {
		return nil, fmt.Errorf("error searching for similarities: %v", err)
	}

	if len(similarities) == 0 {
		return mcp.NewToolResultText("No similar documents found"), nil
	}

	context := embeddings.GenerateContextFromSimilarities(similarities)

	return mcp.NewToolResultText(context), nil
}
