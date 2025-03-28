# Beating Heart Nostr

A Retrieval-Augmented Generation (RAG) system for the Nostr Protocol NIPs (Nostr Implementation Possibilities). This project creates a semantic search engine for Nostr protocol specifications, allowing developers to quickly find relevant information across all NIPs using natural language queries. Also exposes an MCP server to use it.

## Overview

Beating Heart Nostr serves as the knowledge core for Nostr protocol development:

1. **Intelligent Document Processing**: Semantically chunks the [Nostr NIPs repository](https://github.com/nostr-protocol/nips) with context preservation
2. **Advanced Embedding Generation**: Creates high-quality embeddings using `nomic-embed-text` model with task-specific prefixes
3. **Efficient Vector Storage**: Organizes embeddings in a BBolt vector database
4. **Natural Language Interface**: Provides an intuitive query system that understands developer questions about the protocol

## Requirements

- Go 1.20 or higher
- [Ollama](https://ollama.ai/) running locally with the `nomic-embed-text` model
- Git (for cloning the NIPs repository)

## Installation

```bash
# Clone this repository
git clone https://github.com/yourusername/beating-heart-nostr.git
cd beating-heart-nostr

# Download dependencies
go mod tidy

# Make sure Ollama is running and has the required model
ollama pull nomic-embed-text
```

## Usage

### Running the MCP Server (Default)

By default, running the application will start the MCP server:

```bash
go run .
```

This starts an MCP server that provides the `query_nips` tool for AI agents.

### Creating the RAG Database

To create or update the RAG database:

```bash
go run . -ingest
```

This will:
1. Process all markdown files in the `./data/` directory
2. Create embeddings for each chunk
3. Store the embeddings in `./embeddings.db`

To also clone the Nostr NIPs repository into the data directory:

```bash
go run . -ingest -clone-nips
```

This will clone the repository to `./data/nips-repo/` before processing.

### Querying the RAG Database

To query the RAG database:

```bash
go run . -query -text "What is NIP-01?"
```

Additional options:
- `-similarity`: The similarity threshold for retrieving documents (default: 0.3)
- `-results`: The number of similar documents to retrieve (default: 3)

Example:
```bash
go run . -query -text "What are the message types from relay to client in NIP-01?" -results 5 -similarity 0.25
```

The system will return the most relevant sections from the NIPs documentation that answer your query.

### Running as an MCP Server

The application runs as an MCP server by default. The server provides the following capabilities for AI agents:

#### Tools
- `query_nostr_data`: Searches the Nostr documentation for semantically similar content
  - `query` (required): The search query
  - `similarity` (optional): Similarity threshold (0.0-1.0)
  - `num_results` (optional): Number of results to return

#### Resources
- `nostr://event-kinds`: List of standardized Nostr event kinds and their descriptions
- `nostr://standard-tags`: List of standardized Nostr tags and their descriptions

Test with the MCP inspector:
```bash
npx @modelcontextprotocol/inspector go run .
```

## How It Works

1. **Semantic Chunking**: The system processes markdown files from the `./data/` directory using semantic chunking to preserve the document structure and meaning.

2. **Context-Aware Embeddings**: Each chunk is enhanced with metadata and converted into a vector embedding using the `nomic-embed-text` model with task-specific prefixes:
   - Document chunks use the `search_document:` prefix
   - Queries use the `search_query:` prefix

3. **Overlap Strategy**: The system maintains context continuity between chunks by including overlap from previous sections.

4. **Vector Search**: When you query the system:
   - Your query is converted to an embedding with the appropriate prefix
   - The system finds the most semantically similar document chunks using cosine similarity
   - The top matching chunks are returned as context

5. **Metadata Preservation**: Each chunk maintains information about its source file, section headers, and position in the document hierarchy.

## Customization

You can modify the constants in `main.go` to customize the behavior:

```go
const (
    repoURL        = "https://github.com/nostr-protocol/nips"
    dataDir        = "./data"
    cloneDir       = "./data/nips-repo"
    dbPath         = "./embeddings.db"
    ollamaURL      = "http://localhost:11434"
    embeddingModel = "nomic-embed-text"
)
```

### Adding Custom Documents

To add your own markdown documents to the RAG system:

1. Place your markdown files in the `./data/` directory
2. Run `go run . -ingest` to process the files and update the database

### Key Parameters

- **Similarity Threshold**: Controls how closely a document must match your query (default: 0.3)
- **Number of Results**: Controls how many document chunks are returned (default: 3)
- **Embedding Model**: The model used for creating vector embeddings

## License

This project is open source and available under the [MIT License](LICENSE).

## Contributing

Contributions are welcome! Here are some ways you can contribute:

- Improve the chunking strategy for better semantic understanding
- Enhance the query processing for more accurate results

## Acknowledgments

- [Nostr Protocol](https://github.com/nostr-protocol/nips) for the NIPs documentation
- [Parakeet](https://github.com/parakeet-nest/parakeet) for the RAG implementation
- [Ollama](https://ollama.ai/) for the embedding model
