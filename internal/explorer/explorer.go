// Package explorer projects authenticated public chain facts without private inference.
package explorer

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/invisi-labs/invisi-explorer/internal/api"
	"github.com/invisi-labs/invisi-explorer/internal/model"
	p "github.com/invisi-labs/invisi-explorer/internal/wire"
)

type Options struct {
	PublicState     PublicState
	Dialect         string
	CursorKey       []byte
	Metadata        any
	Scopes          []p.Scope
	Recognition     func(context.Context, model.Operation) (any, error)
	NoteRecognition func(context.Context, string, string, p.Field) (any, error)
	Roots           func(context.Context) (any, error)
	Diagnostics     func(context.Context) (any, error)
}
type Explorer struct {
	db        *sql.DB
	options   Options
	instances []p.Scope
	ready     bool
}
type ChainSnapshot struct {
	ChainID         string `json:"chainId"`
	ProtocolAddress string `json:"protocolAddress"`
	BlockNumber     string `json:"blockNumber"`
	BlockHash       string `json:"blockHash"`
	CanonicalCount  int64  `json:"canonicalOperationCount"`
	PrefixReady     bool   `json:"prefixReady"`
}
type Snapshot struct {
	Chains                 []ChainSnapshot `json:"chains,omitempty"`
	RecognitionFingerprint string          `json:"recognitionFingerprint"`
	GenerationFingerprint  string          `json:"generationFingerprint"`
	AcquiredAt             string          `json:"acquiredAt"`
	AnchorFingerprint      string          `json:"canonicalFingerprint"`
	HeadVector             string          `json:"-"`
}
type Page struct {
	Items      []any    `json:"items"`
	Total      string   `json:"total"`
	NextCursor string   `json:"nextCursor,omitempty"`
	Snapshot   Snapshot `json:"snapshot"`
	Unit       string   `json:"unit,omitempty"`
}
type position struct {
	Time  string `json:"t"`
	ID    string `json:"i"`
	Index int64  `json:"x"`
}
type cursor struct {
	Version  int      `json:"v"`
	Route    string   `json:"r"`
	Filter   string   `json:"f"`
	Snapshot Snapshot `json:"s"`
	After    position `json:"a"`
	Heads    string   `json:"h"`
}
type query struct {
	chain, protocol, kind, token, namespace, hash, address string
	limit                                                  int
	cursor                                                 string
	history, onlyOrphans                                   bool
}

