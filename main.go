package main

import (
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/parakeet-nest/parakeet/content"
	"github.com/parakeet-nest/parakeet/embeddings"
	"github.com/parakeet-nest/parakeet/llm"
)

const (
	repoURL        = "https://github.com/nostr-protocol/nips"
	cloneDir       = "./nips-repo"
	dbPath         = "./embeddings.db"
	ollamaURL      = "http://localhost:11434"
	embeddingModel = "nomic-embed-text"
)

// Global counter for generating unique IDs
var embeddingCounter int = 0

func main() {
	// Define command-line flags
	queryMode := flag.Bool("query", false, "Run in query mode")
	queryText := flag.String("text", "", "The query text when in query mode")
	similarity := flag.Float64("similarity", 0.3, "The similarity threshold for retrieving documents")
	numResults := flag.Int("results", 3, "The number of similar documents to retrieve")

	// Parse flags
	flag.Parse()

	if *queryMode {
		// Run in query mode
		if *queryText == "" {
			fmt.Println("Please provide a query using the -text flag")
			flag.Usage()
			os.Exit(1)
		}
		queryDatabase(*queryText, *similarity, *numResults)
	} else {
		// Run in database creation mode
		createDatabase()
	}
}

func createDatabase() {
	// Create a new vector store
	store := embeddings.BboltVectorStore{}
	err := store.Initialize(dbPath)
	if err != nil {
		fmt.Printf("Error initializing vector store: %v\n", err)
		return
	}

	// Clone the repository if it doesn't exist
	fmt.Println("Cloning repository...")
	_, err = git.PlainClone(cloneDir, false, &git.CloneOptions{
		URL:      repoURL,
		Progress: os.Stdout,
	})
	if err != nil && err != git.ErrRepositoryAlreadyExists {
		fmt.Printf("Error cloning repository: %v\n", err)
		return
	}

	// Process the repository files
	fmt.Println("Processing repository files...")
	err = processRepository(cloneDir, &store)
	if err != nil {
		fmt.Printf("Error processing repository: %v\n", err)
		return
	}

	fmt.Println("RAG database created successfully!")
}

func queryDatabase(query string, similarity float64, numResults int) {
	// Initialize the vector store
	store := embeddings.BboltVectorStore{}
	err := store.Initialize(dbPath)
	if err != nil {
		log.Fatalf("Error initializing vector store: %v", err)
	}

	// Create embedding from the query
	fmt.Println("Creating embedding from query...")
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
		log.Fatalf("Error creating embedding: %v", err)
	}

	// Search for similar documents
	fmt.Println("Searching for similar documents...")
	similarities, err := store.SearchTopNSimilarities(queryEmbedding, similarity, numResults)
	if err != nil {
		log.Fatalf("Error searching for similarities: %v", err)
	}

	if len(similarities) == 0 {
		fmt.Println("No similar documents found")
		return
	}

	fmt.Printf("Found %d similar documents\n\n", len(similarities))

	// Generate context from similarities
	context := embeddings.GenerateContextFromSimilarities(similarities)

	fmt.Println(context)

	fmt.Println("")
}

func processRepository(repoPath string, store *embeddings.BboltVectorStore) error {
	// Walk through the repository and process markdown files
	var processedCount int

	return filepath.WalkDir(repoPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip .git directory
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}

		// Process only markdown files
		if !d.IsDir() && strings.HasSuffix(strings.ToLower(d.Name()), ".md") {
			processedCount++
			fmt.Printf("Processing file %d: %s\n", processedCount, path)
			err := processFile(path, store)
			return err
		}

		return nil
	})
}

func processFile(filePath string, store *embeddings.BboltVectorStore) error {
	// Read file content
	fileContent, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("error reading file %s: %v", filePath, err)
	}

	// For protocol specifications, we'll always use semantic chunking
	// as it's the most effective for structured markdown documents
	return processMarkdownChunks(filePath, fileContent, store)
}

// processMarkdownChunks parses markdown into semantic chunks and creates embeddings for each
func processMarkdownChunks(filePath string, fileContent []byte, store *embeddings.BboltVectorStore) error {
	// Extract filename for better metadata
	filename := filepath.Base(filePath)

	// Use Parakeet's markdown parser to create semantically meaningful chunks
	fmt.Printf("Parsing markdown file: %s\n", filePath)
	chunks := content.ParseMarkdownWithLineage(string(fileContent))

	// Process all chunks from the file
	fmt.Printf("Found %d markdown chunks in %s\n", len(chunks), filePath)
	fmt.Printf("Processing %d markdown chunks from %s\n", len(chunks), filePath)

	// Extract NIP number from filename if possible (for protocol specifications)
	nipNumber := extractNipIdentifier(filename)

	// Create embeddings for each chunk and store them
	for i, chunk := range chunks {
		// Increment the counter to generate a unique ID
		embeddingCounter++
		id := fmt.Sprintf("%s-chunk-%d", nipNumber, embeddingCounter)

		parentHeaders := extractParentHeaders(chunk.Lineage)
		metadata := fmt.Sprintf("search_document: Section: %s\nParent Sections: %s\nSource File: %s\n\n%s",
			chunk.Header,
			parentHeaders,
			filePath,
			chunk.Content)

		if i > 0 && len(chunks[i-1].Content) > 0 {
			prevContent := chunks[i-1].Content
			overlapText := extractOverlap(prevContent)
			if overlapText != "" {
				metadata = fmt.Sprintf("%s\n\nContext from previous section:\n%s", metadata, overlapText)
			}
		}

		fmt.Printf("Creating embedding for chunk %s (header: %s)\n", id, chunk.Header)

		// Create embedding
		embedding, err := embeddings.CreateEmbedding(
			ollamaURL,
			llm.Query4Embedding{
				Model:  embeddingModel,
				Prompt: metadata,
			},
			id,
		)

		if err != nil {
			fmt.Printf("Warning: Error creating embedding for %s: %v\n", id, err)
			continue
		}

		// Save embedding to the store
		_, err = store.Save(embedding)
		if err != nil {
			fmt.Printf("Warning: Error saving embedding for %s: %v\n", id, err)
		}
	}

	return nil
}

// extractParentHeaders extracts parent section headers from the lineage string
func extractParentHeaders(lineage string) string {
	if lineage == "" {
		return "Root"
	}

	// Split lineage by '>' and clean up
	parts := strings.Split(lineage, ">")
	var cleanParts []string

	for _, part := range parts {
		cleanPart := strings.TrimSpace(part)
		if cleanPart != "" {
			cleanParts = append(cleanParts, cleanPart)
		}
	}

	return strings.Join(cleanParts, " > ")
}

// extractOverlap extracts the last 1-2 sentences from text for overlap
func extractOverlap(text string) string {
	sentenceRegex := regexp.MustCompile(`[.!?]\s+`)
	sentences := sentenceRegex.Split(text, -1)
	if len(sentences) <= 1 {
		return text
	} else if len(sentences[len(sentences)-1]) < 20 && len(sentences) > 2 {
		// If last sentence is very short, include 2 sentences
		return strings.Join(sentences[len(sentences)-2:], ". ") + "."
	} else {
		// Otherwise just the last sentence
		return sentences[len(sentences)-1] + "."
	}
}

// extractNipIdentifier extracts a simple identifier from a filename
func extractNipIdentifier(filename string) string {
	return strings.TrimSuffix(filename, ".md")
}
