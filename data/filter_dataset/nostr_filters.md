# Nostr Filters Dataset

This document contains a comprehensive collection of Nostr filters and their natural language descriptions. The dataset is designed to be used with a RAG (Retrieval-Augmented Generation) system to facilitate retrieval of relevant Nostr filter patterns through natural language queries.

> **Note**: Each filter example in this document uses generic placeholders like `<pubkey1>`, `<event_id1>`, and `<topic1>` instead of specific values. This allows the examples to focus on structure and purpose rather than specific content.

## Basic Filter Structure

A Nostr filter is a JSON object that specifies which events a client is interested in receiving from a relay. Filters are used in subscription requests (`REQ` messages) and determine what data will be returned.

```json
{
  "ids": ["<event_id>", ...],
  "authors": ["<pubkey>", ...],
  "kinds": [<kind_number>, ...],
  "#<tag>": ["<tag_value>", ...],
  "since": <unix_timestamp>,
  "until": <unix_timestamp>,
  "limit": <max_events>
}
```

## Filter Attributes

### IDs Filter

The `ids` attribute filters events by their specific event IDs.

```json
{
  "ids": ["<event_id1>"]
}
```

**Description**: A filter that retrieves a specific event with the given event ID.

```json
{
  "ids": [
    "<event_id1>",
    "<event_id2>"
  ]
}
```

**Description**: A filter that retrieves multiple specific events by their IDs.

### Authors Filter

The `authors` attribute filters events by their author's public key.

```json
{
  "authors": ["<pubkey1>"]
}
```

**Description**: A filter that retrieves all events published by a specific author with the given public key.

```json
{
  "authors": [
    "<pubkey1>",
    "<pubkey2>"
  ]
}
```

**Description**: A filter that retrieves events from multiple specific authors identified by their public keys.

### Kinds Filter

The `kinds` attribute filters events by their kind number.

```json
{
  "kinds": [1]
}
```

**Description**: A filter that retrieves all text note events (kind 1).

```json
{
  "kinds": [0, 3]
}
```

**Description**: A filter that retrieves both metadata events (kind 0) and contact list events (kind 3).

```json
{
  "kinds": [30023]
}
```

**Description**: A filter that retrieves all long-form content events (kind 30023).

### Tag Filters

Tag filters allow filtering based on specific tag values.

```json
{
  "#e": ["<event_id1>"]
}
```

**Description**: A filter that retrieves events that reference a specific event ID in their 'e' tags.

```json
{
  "#p": ["<pubkey1>"]
}
```

**Description**: A filter that retrieves events that mention a specific user (public key) in their 'p' tags.

```json
{
  "#t": ["<topic1>", "<topic2>"]
}
```

**Description**: A filter that retrieves events with either of the specified hashtags in their 't' tags.

### Time-based Filters

The `since` and `until` attributes filter events based on their creation time.

```json
{
  "since": 1609459200
}
```

**Description**: A filter that retrieves all events created after January 1, 2021 (Unix timestamp 1609459200).

```json
{
  "until": 1640995199
}
```

**Description**: A filter that retrieves all events created before December 31, 2021 (Unix timestamp 1640995199).

```json
{
  "since": 1609459200,
  "until": 1640995199
}
```

**Description**: A filter that retrieves all events created during the year 2021, between January 1 and December 31.

### Limit Filter

The `limit` attribute restricts the number of events returned.

```json
{
  "limit": 10
}
```

**Description**: A filter that limits the results to the 10 most recent events.

## Combined Filters

Filters can combine multiple attributes to create more specific queries. Each combination serves a distinct purpose and retrieves a targeted set of events.

### Author-specific Content with Limit

```json
{
  "authors": ["<pubkey1>"],
  "kinds": [1],
  "limit": 20
}
```

**Description**: A filter that retrieves the 20 most recent text notes (kind 1) from a specific author.

### Content by Topic with Time Constraint

```json
{
  "kinds": [1],
  "#t": ["<topic1>"],
  "since": 1640995200
}
```

**Description**: A filter that retrieves all text notes with a specific hashtag created after January 1, 2022.

### Author Content Mentioning Specific User

```json
{
  "authors": ["<pubkey1>"],
  "kinds": [1, 6],
  "#p": ["<pubkey2>"]
}
```

**Description**: A filter that retrieves all text notes (kind 1) and reposts (kind 6) from a specific author that mention a particular user.

## Common Use Cases

This section presents filters designed for specific real-world use cases in Nostr applications.

### User Profile Retrieval

```json
{
  "authors": ["<pubkey1>"],
  "kinds": [0],
  "limit": 1
}
```

**Description**: A filter to retrieve the most recent profile metadata (kind 0) for a specific user. This is commonly used when displaying user profiles or verifying user information.

### Contact List Retrieval

```json
{
  "authors": ["<pubkey1>"],
  "kinds": [3],
  "limit": 1
}
```

**Description**: A filter to retrieve the most recent contact list (kind 3) for a specific user. This is essential for building social graphs and showing user connections in Nostr clients.