func publicError(code string) error { return &api.Error{Code: code} }
func New(db *sql.DB, options Options) *Explorer {
	e := &Explorer{db: db, options: options, ready: db != nil && len(options.CursorKey) >= 32 && (options.Dialect == "sqlite" || options.Dialect == "postgres")}
	e.options.CursorKey = append([]byte(nil), options.CursorKey...)
	e.options.Scopes = append([]p.Scope(nil), options.Scopes...)
	seen := map[string]bool{}
	for _, s := range options.Scopes {
		if s.Validate() != nil {
			e.ready = false
			continue
		}
		key := s.ChainID + "/" + strings.ToLower(s.ProtocolAddress)
		if !seen[key] {
			seen[key] = true
			e.instances = append(e.instances, s)
		}
	}
	sort.Slice(e.instances, func(i, j int) bool { return e.instances[i].Key() < e.instances[j].Key() })

	return e
}
func (e *Explorer) sql(query string) string {
	if e.options.Dialect != "postgres" {
		return query
	}
	var b strings.Builder
	n := 0
	for _, c := range query {
		if c == '?' {
			n++
			fmt.Fprintf(&b, "$%d", n)
		} else {
			b.WriteRune(c)
		}
	}
	return b.String()
}
func pad(value string) string {
	n, err := p.Uint64(value)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%020d", n)
}
func hashHex(s string, n int) bool { _, err := p.DecodeHex(s, n); return err == nil }
func parseQuery(values url.Values) (query, error) {
	q := query{limit: 20}
	allowed := map[string]bool{"chainId": true, "protocolAddress": true, "kind": true, "tokenAddress": true, "namespace": true, "type": true, "limit": true, "cursor": true}
	for key, v := range values {
		if !allowed[key] || len(v) != 1 || len(v[0]) > 4096 {
			return q, publicError("INVALID_PARAMS")
		}
	}
	q.chain = values.Get("chainId")
	q.protocol = strings.ToLower(values.Get("protocolAddress"))
	q.token = strings.ToLower(values.Get("tokenAddress"))
	q.kind = values.Get("kind")
	q.namespace = values.Get("namespace")
	if q.namespace == "" {
		q.namespace = values.Get("type")
	}
	q.cursor = values.Get("cursor")
	if q.chain != "" {
		if _, err := p.Uint64(q.chain); err != nil {
			return q, publicError("INVALID_PARAMS")
		}
	}
	for _, a := range []string{q.protocol, q.token} {
		if a != "" && !hashHex(a, 20) {
			return q, publicError("INVALID_PARAMS")
		}
	}
	if text := values.Get("limit"); text != "" {
		n, err := strconv.Atoi(text)
		if err != nil || n < 1 || n > 200 || strconv.Itoa(n) != text {
			return q, publicError("INVALID_PARAMS")
		}
		q.limit = n
	}
	if q.kind != "" {
		valid := false
		for _, k := range []string{"shield", "transact", "creditDraw", "createCredit", "updateCredit", "revokeCredit", "unsupported"} {
			valid = valid || q.kind == k
		}
		if !valid {
			return q, publicError("INVALID_PARAMS")
		}
	}
	if len(q.namespace) > 32 {
		return q, publicError("INVALID_PARAMS")
	}
	return q, nil
}
func (e *Explorer) filters(q query) (string, []any) {
	var clauses []string
	var args []any
	if q.chain != "" {
		clauses = append(clauses, "o.chain_id=?")
		args = append(args, q.chain)
	}
	if q.protocol != "" {
		clauses = append(clauses, "LOWER(o.protocol_address)=?")
		args = append(args, q.protocol)
	}
	if q.kind != "" {
		clauses = append(clauses, e.extract("o.payload", "kind")+"=?")
		args = append(args, q.kind)
	}
	if q.token != "" {
		clauses = append(clauses, "LOWER("+e.extract("o.payload", "tokenAddress")+")=?")
		args = append(args, q.token)
	}
	if len(clauses) == 0 {
		return "1=1", nil
	}
	return strings.Join(clauses, " AND "), args
}
func (e *Explorer) extract(column, key string) string {
	if e.options.Dialect == "postgres" {
		return "(" + column + "::jsonb->>'" + key + "')"
	}
	return "json_extract(" + column + ",'$." + key + "')"
}
func (e *Explorer) timeSQL() string {
	x := e.extract("o.payload", "timestamp")
	if e.options.Dialect == "postgres" {
		return "lpad(" + x + ",20,'0')"
	}
	return "substr('00000000000000000000'||" + x + ",-20)"
}
func (e *Explorer) fingerprint(ctx context.Context) (string, error) {
	states := []string{}
	scopes := append([]p.Scope(nil), e.options.Scopes...)
	sort.Slice(scopes, func(i, j int) bool { return scopes[i].Key() < scopes[j].Key() })
	for _, scope := range scopes {
		var generation, published string
		err := e.db.QueryRowContext(ctx, e.sql("SELECT generation,published FROM scopes WHERE id=?"), scope.Key()).Scan(&generation, &published)
		if errors.Is(err, sql.ErrNoRows) {
			generation = pad("1")
			published = pad("0")
		} else if err != nil {
			return "", publicError("UNAVAILABLE")
		}
		var revision int64
		_ = e.db.QueryRowContext(ctx, "SELECT version FROM readmodel_versions WHERE id=?", scope.Key()).Scan(&revision)
		states = append(states, scope.Key()+"/"+generation+"/"+published+"/"+strconv.FormatInt(revision, 10))
	}
	b, _ := json.Marshal(states)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:]), nil
}
func (e *Explorer) snapshot(ctx context.Context, q query) (Snapshot, error) {
	return e.snapshotAt(ctx, q)
}
func (e *Explorer) snapshotAt(ctx context.Context, q query, vectors ...string) (Snapshot, error) {
	s := Snapshot{Chains: []ChainSnapshot{}, AcquiredAt: strconv.FormatInt(time.Now().Unix(), 10)}
	var err error
	s.RecognitionFingerprint, err = e.fingerprint(ctx)
	if err != nil {
		return s, err
	}
	s.GenerationFingerprint, err = e.generationFingerprint(ctx)
	if err != nil {
		return s, err
	}
	selected := []p.Scope{}
	for _, instance := range e.instances {
		if q.chain != "" && q.chain != instance.ChainID || q.protocol != "" && q.protocol != strings.ToLower(instance.ProtocolAddress) {
			continue
		}
		selected = append(selected, instance)
	}
	var heads []uint64
	var ready []bool
	if len(vectors) > 0 {
		data, decodeErr := base64.RawURLEncoding.DecodeString(vectors[0])
		if decodeErr != nil || len(data) > 65536 {
			return s, publicError("INVALID_PARAMS")
		}
		for range selected {
			if len(data) == 0 || data[0] > 1 {
				return s, publicError("INVALID_PARAMS")
			}
			flag := data[0] == 1
			data = data[1:]
			head := uint64(0)
			if flag {
				var n int
				head, n = binary.Uvarint(data)
				if n <= 0 {
					return s, publicError("INVALID_PARAMS")
				}
				data = data[n:]
			}
			ready = append(ready, flag)
			heads = append(heads, head)
		}
		if len(data) != 0 {
			return s, publicError("INVALID_PARAMS")
		}
	}
	for i, instance := range selected {
		chain := ChainSnapshot{ChainID: instance.ChainID, ProtocolAddress: strings.ToLower(instance.ProtocolAddress), BlockNumber: "0", BlockHash: p.ZeroHash}
		key := chain.ChainID + "/" + chain.ProtocolAddress
		var currentNumber, currentHash string
		err = e.db.QueryRowContext(ctx, e.sql("SELECT block_number,block_hash FROM chain_checkpoints WHERE id=?"), key).Scan(&currentNumber, &currentHash)
		exists := err == nil
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return s, publicError("UNAVAILABLE")
		}
		if len(vectors) > 0 && !ready[i] {
			s.Chains = append(s.Chains, chain)
			continue
		}
		if !exists {
			if len(vectors) > 0 && ready[i] {
				return s, publicError("CHAIN_REORGED")
			}
			s.Chains = append(s.Chains, chain)
			continue
		}
		current, parseErr := strconv.ParseUint(currentNumber, 10, 64)
		if parseErr != nil || !hashHex(currentHash, 32) {
			return s, publicError("UNAVAILABLE")
		}
		head := current
		anchorHash := currentHash
		if len(vectors) > 0 {
			head = heads[i]
			if current < head {
				return s, publicError("CHAIN_REORGED")
			}
			if current != head {
				err = e.db.QueryRowContext(ctx, e.sql("SELECT block_hash FROM chain_checkpoints WHERE id=? AND block_number=?"), key+"/block/"+fmt.Sprintf("%020d", head), fmt.Sprintf("%020d", head)).Scan(&anchorHash)
				if err != nil {
					return s, publicError("CHAIN_REORGED")
				}
			}
		}
		chain.PrefixReady = true
		chain.BlockNumber = strconv.FormatUint(head, 10)
		chain.BlockHash = anchorHash
		if err = e.db.QueryRowContext(ctx, e.sql("SELECT COUNT(*) FROM operations o WHERE o.chain_id=? AND LOWER(o.protocol_address)=? AND o.canonical=1 AND o.block_number<=?"), chain.ChainID, chain.ProtocolAddress, pad(chain.BlockNumber)).Scan(&chain.CanonicalCount); err != nil {
			return s, publicError("UNAVAILABLE")
		}
		s.Chains = append(s.Chains, chain)
	}
	for _, chain := range s.Chains {
		if !chain.PrefixReady {
			return s, publicError("INDEX_PENDING")
		}
	}
	var headBytes []byte
	for _, chain := range s.Chains {
		if !chain.PrefixReady {
			headBytes = append(headBytes, 0)
		} else {
			headBytes = append(headBytes, 1)
			headBytes = binary.AppendUvarint(headBytes, parseNumber(chain.BlockNumber))
		}
	}
	s.HeadVector = base64.RawURLEncoding.EncodeToString(headBytes)
	body, _ := json.Marshal(s.Chains)
	hash := sha256.Sum256(body)
	s.AnchorFingerprint = hex.EncodeToString(hash[:])
	return s, nil
}
func parseNumber(s string) uint64 { n, _ := strconv.ParseUint(s, 10, 64); return n }
func (e *Explorer) validateSnapshot(ctx context.Context, s Snapshot) error {
	fingerprint, err := e.fingerprint(ctx)
	if err != nil {
		return err
	}
	generation, err := e.generationFingerprint(ctx)
	if err != nil {
		return err
	}
	if generation != s.GenerationFingerprint {
		return publicError("CHAIN_REORGED")
	}
	if fingerprint != s.RecognitionFingerprint {
		return publicError("CONFLICT")
	}
	for _, a := range s.Chains {
		if !a.PrefixReady {
			continue
		}
		key := a.ChainID + "/" + a.ProtocolAddress
		var currentNumber, currentHash string
		if err = e.db.QueryRowContext(ctx, e.sql("SELECT block_number,block_hash FROM chain_checkpoints WHERE id=?"), key).Scan(&currentNumber, &currentHash); err != nil {
			return publicError("CHAIN_REORGED")
		}
		current := parseNumber(currentNumber)
		head := parseNumber(a.BlockNumber)
		if current < head {
			return publicError("CHAIN_REORGED")
		}
		anchor := currentHash
		if current != head {
			if err = e.db.QueryRowContext(ctx, e.sql("SELECT block_hash FROM chain_checkpoints WHERE id=? AND block_number=?"), key+"/block/"+pad(a.BlockNumber), pad(a.BlockNumber)).Scan(&anchor); err != nil {
				return publicError("CHAIN_REORGED")
			}
		}
		if anchor != a.BlockHash {
			return publicError("CHAIN_REORGED")
		}
		var count int64
		if err = e.db.QueryRowContext(ctx, e.sql("SELECT COUNT(*) FROM operations WHERE chain_id=? AND LOWER(protocol_address)=? AND canonical=1 AND block_number<=?"), a.ChainID, a.ProtocolAddress, pad(a.BlockNumber)).Scan(&count); err != nil {
			return publicError("UNAVAILABLE")
		}
		if count != a.CanonicalCount {
			return publicError("CHAIN_REORGED")
		}
	}

	return nil
}
func snapshotWhere(s Snapshot) (string, []any) {
	clauses := []string{}
	args := []any{}
	for _, a := range s.Chains {
		if !a.PrefixReady {
			continue
		}
		clauses = append(clauses, "(o.chain_id=? AND LOWER(o.protocol_address)=? AND o.block_number<=?)")
		args = append(args, a.ChainID, a.ProtocolAddress, pad(a.BlockNumber))
	}
	if len(clauses) == 0 {
		return "0=1", nil
	}
	return "(" + strings.Join(clauses, " OR ") + ")", args
}
func filterHash(route string, q query) string {
	body, _ := json.Marshal([]any{route, q.chain, q.protocol, q.kind, q.token, q.namespace, q.hash, q.address, q.limit, q.history, q.onlyOrphans})
	h := sha256.Sum256(body)
	return hex.EncodeToString(h[:])
}
func (e *Explorer) cursorEncode(c cursor) (string, error) {
	c.Heads = c.Snapshot.HeadVector
	c.Snapshot.Chains = nil
	body, _ := json.Marshal(c)
	mac := hmac.New(sha256.New, e.options.CursorKey)
	mac.Write(body)
	token := base64.RawURLEncoding.EncodeToString(body) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if len(token) > 3500 {
		return "", publicError("UNSUPPORTED")
	}
	return token, nil
}
func (e *Explorer) pageContext(ctx context.Context, route string, q query) (Snapshot, *position, error) {
	if q.cursor == "" {
		s, err := e.snapshot(ctx, q)
		return s, nil, err
	}
	parts := strings.Split(q.cursor, ".")
	if len(parts) != 2 || len(q.cursor) > 3500 {
		return Snapshot{}, nil, publicError("INVALID_PARAMS")
	}
	body, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return Snapshot{}, nil, publicError("INVALID_PARAMS")
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return Snapshot{}, nil, publicError("INVALID_PARAMS")
	}
	mac := hmac.New(sha256.New, e.options.CursorKey)
	mac.Write(body)
	if !hmac.Equal(signature, mac.Sum(nil)) {
		return Snapshot{}, nil, publicError("INVALID_PARAMS")
	}
	var c cursor
	if json.Unmarshal(body, &c) != nil || c.Version != 1 || c.Route != route || c.Filter != filterHash(route, q) {
		return Snapshot{}, nil, publicError("INVALID_PARAMS")
	}
	restored, err := e.snapshotAt(ctx, q, c.Heads)
	if err != nil {
		var pending *api.Error
		if errors.As(err, &pending) && pending.Code == "INDEX_PENDING" {
			return Snapshot{}, nil, publicError("CHAIN_REORGED")
		}
		return Snapshot{}, nil, err
	}
	if restored.GenerationFingerprint != c.Snapshot.GenerationFingerprint {
		return Snapshot{}, nil, publicError("CHAIN_REORGED")
	}
	if restored.RecognitionFingerprint != c.Snapshot.RecognitionFingerprint {
		return Snapshot{}, nil, publicError("CONFLICT")
	}
	if restored.AnchorFingerprint != c.Snapshot.AnchorFingerprint {
		return Snapshot{}, nil, publicError("CHAIN_REORGED")
	}
	restored.AcquiredAt = c.Snapshot.AcquiredAt
	if err = e.validateSnapshot(ctx, restored); err != nil {
		return Snapshot{}, nil, err
	}
	return restored, &c.After, nil
}
func (e *Explorer) Public(ctx context.Context, pathSuffix string, values url.Values, body json.RawMessage) (any, error) {
	if pathSuffix == "/metadata" {
		if len(values) != 0 || len(body) != 0 {
			return nil, publicError("INVALID_PARAMS")
		}
		return e.options.Metadata, nil
	}
	if !e.ready {
		return nil, publicError("UNAVAILABLE")
	}
	if pathSuffix == "/search" {
		return e.search(ctx, values, body)
	}
	q, err := parseQuery(values)
	if err != nil {
		return nil, err
	}
	if len(body) > 0 {
		return nil, publicError("INVALID_PARAMS")
	}
	parts := strings.Split(strings.Trim(pathSuffix, "/"), "/")
	if len(parts) > 3 {
		return nil, publicError("NOT_FOUND")
	}
	route := parts[0]
	if len(parts) == 3 && !((route == "notes" || route == "credit-states" || route == "commitments") && parts[2] == "occurrences") {
		return nil, publicError("NOT_FOUND")
	}
	if q.namespace != "" {
		allowed := false
		switch route {
		case "notes":
			allowed = q.namespace == "note"
		case "credit-states":
			allowed = q.namespace == "credit" || q.namespace == "creditState"
		case "nullifiers":
			allowed = q.namespace == "note" || q.namespace == "credit"
		case "commitments":
			allowed = q.namespace == "note" || q.namespace == "credit" || q.namespace == "creditState"
		case "trees":
			for _, v := range []string{"protocol", "recognition"} {
				allowed = allowed || q.namespace == v
			}
		}
		if !allowed {
			return nil, publicError("INVALID_PARAMS")
		}
	}
	id := ""
	if len(parts) > 1 {
		id = parts[1]
	}
	switch route {
	case "overview":
		if q.cursor != "" {
			return nil, publicError("INVALID_PARAMS")
		}
		return e.overview(ctx, q)
	case "transactions":
		if id != "" {
			return e.transaction(ctx, q, id)
		}
		return e.list(ctx, route, q)
	case "notes", "credit-states", "nullifiers":
		if id != "" {
			return e.object(ctx, route, q, id, len(parts) == 3 && parts[2] == "occurrences")
		}
		return e.list(ctx, route, q)
	case "commitments":
		if id != "" {
			return e.commitment(ctx, q, id)
		}
		return e.list(ctx, "commitments", q)
	case "grants":
		if id != "" {
			return e.grant(ctx, q, id)
		}
		return e.list(ctx, route, q)
	case "addresses":
		if id != "" {
			return e.address(ctx, q, id)
		}
		return nil, publicError("NOT_FOUND")
	case "trees":
		return e.trees(ctx, q, id)
	case "roots":
		return e.roots(ctx, q, id)
	case "diagnostics":
		return callback(e.options.Diagnostics, ctx, id)
	case "broadcasters":
		return e.directory(ctx, q, id)
	default:
		return nil, publicError("NOT_FOUND")
	}
}
func callback(fn func(context.Context) (any, error), ctx context.Context, id string) (any, error) {
	if fn == nil {
		return nil, publicError("UNAVAILABLE")
	}
	v, err := fn(ctx)
	if err != nil {
		return nil, publicError("UNAVAILABLE")
	}
	if id == "" {
		return v, nil
	}
	body, _ := json.Marshal(v)
	var list []map[string]any
	if json.Unmarshal(body, &list) != nil {
		var page struct {
			Items []map[string]any `json:"items"`
		}
		if json.Unmarshal(body, &page) == nil {
			list = page.Items
		}
	}
	for _, item := range list {
		for _, key := range []string{"id", "rootId"} {
			if item[key] == id {
				return item, nil
			}
		}
	}
	return nil, publicError("NOT_FOUND")
}

