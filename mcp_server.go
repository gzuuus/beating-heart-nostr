package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip19"
	"github.com/parakeet-nest/parakeet/embeddings"
	"github.com/parakeet-nest/parakeet/llm"
)

var globalStore embeddings.BboltVectorStore

// CodeSnippetCache stores code snippet events from Nostr relays
type CodeSnippetCache struct {
	events     []*nostr.Event
	lastUpdate time.Time
	mutex      sync.RWMutex
}

// Global cache for code snippets
var codeSnippetCache = CodeSnippetCache{}

func StartMCPServer() error {
	// Load repository configurations if not already done
	if len(repos) == 0 {
		loadReposConfig("")
	}

	err := globalStore.Initialize(dbPath)
	if err != nil {
		return fmt.Errorf("error initializing vector store: %v", err)
	}
	
	// Start background process to populate code snippet cache
	go populateCodeSnippetCache()

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

	// Add the code snippets search tool
	codeSnippetsTool := mcp.NewTool("search_code_snippets",
		mcp.WithDescription("Searches for code snippets in the Nostr network using kind 1337 events."),
		mcp.WithString("language",
			mcp.Description("The programming language to search for (e.g., 'javascript', 'python', 'rust'). Optional but recommended."),
		),
		mcp.WithString("author",
			mcp.Description("Optional author's public key or npub to filter by"),
		),
		mcp.WithString("query",
			mcp.Description("Optional search query to match against name, description, license, runtime, etc."),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum number of code snippets to return (default: 10)"),
		),
	)

	s.AddTool(codeSnippetsTool, searchCodeSnippetsHandler)

	// fmt.Println("Starting MCP server for Nostr RAG system...")
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

// populateCodeSnippetCache fetches code snippets from relays and stores them in memory
func populateCodeSnippetCache() {
	// Run initial population
	updateCodeSnippetCache()

	// Set up ticker to refresh cache periodically (every 30 minutes)
	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		updateCodeSnippetCache()
	}
}

// updateCodeSnippetCache refreshes the code snippet cache with events from relays
func updateCodeSnippetCache() {
	// fmt.Println("Updating code snippet cache...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// List of relays to connect to
	relays := []string{
		"wss://relay.damus.io",
		"wss://relay.nostr.band",
		"wss://nos.lol",
		"wss://relay.snort.social",
	}

	// Create a filter for all code snippets (kind 1337)
	filter := nostr.Filter{
		Kinds: []int{1337}, // Code snippet kind
		Limit: 500,        // Get a good number of snippets
	}

	// Collect events from relays
	var newEvents []*nostr.Event
	for _, url := range relays {
		relay, err := nostr.RelayConnect(ctx, url)
		if err != nil {
			// fmt.Printf("Cache update: Failed to connect to relay %s: %v\n", url, err)
			continue
		}

		// Subscribe to the relay with our filter
		sub, err := relay.Subscribe(ctx, []nostr.Filter{filter})
		if err != nil {
			// fmt.Printf("Cache update: Failed to subscribe to relay %s: %v\n", url, err)
			relay.Close()
			continue
		}

		// Collect events from this relay
		for ev := range sub.Events {
			newEvents = append(newEvents, ev)
		}

		// Close the subscription and relay connection
		sub.Unsub()
		relay.Close()
	}

	// Update the cache with new events
	if len(newEvents) > 0 {
		codeSnippetCache.mutex.Lock()
		codeSnippetCache.events = newEvents
		codeSnippetCache.lastUpdate = time.Now()
		codeSnippetCache.mutex.Unlock()
		// fmt.Printf("Code snippet cache updated with %d events\n", len(newEvents))
	} else {
		fmt.Println("No new code snippets found for cache update")
	}
}

