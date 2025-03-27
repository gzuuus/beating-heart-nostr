# Beating Heart Nostr

A powerful Retrieval-Augmented Generation (RAG) system for the Nostr Protocol NIPs (Nostr Implementation Possibilities). This project creates a semantic search engine for Nostr protocol specifications, allowing developers to quickly find relevant information across all NIPs using natural language queries.

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

### Creating the RAG Database

To create the RAG database from the Nostr NIPs repository:

```bash
go run .
```

This will:
1. Clone the Nostr NIPs repository to `./nips-repo/`
2. Process all markdown files in the repository
3. Create embeddings for each chunk
4. Store the embeddings in `./embeddings.db`

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

## How It Works

1. **Semantic Chunking**: The system processes markdown files from the Nostr NIPs repository using semantic chunking to preserve the document structure and meaning.

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
    cloneDir       = "./nips-repo"
    dbPath         = "./embeddings.db"
    ollamaURL      = "http://localhost:11434"
    embeddingModel = "nomic-embed-text"
)
```

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
- Add support for additional embedding models
- Create a web interface for easier querying

## Acknowledgments

- [Nostr Protocol](https://github.com/nostr-protocol/nips) for the NIPs documentation
- [Parakeet](https://github.com/parakeet-nest/parakeet) for the RAG implementation
- [Ollama](https://ollama.ai/) for the embedding model
