package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/parakeet-nest/parakeet/embeddings"
	"github.com/parakeet-nest/parakeet/llm"
)

var globalStore embeddings.BboltVectorStore

func StartMCPServer() error {
	// Load repository configurations if not already done
	if len(repos) == 0 {
		loadReposConfig("")
	}

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

	eventKindsResource := mcp.NewResource(
		"nostr://event-kinds",
		"Nostr Event Kinds",
		mcp.WithResourceDescription("List of standardized Nostr event kinds and their descriptions"),
		mcp.WithMIMEType("text/markdown"),
	)
	s.AddResource(eventKindsResource, eventKindsResourceHandler)

	standardTagsResource := mcp.NewResource(
		"nostr://standard-tags",
		"Nostr Standardized Tags",
		mcp.WithResourceDescription("List of standardized Nostr tags and their descriptions"),
		mcp.WithMIMEType("text/markdown"),
	)
	s.AddResource(standardTagsResource, standardTagsResourceHandler)

	fmt.Println("Starting MCP server for Nostr RAG system...")
	return server.ServeStdio(s)
}

func queryNostrDataHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// The query handler only needs the embedding database, not the repositories directly
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

func eventKindsResourceHandler(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	// Find the nips repository in repos
	var nipsRepo RepoConfig
	for _, repo := range repos {
		if repo.Name == "nips" && repo.Enabled {
			nipsRepo = repo
			break
		}
	}

	if nipsRepo.CloneDir == "" {
		return nil, fmt.Errorf("NIPs repository not found or not enabled")
	}

	readmePath := filepath.Join(nipsRepo.CloneDir, "README.md")

	if _, err := os.Stat(readmePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("NIPs repository README not found at %s", readmePath)
	}

	content, err := os.ReadFile(readmePath)
	if err != nil {
		return nil, fmt.Errorf("error reading README: %v", err)
	}

	eventKindsSection := extractSection(string(content), "## Event Kinds", "##")
	if eventKindsSection == "" {
		return nil, errors.New("event kinds section not found in README")
	}

	formattedContent := fmt.Sprintf("# Nostr Event Kinds\n\n%s", eventKindsSection)

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      request.Params.URI,
			MIMEType: "text/markdown",
			Text:     formattedContent,
		},
	}, nil
}

func standardTagsResourceHandler(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	// Find the nips repository in repos
	var nipsRepo RepoConfig
	for _, repo := range repos {
		if repo.Name == "nips" && repo.Enabled {
			nipsRepo = repo
			break
		}
	}

	if nipsRepo.CloneDir == "" {
		return nil, fmt.Errorf("NIPs repository not found or not enabled")
	}

	readmePath := filepath.Join(nipsRepo.CloneDir, "README.md")

	if _, err := os.Stat(readmePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("NIPs repository README not found at %s", readmePath)
	}

	content, err := os.ReadFile(readmePath)
	if err != nil {
		return nil, fmt.Errorf("error reading README: %v", err)
	}

	tagsSection := extractSection(string(content), "## Standardized Tags", "##")
	if tagsSection == "" {
		return nil, errors.New("standardized tags section not found in README")
	}

	formattedContent := fmt.Sprintf("# Nostr Standardized Tags\n\n%s", tagsSection)

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      request.Params.URI,
			MIMEType: "text/markdown",
			Text:     formattedContent,
		},
	}, nil
}

func extractSection(content, startMarker, endMarker string) string {
	startIndex := strings.Index(content, startMarker)
	if startIndex == -1 {
		return ""
	}

	endIndex := strings.Index(content[startIndex+len(startMarker):], endMarker)
	if endIndex == -1 {
		return strings.TrimSpace(content[startIndex:])
	}

	sectionContent := content[startIndex : startIndex+len(startMarker)+endIndex]
	return strings.TrimSpace(sectionContent)
}
