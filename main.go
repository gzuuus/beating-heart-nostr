package main

import (
	"encoding/json"
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
	dataDir        = "./data"
	dbPath         = "./embeddings.db"
	ollamaURL      = "http://localhost:11434"
	embeddingModel = "nomic-embed-text"
)

// RepoConfig holds configuration for a repository to be included in the RAG system
type RepoConfig struct {
	URL      string // Repository URL
	Name     string // Repository name (used for directory naming)
	CloneDir string // Directory where the repo will be cloned
	Enabled  bool   // Whether this repo is enabled
}

// configFile is the path to the repository configuration file
const configFile = "repos.json"

// repos holds the repositories that are configured in the system
var repos []RepoConfig

// Global counter for generating unique IDs
var embeddingCounter int = 0

func main() {
	// Define command-line flags
	queryMode := flag.Bool("query", false, "Run in query mode")
	queryText := flag.String("text", "", "The query text when in query mode")
	similarity := flag.Float64("similarity", 0.6, "The similarity threshold for retrieving documents")
	numResults := flag.Int("results", 3, "The number of similar documents to retrieve")
	_ = flag.Bool("mcp", true, "Run as an MCP server (default)")
	ingestMode := flag.Bool("ingest", false, "Ingest data into the RAG database")
	cloneRepos := flag.Bool("clone-repos", false, "Clone all enabled repositories into the data directory")

	// Repository configuration flags
	customConfigFile := flag.String("repos-config", "", "Path to a custom JSON file containing repository configurations")
	addRepo := flag.String("add-repo", "", "Add a repository in format 'url,name' (e.g., 'https://github.com/example/repo,example')")
	listRepos := flag.Bool("list-repos", false, "List all configured repositories")

	// Parse flags
	flag.Parse()

	// Create data directory if it doesn't exist
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		err := os.MkdirAll(dataDir, 0755)
		if err != nil {
			log.Fatalf("Error creating data directory: %v", err)
		}
	}

	// Load repository configurations
	loadReposConfig(*customConfigFile)

	// Add a new repository if requested
	if *addRepo != "" {
		addRepository(*addRepo)
	}

	if *listRepos {
		// List all configured repositories
		listRepositories()
	} else if *cloneRepos {
		// Just clone the repositories without ingestion
		cloneAllRepositories()
	} else if *ingestMode {
		// Run in database creation mode
		fmt.Println("Starting data ingestion...")
		createDatabase(*cloneRepos)
	} else if *queryMode {
		// Run in query mode
		if *queryText == "" {
			fmt.Println("Please provide a query using the -text flag")
			flag.Usage()
			os.Exit(1)
		}
		queryDatabase(*queryText, *similarity, *numResults)
	} else {
		// Run as an MCP server (default)
		// fmt.Println("Starting in MCP server mode...")
		err := StartMCPServer()
		if err != nil {
			log.Fatalf("Error running MCP server: %v", err)
		}
	}
}

// cloneAllRepositories clones all enabled repositories in the configuration
func cloneAllRepositories() {
	if len(repos) == 0 {
		fmt.Println("No repositories configured. Create a repos.json file or use -add-repo to add repositories.")
		return
	}

	fmt.Println("Cloning all enabled repositories...")
	for _, repo := range repos {
		if !repo.Enabled {
			continue
		}

		fmt.Printf("Cloning repository: %s...\n", repo.Name)
		_, err := git.PlainClone(repo.CloneDir, false, &git.CloneOptions{
			URL:      repo.URL,
			Progress: os.Stdout,
		})
		if err != nil && err != git.ErrRepositoryAlreadyExists {
			fmt.Printf("Error cloning repository %s: %v\n", repo.Name, err)
			// Continue with other repositories even if one fails
		}
	}
	fmt.Println("Cloning completed.")
}