### Conversation Thread Retrieval

```json
{
  "#e": ["<event_id1>"],
  "kinds": [1]
}
```

**Description**: A filter to retrieve all text notes (kind 1) that are part of a conversation thread referencing a specific event. This is used to display threaded conversations and replies to a particular post.

### Direct Messages Retrieval

```json
{
  "kinds": [4],
  "#p": ["<pubkey1>"]
}
```

**Description**: A filter to retrieve all encrypted direct messages (kind 4) that mention a specific user. This is used in private messaging features of Nostr clients to show conversations between users.

### Network-wide Feed Retrieval

```json
{
  "kinds": [1],
  "limit": 100
}
```

**Description**: A filter to retrieve the 100 most recent text notes (kind 1) from all users across the network. This is used to create a general timeline or discovery feed in Nostr clients.

### Long-form Content Retrieval

```json
{
  "authors": ["<pubkey1>"],
  "kinds": [30023]
}
```

**Description**: A filter to retrieve all long-form content (kind 30023) such as articles and blogs published by a specific user. This is used for displaying a user's published articles or blog posts in Nostr clients.

### Multiple Hashtag Content Retrieval

```json
{
  "#t": ["<topic1>", "<topic2>"],
  "kinds": [1],
  "limit": 50
}
```

**Description**: A filter to retrieve the 50 most recent text notes (kind 1) that contain either of the specified hashtags. This is used for topic-based discovery and content aggregation by subject matter.

## Advanced Filter Patterns

This section covers more specialized filter patterns for complex use cases in Nostr applications.

### Reaction Events Retrieval

```json
{
  "kinds": [7],
  "#e": ["<event_id1>"]
}
```

**Description**: A filter to retrieve all reaction events (kind 7) to a specific event. This is used to show likes, emojis, and other reactions to a particular post or note.

### Contact List Feed Retrieval

```json
{
  "authors": [
    "<pubkey1>",
    "<pubkey2>",
    "<pubkey3>"
  ],
  "kinds": [1],
  "limit": 100
}
```

**Description**: A filter to retrieve the 100 most recent text notes from a list of users in a contact list. This is used to create personalized feeds showing content only from followed users.

### Geolocation-based Content Retrieval

```json
{
  "#g": ["<geohash>"],
  "kinds": [1]
}
```

**Description**: A filter to retrieve text notes that contain a specific geohash location tag. This is used for location-based discovery and showing content from specific geographic areas.

### URL Reference Content Retrieval

```json
{
  "#r": ["<url>"],
  "kinds": [1]
}
```

**Description**: A filter to retrieve text notes that reference a specific URL. This is used to find discussions about particular web content or to track mentions of specific resources.

### Time-bounded Multi-type Content Retrieval

```json
{
  "authors": ["<pubkey1>"],
  "kinds": [1, 6, 7],
  "since": 1640995200,
  "until": 1672531199,
  "limit": 50
}
```

**Description**: A filter to retrieve the 50 most recent text notes, reposts, and reactions from a specific author created during a specific time period (in this example, the year 2022). This is used for historical content analysis or viewing a user's activity during a particular timeframe.

### Lightning Payment (Zap) Retrieval

```json
{
  "kinds": [9735],
  "#p": ["<pubkey1>"]
}
```

**Description**: A filter to retrieve all zap events (kind 9735) sent to a specific user. This is used to track Lightning Network payments and financial activity within the Nostr ecosystem.

### Delegated Content Retrieval

```json
{
  "#delegation": ["<pubkey1>"],
  "kinds": [1]
}
```

**Description**: A filter to retrieve text notes that were published through delegation by a specific delegator. This is used to track content published on behalf of users via the NIP-26 delegation mechanism.

## Multiple Filters in a Single Request

In Nostr, a subscription request can include multiple filters, which are combined with OR logic.

```
["REQ", "subscription_id", 
  {
    "authors": ["<pubkey1>"],
    "kinds": [1]
  },
  {
    "#t": ["<topic1>"],
    "kinds": [1]
  }
]
```

**Description**: A subscription request with two filters: one to retrieve text notes from a specific author, and another to retrieve text notes with a specific hashtag. Events matching either filter will be returned.

## Filter for Specific Nostr Implementations

### NIP-05 Verification Filter

```json
{
  "kinds": [0],
  "authors": ["<pubkey1>"]
}
```

**Description**: A filter to retrieve the profile metadata for a specific user to check their NIP-05 verification status.

### NIP-51 Lists Filter

```json
{
  "kinds": [30000, 30001],
  "authors": ["<pubkey1>"]
}
```

**Description**: A filter to retrieve categorized lists (kind 30000) and bookmark lists (kind 30001) created by a specific user.

### NIP-58 Badge Definition Filter

```json
{
  "kinds": [30009],
  "authors": ["<pubkey1>"]
}
```

**Description**: A filter to retrieve badge definitions (kind 30009) created by a specific user.
