package tidal

import (
	"encoding/json"
	"fmt"
	"net/url"
)

// Resource represents a JSON:API resource object.
type Resource struct {
	Type          string            `json:"type"`
	ID            string            `json:"id"`
	Attributes    json.RawMessage   `json:"attributes"`
	Relationships map[string]json.RawMessage `json:"relationships,omitempty"`
}

// ResourceIdentifier is a JSON:API resource linkage (type + id only).
type ResourceIdentifier struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

// Document is a JSON:API top-level document.
type Document struct {
	Data     json.RawMessage `json:"data"`
	Included []Resource      `json:"included,omitempty"`
	Links    *Links          `json:"links,omitempty"`
}

// Links represents JSON:API pagination links.
type Links struct {
	Self string     `json:"self"`
	Next string     `json:"next,omitempty"`
	Meta *LinksMeta `json:"meta,omitempty"`
}

// LinksMeta holds metadata from the links object.
type LinksMeta struct {
	Total int `json:"total,omitempty"`
}

// RelationshipData holds the "data" field of a relationship.
type RelationshipData struct {
	Data json.RawMessage `json:"data"`
}

// ParseOne parses a single-resource JSON:API response body.
// It unmarshals the resource's attributes into T and sets the ID field.
func ParseOne[T any](body []byte, setID func(*T, string)) (*T, []Resource, error) {
	var doc Document
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, nil, fmt.Errorf("parse document: %w", err)
	}

	var res Resource
	if err := json.Unmarshal(doc.Data, &res); err != nil {
		return nil, nil, fmt.Errorf("parse resource: %w", err)
	}

	var item T
	if res.Attributes != nil {
		if err := json.Unmarshal(res.Attributes, &item); err != nil {
			return nil, nil, fmt.Errorf("parse attributes: %w", err)
		}
	}
	setID(&item, res.ID)

	return &item, doc.Included, nil
}

// ParseMany parses a multi-resource JSON:API response body.
// Returns the items, the next cursor (empty string if no next page), and any error.
func ParseMany[T any](body []byte, setID func(*T, string)) ([]T, []Resource, string, error) {
	var doc Document
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, nil, "", fmt.Errorf("parse document: %w", err)
	}

	var resources []Resource
	if err := json.Unmarshal(doc.Data, &resources); err != nil {
		return nil, nil, "", fmt.Errorf("parse resources array: %w", err)
	}

	items := make([]T, 0, len(resources))
	for _, res := range resources {
		var item T
		if res.Attributes != nil {
			if err := json.Unmarshal(res.Attributes, &item); err != nil {
				return nil, nil, "", fmt.Errorf("parse attributes for %s/%s: %w", res.Type, res.ID, err)
			}
		}
		setID(&item, res.ID)
		items = append(items, item)
	}

	nextCursor := extractCursor(doc.Links)
	return items, doc.Included, nextCursor, nil
}

// ParseRelationship parses a relationship response that contains resource identifiers.
// This is used for endpoints like /userCollections/{id}/relationships/albums
// which return included resources with attributes, plus data as identifiers.
func ParseRelationship[T any](body []byte, setID func(*T, string)) ([]T, string, error) {
	var doc Document
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, "", fmt.Errorf("parse document: %w", err)
	}

	// Parse the data array as resource identifiers
	var identifiers []ResourceIdentifier
	if doc.Data != nil {
		if err := json.Unmarshal(doc.Data, &identifiers); err != nil {
			return nil, "", fmt.Errorf("parse identifiers: %w", err)
		}
	}

	// Build a map of included resources by type+id
	includedMap := make(map[string]Resource, len(doc.Included))
	for _, inc := range doc.Included {
		key := inc.Type + "/" + inc.ID
		includedMap[key] = inc
	}

	items := make([]T, 0, len(identifiers))
	for _, ident := range identifiers {
		key := ident.Type + "/" + ident.ID
		res, ok := includedMap[key]
		if !ok {
			// No included data; create item with just the ID
			var item T
			setID(&item, ident.ID)
			items = append(items, item)
			continue
		}

		var item T
		if res.Attributes != nil {
			if err := json.Unmarshal(res.Attributes, &item); err != nil {
				return nil, "", fmt.Errorf("parse included attributes for %s: %w", key, err)
			}
		}
		setID(&item, ident.ID)
		items = append(items, item)
	}

	nextCursor := extractCursor(doc.Links)
	return items, nextCursor, nil
}

// extractCursor extracts the cursor value from the links.next URL.
func extractCursor(links *Links) string {
	if links == nil || links.Next == "" {
		return ""
	}
	u, err := url.Parse(links.Next)
	if err != nil {
		return ""
	}
	return u.Query().Get("page[cursor]")
}

// ResourcePayload builds a JSON:API resource linkage payload for POST/DELETE.
// Example: {"data": [{"type": "albums", "id": "12345"}]}
func ResourcePayload(resourceType string, ids []string) map[string]any {
	data := make([]map[string]string, len(ids))
	for i, id := range ids {
		data[i] = map[string]string{
			"type": resourceType,
			"id":   id,
		}
	}
	return map[string]any{"data": data}
}

// CreateResourcePayload builds a JSON:API resource creation payload.
// Example: {"data": {"type": "playlists", "attributes": {...}}}
func CreateResourcePayload(resourceType string, attributes any) map[string]any {
	return map[string]any{
		"data": map[string]any{
			"type":       resourceType,
			"attributes": attributes,
		},
	}
}

// UpdateResourcePayload builds a JSON:API resource update (PATCH) payload.
// Example: {"data": {"type": "playlists", "id": "xxx", "attributes": {...}}}
func UpdateResourcePayload(resourceType, id string, attributes any) map[string]any {
	return map[string]any{
		"data": map[string]any{
			"type":       resourceType,
			"id":         id,
			"attributes": attributes,
		},
	}
}
