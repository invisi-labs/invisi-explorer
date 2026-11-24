package explorer

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/invisi-labs/invisi-explorer/internal/model"
)

type directoryCursor struct {
	Version  int    `json:"version"`
	Filter   string `json:"filter"`
	Revision string `json:"revision"`
	After    string `json:"after"`
}

func (e *Explorer) cachedDirectory(ctx context.Context, q query, id string) (any, error) {
	if q.kind != "" || q.token != "" || q.namespace != "" || q.history || q.onlyOrphans {
		return nil, publicError("INVALID_PARAMS")
	}
	if id != "" && (!publicAddress(id) || q.cursor != "") {
		return nil, publicError("INVALID_PARAMS")
	}
	q.hash = strings.ToLower(id)
	directories, err := e.options.BroadcasterDirectory(ctx)
	if err != nil {
		return nil, publicError("UNAVAILABLE")
	}
	entries := []model.BroadcasterEntry{}
	sources := []any{}
	revisionRows := []any{}
	complete := true
	for _, directory := range directories {
		if q.chain != "" && q.chain != directory.ChainID || q.protocol != "" && !strings.EqualFold(q.protocol, directory.ProtocolAddress) {
			continue
		}
		stale := directory.CheckedAt == 0 || time.Now().Unix()-directory.CheckedAt > 90
		if !directory.Complete || directory.Status == "unavailable" || stale {
			complete = false
		}
		sources = append(sources, map[string]any{"chainId": directory.ChainID, "protocolAddress": directory.ProtocolAddress, "registryAddress": directory.RegistryAddress, "registryCodeHash": directory.RegistryCodeHash, "anchor": directory.Anchor, "status": directory.Status, "checkedAt": directory.CheckedAt, "stale": stale, "complete": directory.Complete})
		for _, entry := range directory.Items {
			if id != "" && !strings.EqualFold(entry.Operator, id) {
				continue
			}
			entry.Observation.Stale = entry.Observation.CheckedAt == 0 || time.Now().Unix()-entry.Observation.CheckedAt > 90
			registryIdentity := entry.Broadcaster
			registryIdentity.ServiceHealth = ""
			entry.ServiceHealth = entry.Observation.Status
			if entry.Observation.Stale {
				entry.ServiceHealth = "stale"
			}
			entries = append(entries, entry)
			revisionRows = append(revisionRows, []any{entry.ChainID, entry.ProtocolAddress, registryIdentity})
		}
	}
	sort.Slice(entries, func(i, j int) bool { return directoryEntryKey(entries[i]) < directoryEntryKey(entries[j]) })
	// Registry identity governs pagination; runtime polling does not invalidate it.
	sort.Slice(revisionRows, func(i, j int) bool {
		a, _ := json.Marshal(revisionRows[i])
		b, _ := json.Marshal(revisionRows[j])
		return string(a) < string(b)
	})
	body, _ := json.Marshal(revisionRows)
	digest := sha256.Sum256(body)
	revision := hex.EncodeToString(digest[:])
	if id != "" {
		if len(entries) == 0 {
			if !complete {
				return nil, publicError("UNAVAILABLE")
			}
			return nil, publicError("NOT_FOUND")
		}
		if len(entries) == 1 {
			value := copyMap(entries[0])
			value["sources"] = sources
			value["directoryComplete"] = complete
			return value, nil
		}
		return map[string]any{"id": id, "operator": id, "occurrences": entries, "sources": sources, "directoryComplete": complete}, nil
	}
	after := ""
	if q.cursor != "" {
		c, err := e.decodeDirectoryCursor(q.cursor)
		if err != nil {
			return nil, err
		}
		if c.Filter != filterDigest("broadcasters", q) {
			return nil, publicError("INVALID_PARAMS")
		}
		if c.Revision != revision {
			return nil, publicError("CONFLICT")
		}
		after = c.After
	}
	selected := []model.BroadcasterEntry{}
	for _, entry := range entries {
		if directoryEntryKey(entry) > after {
			selected = append(selected, entry)
		}
	}
	result := map[string]any{"items": selected, "total": strconv.Itoa(len(entries)), "sources": sources, "directoryComplete": complete, "status": "cached", "totalBasis": "cached_registry"}
	if len(selected) > q.limit {
		selected = selected[:q.limit]
		result["items"] = selected
		c := directoryCursor{Version: 1, Filter: filterDigest("broadcasters", q), Revision: revision, After: directoryEntryKey(selected[len(selected)-1])}
		result["nextCursor"] = e.encodeDirectoryCursor(c)
	}
	return result, nil
}
func directoryEntryKey(entry model.BroadcasterEntry) string {
	return entry.ChainID + "/" + strings.ToLower(entry.ProtocolAddress) + "/" + strings.ToLower(entry.Operator)
}
func (e *Explorer) encodeDirectoryCursor(c directoryCursor) string {
	body, _ := json.Marshal(c)
	mac := hmac.New(sha256.New, e.options.CursorKey)
	mac.Write(body)
	return base64.RawURLEncoding.EncodeToString(body) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
func (e *Explorer) decodeDirectoryCursor(token string) (directoryCursor, error) {
	var c directoryCursor
	parts := strings.Split(token, ".")
	if len(parts) != 2 || len(token) > 3500 {
		return c, publicError("INVALID_PARAMS")
	}
	body, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return c, publicError("INVALID_PARAMS")
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return c, publicError("INVALID_PARAMS")
	}
	mac := hmac.New(sha256.New, e.options.CursorKey)
	mac.Write(body)
	if !hmac.Equal(sig, mac.Sum(nil)) || json.Unmarshal(body, &c) != nil || c.Version != 1 || c.After == "" {
		return c, publicError("INVALID_PARAMS")
	}
	return c, nil
}