func createDatabase(cloneRepos bool) {
	// Create a new vector store
	store := embeddings.BboltVectorStore{}
	err := store.Initialize(dbPath)
	if err != nil {
		fmt.Printf("Error initializing vector store: %v\n", err)
		return
	}

	// Clone all enabled repositories if requested
	if cloneRepos {
		cloneAllRepositories()
	}

	// Process all markdown files in the data directory
	fmt.Println("Processing markdown files in data directory...")
	err = processDataDirectory(&store)
	if err != nil {
		fmt.Printf("Error processing data directory: %v\n", err)
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

// loadReposConfig loads the repository configuration from a file
func loadReposConfig(customConfigFile string) {
	// Determine which config file to use
	cfgFile := configFile
	if customConfigFile != "" {
		cfgFile = customConfigFile
	}

	// Check if the config file exists
	if _, err := os.Stat(cfgFile); os.IsNotExist(err) {
		// If it's the default config file and it doesn't exist, create an empty one
		if cfgFile == configFile {
			fmt.Printf("No repository configuration file found at %s. Creating an empty one.\n", cfgFile)
			repos = []RepoConfig{}
			saveReposToFile(cfgFile)
		} else {
			// If it's a custom config file that doesn't exist, exit with error
			fmt.Printf("Error: Configuration file %s not found\n", cfgFile)
			os.Exit(1)
		}
		return
	}

	// Load the repositories from the file
	file, err := os.ReadFile(cfgFile)
	if err != nil {
		fmt.Printf("Error reading repository config file: %v\n", err)
		os.Exit(1)
	}

	err = json.Unmarshal(file, &repos)
	if err != nil {
		fmt.Printf("Error parsing repository config file: %v\n", err)
		os.Exit(1)
	}

	// Ensure clone directories are properly set
	for i := range repos {
		if repos[i].CloneDir == "" {
			repos[i].CloneDir = filepath.Join(dataDir, repos[i].Name+"-repo")
		}
	}

	// Ensure at least one repository is enabled if we have repositories
	if len(repos) > 0 {
		hasEnabled := false
		for _, repo := range repos {
			if repo.Enabled {
				hasEnabled = true
				break
			}
		}

		if !hasEnabled {
			// Enable the first repository if none are enabled
			repos[0].Enabled = true
			saveReposToFile(cfgFile)
		}
	}
}

// addRepository adds a new repository to the configuration
func addRepository(addRepoStr string) {
	parts := strings.Split(addRepoStr, ",")
	if len(parts) < 2 {
		fmt.Println("Error: Repository must be specified as 'url,name'")
		os.Exit(1)
	}

	url := parts[0]
	name := parts[1]

	// Check if repository already exists
	for _, repo := range repos {
		if repo.URL == url {
			fmt.Printf("Repository with URL %s already exists\n", url)
			return
		}
	}

	// Add the new repository
	newRepo := RepoConfig{
		URL:      url,
		Name:     name,
		CloneDir: filepath.Join(dataDir, name+"-repo"),
		Enabled:  true,
	}

	repos = append(repos, newRepo)
	saveReposToFile(configFile) // Always save to the default config file
	fmt.Printf("Added repository: %s (%s)\n", name, url)
}

// saveReposToFile saves the current repository configurations to a JSON file
func saveReposToFile(filePath string) {
	data, err := json.MarshalIndent(repos, "", "  ")
	if err != nil {
		fmt.Printf("Error serializing repository config: %v\n", err)
		return
	}

	err = os.WriteFile(filePath, data, 0644)
	if err != nil {
		fmt.Printf("Error writing repository config file: %v\n", err)
	}
}

// listRepositories displays all configured repositories
func listRepositories() {
	if len(repos) == 0 {
		fmt.Println("No repositories configured. Use -add-repo to add a repository.")
		return
	}

	fmt.Println("Configured Repositories:")
	fmt.Println("------------------------")

	for i, repo := range repos {
		status := "Disabled"
		if repo.Enabled {
			status = "Enabled"
		}

		fmt.Printf("%d. %s (%s)\n", i+1, repo.Name, status)
		fmt.Printf("   URL: %s\n", repo.URL)
		fmt.Printf("   Clone Directory: %s\n", repo.CloneDir)
		fmt.Println()
	}
}

func processDataDirectory(store *embeddings.BboltVectorStore) error {
	if len(repos) == 0 {
		fmt.Println("No repositories configured. Use -add-repo to add a repository.")
		return fmt.Errorf("no repositories configured")
	}

	// Process all enabled repositories
	for _, repo := range repos {
		if !repo.Enabled {
			continue
		}

		fmt.Printf("Processing repository: %s\n", repo.Name)
		err := processRepository(repo.CloneDir, store, repo.Name)
		if err != nil {
			fmt.Printf("Error processing repository %s: %v\n", repo.Name, err)
			// Continue with other repositories even if one fails
		}
	}

	return nil
}

// processRepository processes all markdown files in a specific repository
func processRepository(repoDir string, store *embeddings.BboltVectorStore, repoName string) error {
	// Walk through the repository directory and process markdown files
	var processedCount int

	return filepath.WalkDir(repoDir, func(path string, d fs.DirEntry, err error) error {
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
			fmt.Printf("Processing file %d from %s: %s\n", processedCount, repoName, path)
			err := processFile(path, store, repoName)
			return err
		}

		return nil
	})
}

func processFile(filePath string, store *embeddings.BboltVectorStore, repoName string) error {
	// Read file content
	fileContent, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("error reading file %s: %v", filePath, err)
	}

	// For protocol specifications, we'll always use semantic chunking
	// as it's the most effective for structured markdown documents
	return processMarkdownChunks(filePath, fileContent, store, repoName)
}

// processMarkdownChunks parses markdown into semantic chunks and creates embeddings for each
func processMarkdownChunks(filePath string, fileContent []byte, store *embeddings.BboltVectorStore, repoName string) error {
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
		metadata := fmt.Sprintf("search_document: Section: %s\nParent Sections: %s\n\n%s",
			chunk.Header,
			parentHeaders,
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