// searchCodeSnippetsHandler handles requests to search for code snippets in the Nostr network
func searchCodeSnippetsHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Extract parameters from the request
	language, _ := request.Params.Arguments["language"].(string)
	author, _ := request.Params.Arguments["author"].(string)
	query, _ := request.Params.Arguments["query"].(string)

	// Default limit to 10 if not specified
	limit := 10
	if limitVal, ok := request.Params.Arguments["limit"].(float64); ok {
		limit = int(limitVal)
	}

	// Ensure we have at least one search parameter
	if language == "" && author == "" && query == "" {
		return nil, errors.New("at least one of 'language', 'author', or 'query' must be provided")
	}

	// Process author if provided (convert npub to hex if needed)
	if author != "" && strings.HasPrefix(author, "npub") {
		_, decodedAuthor, err := nip19.Decode(author)
		if err == nil {
			author = decodedAuthor.(string)
		} else {
			fmt.Printf("Failed to decode npub %s: %v\n", author, err)
		}
	}

	// First try to find events in the cache
	cachedEvents := searchCachedEvents(language, author, query, limit)
	
	// If we found enough events in the cache, return them
	if len(cachedEvents) >= limit {
		return formatCodeSnippetResults(cachedEvents, language, author, query, limit)
	}
	
	// If cache is empty or doesn't have enough results, fall back to live relay search
	if len(cachedEvents) == 0 {
		// Special case for query-only searches
		if language == "" && author == "" && query != "" {
			relayEvents := searchByQueryOnly(ctx, query, limit)
			return formatCodeSnippetResults(relayEvents, language, author, query, limit)
		}
		
		relayEvents := searchRelayEvents(ctx, language, author, query, limit)
		return formatCodeSnippetResults(relayEvents, language, author, query, limit)
	} else {
		// We have some results from cache but not enough, so get more from relays
		neededEvents := limit - len(cachedEvents)
		relayEvents := searchRelayEvents(ctx, language, author, query, neededEvents)
		
		// Combine cache and relay results
		combinedEvents := append(cachedEvents, relayEvents...)
		if len(combinedEvents) > limit {
			combinedEvents = combinedEvents[:limit]
		}
		
		return formatCodeSnippetResults(combinedEvents, language, author, query, limit)
	}
}

// searchCachedEvents searches the in-memory cache for matching code snippets
func searchCachedEvents(language, author, query string, limit int) []*nostr.Event {
	// Lock for reading from cache
	codeSnippetCache.mutex.RLock()
	defer codeSnippetCache.mutex.RUnlock()
	
	// Check if cache is empty
	if len(codeSnippetCache.events) == 0 {
		return nil
	}
	
	// Filter events from cache based on criteria
	var matchingEvents []*nostr.Event
	for _, ev := range codeSnippetCache.events {
		// Check language filter
		if language != "" {
			langMatch := false
			for _, tag := range ev.Tags {
				if len(tag) >= 2 && tag[0] == "l" && strings.EqualFold(tag[1], language) {
					langMatch = true
					break
				}
			}
			if !langMatch {
				continue
			}
		}
		
		// Check author filter
		if author != "" && ev.PubKey != author {
			continue
		}
		
		// Check query filter - always match if query is empty
		if query != "" && !matchesQuery(ev, query) {
			continue
		}
		
		// Event matches all criteria
		matchingEvents = append(matchingEvents, ev)
		if len(matchingEvents) >= limit {
			break
		}
	}
	
	return matchingEvents
}