func instanceWhere(s Snapshot) (string, []any) {
	clauses := []string{}
	args := []any{}
	for _, a := range s.Chains {
		clauses = append(clauses, "(o.chain_id=? AND LOWER(o.protocol_address)=?)")
		args = append(args, a.ChainID, a.ProtocolAddress)
	}
	if len(clauses) == 0 {
		return "0=1", nil
	}
	return "(" + strings.Join(clauses, " OR ") + ")", args
}

func (e *Explorer) generationFingerprint(ctx context.Context) (string, error) {
	states := []string{}
	scopes := append([]p.Scope(nil), e.options.Scopes...)
	sort.Slice(scopes, func(i, j int) bool { return scopes[i].Key() < scopes[j].Key() })
	for _, scope := range scopes {
		var generation string
		err := e.db.QueryRowContext(ctx, e.sql("SELECT generation FROM chain_generations WHERE id=?"), scope.Key()).Scan(&generation)
		if errors.Is(err, sql.ErrNoRows) {
			generation = pad("1")
		} else if err != nil {
			return "", publicError("UNAVAILABLE")
		}
		states = append(states, scope.Key()+"/"+generation)
	}
	b, _ := json.Marshal(states)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:]), nil
}

func (e *Explorer) successSQL() string {
	if e.options.Dialect == "postgres" {
		return e.extract("o.payload", "success") + "='true'"
	}
	return e.extract("o.payload", "success") + "=1"
}