// searchRelayEvents searches live relays for matching code snippets
func searchRelayEvents(ctx context.Context, language, author, query string, limit int) []*nostr.Event {
	// If we have a query but no language or author, use a more general approach
	if query != "" && language == "" && author == "" {
		return searchByQueryOnly(ctx, query, limit)
	}
	
	// List of relays to connect to
	relays := []string{
		"wss://relay.damus.io",
		"wss://purplepag.es",
		"wss://relay.current.fyi",
		"wss://relay.nostr.band",
		"wss://nos.lol",
		"wss://relay.snort.social",
	}

	// Create a filter for code snippets (kind 1337)
	filter := nostr.Filter{
		Kinds: []int{1337}, // Code snippet kind
		Limit: limit,
	}

	// Add language filter if provided
	if language != "" {
		filter.Tags = map[string][]string{"l": {strings.ToLower(language)}}
	}

	// Add author filter if provided
	if author != "" {
		filter.Authors = []string{author}
	}

	// Connect to relays and collect events
	var events []*nostr.Event
	for _, url := range relays {
		relay, err := nostr.RelayConnect(ctx, url)
		if err != nil {
			fmt.Printf("Failed to connect to relay %s: %v\n", url, err)
			continue
		}

		// Set a timeout for subscription - use a longer timeout to ensure we get results
		subCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		// Subscribe to the relay with our filters
		sub, err := relay.Subscribe(subCtx, []nostr.Filter{filter})
		if err != nil {
			fmt.Printf("Failed to subscribe to relay %s: %v\n", url, err)
			continue
		}

		// Collect events from this relay
		for ev := range sub.Events {
			// Apply additional filtering based on query if provided
			if query == "" || matchesQuery(ev, query) {
				events = append(events, ev)
			}

			// Break if we've reached our limit
			if len(events) >= limit {
				break
			}
		}

		// Close the subscription
		sub.Unsub()
		relay.Close()

		// If we've collected enough events, stop connecting to more relays
		if len(events) >= limit {
			break
		}
	}
	
	return events
}

// formatCodeSnippetResults formats the code snippet events into a readable result
func formatCodeSnippetResults(events []*nostr.Event, language, author, query string, limit int) (*mcp.CallToolResult, error) {
	// Format the results
	if len(events) == 0 {
		return mcp.NewToolResultText("No code snippets found matching the criteria."), nil
	}

	// Build a formatted response with the code snippets
	var result strings.Builder
	
	// Create appropriate header based on search parameters
	if language != "" && author != "" {
		result.WriteString(fmt.Sprintf("Found %d code snippets for language '%s' by author '%s':\n\n", len(events), language, author))
	} else if language != "" {
		result.WriteString(fmt.Sprintf("Found %d code snippets for language '%s':\n\n", len(events), language))
	} else if author != "" {
		result.WriteString(fmt.Sprintf("Found %d code snippets by author '%s':\n\n", len(events), author))
	} else {
		result.WriteString(fmt.Sprintf("Found %d code snippets matching query '%s':\n\n", len(events), query))
	}

	for i, ev := range events {
		// Extract tags for display
		// Check for 'name' tag first, then 'f' tag as fallback, then default to "Unnamed Snippet"
		snippetName := getTagValue(ev, "name", "")
		if snippetName == "" {
			snippetName = getTagValue(ev, "f", "Unnamed Snippet")
		}
		
		snippetExt := getTagValue(ev, "extension", "")
		snippetDesc := getTagValue(ev, "description", "No description provided")
		snippetRuntime := getTagValue(ev, "runtime", "")
		snippetLicense := getTagValue(ev, "license", "")
		
		// Get language from tag if not provided in search
		snippetLang := language
		if snippetLang == "" {
			snippetLang = getTagValue(ev, "l", "text")
		}

		// Format the snippet metadata
		result.WriteString(fmt.Sprintf("## Snippet %d: %s\n", i+1, snippetName))
		result.WriteString(fmt.Sprintf("**Description:** %s\n", snippetDesc))
		
		// Add additional metadata if available
		if snippetExt != "" {
			result.WriteString(fmt.Sprintf("**Extension:** %s\n", snippetExt))
		}
		if snippetRuntime != "" {
			result.WriteString(fmt.Sprintf("**Runtime:** %s\n", snippetRuntime))
		}
		if snippetLicense != "" {
			result.WriteString(fmt.Sprintf("**License:** %s\n", snippetLicense))
		}

		// Add author information
		npub, _ := nip19.EncodePublicKey(ev.PubKey)
		result.WriteString(fmt.Sprintf("**Author:** %s\n", npub))

		// Add the code snippet with proper markdown formatting
		result.WriteString("```" + snippetLang + "\n")
		result.WriteString(ev.Content)
		result.WriteString("\n```\n\n")
	}

	return mcp.NewToolResultText(result.String()), nil
}

// matchesQuery checks if an event matches the query string across multiple tag fields
func matchesQuery(ev *nostr.Event, query string) bool {
	// If query is empty, match everything
	if query == "" {
		return true
	}
	
	// Clean and normalize the query
	query = strings.ToLower(strings.TrimSpace(query))
	
	// For exact matches like "ndk", just check directly first
	if len(query) >= 2 && len(query) <= 10 {
		// Check content directly
		if strings.Contains(strings.ToLower(ev.Content), query) {
			return true
		}
		
		// Check all tags directly
		for _, tag := range ev.Tags {
			if len(tag) >= 2 {
				tagValue := strings.ToLower(tag[1])
				if strings.Contains(tagValue, query) {
					return true
				}
			}
		}
	}
	
	// Split query into words for multi-word queries
	words := strings.Fields(query)
	if len(words) == 0 {
		return true // Empty query after trimming
	}
	
	// Check if ANY word matches ANY field (very lenient approach)
	for _, word := range words {
		// Skip very short words (likely not meaningful)
		if len(word) < 2 {
			continue
		}
		
		// Check content
		if strings.Contains(strings.ToLower(ev.Content), word) {
			return true
		}
		
		// Check all tags
		for _, tag := range ev.Tags {
			if len(tag) >= 2 {
				tagValue := strings.ToLower(tag[1])
				if strings.Contains(tagValue, word) {
					return true
				}
			}
		}
	}
	
	return false
}

// contains checks if a string is in a slice
func contains(slice []string, str string) bool {
	for _, item := range slice {
		if item == str {
			return true
		}
	}
	return false
}

// Debug function to print event details - useful for troubleshooting but disabled by default
func debugEvent(ev *nostr.Event) {
	// Disabled to avoid interfering with MCP protocol
	_ = ev
	/*
	fmt.Printf("\nEvent ID: %s\n", ev.ID)
	fmt.Printf("Content: %s\n", ev.Content[:min(50, len(ev.Content))])
	fmt.Println("Tags:")
	for _, tag := range ev.Tags {
		if len(tag) >= 2 {
			fmt.Printf("  %s: %s\n", tag[0], tag[1])
		}
	}
	*/
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// searchByQueryOnly performs a broader search when only a query is provided
func searchByQueryOnly(ctx context.Context, query string, limit int) []*nostr.Event {
	// First check the cache for matches
	cachedResults := searchCachedEvents("", "", query, limit)
	if len(cachedResults) > 0 {
		return cachedResults
	}
	
	// List of relays to connect to - just use a few reliable ones
	relays := []string{
		"wss://relay.damus.io",
		"wss://purplepag.es",
	}
	
	// Just get all code snippets and filter locally
	filter := nostr.Filter{
		Kinds: []int{1337}, // Code snippet kind
		Limit: 50,         // Get a reasonable number to filter locally
		// No time filter to ensure we get results
	}
	
	// Connect to relays and collect events
	var events []*nostr.Event
	var eventIDs = make(map[string]bool) // To avoid duplicates
	
	for _, url := range relays {
		relay, err := nostr.RelayConnect(ctx, url)
		if err != nil {
			continue
		}

		// Set a shorter timeout for subscription to avoid hanging
		subCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		// Subscribe to the relay with our filters
		sub, err := relay.Subscribe(subCtx, []nostr.Filter{filter})
		if err != nil {
			relay.Close()
			continue
		}

		// Collect events from this relay
		for ev := range sub.Events {
			// Skip if we've seen this event before
			if eventIDs[ev.ID] {
				continue
			}
			
			// Apply query filtering
			if matchesQuery(ev, query) {
				events = append(events, ev)
				eventIDs[ev.ID] = true
				
				// Break if we've reached our limit
				if len(events) >= limit {
					break
				}
			}
		}

		// Close the subscription
		sub.Unsub()
		relay.Close()

		// If we've collected enough events, stop connecting to more relays
		if len(events) >= limit {
			break
		}
	}
	
	return events
}

// getTagValue retrieves a tag value from a Nostr event
func getTagValue(ev *nostr.Event, tagName, defaultValue string) string {
	for _, tag := range ev.Tags {
		if len(tag) >= 2 && tag[0] == tagName {
			return tag[1]
		}
	}
	return defaultValue
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
