package upstream

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/invisi-labs/invisi-explorer/internal/config"
	"github.com/invisi-labs/invisi-explorer/internal/model"
	"github.com/invisi-labs/invisi-explorer/internal/readmodel"
)

type onlineProfileStats struct {
	Requests     int64 `json:"requests"`
	LogicalCalls int64 `json:"logicalCalls"`
}

func onlineStats(t *testing.T, root string) onlineProfileStats {
	t.Helper()
	r, e := http.Get(root + "/profile/stats")
	if e != nil {
		t.Fatal(e)
	}
	defer r.Body.Close()
	var result onlineProfileStats
	if e = json.NewDecoder(r.Body).Decode(&result); e != nil {
		t.Fatal(e)
	}
	return result
}

// This opt-in comparison runs both production sync paths against fixture APIs
// backed by identical real indexed batches and a real native node. Read models
// are disposable, and always compared for exact canonical operation payloads.
func TestOnlineCatchUpComparison(t *testing.T) {
	root := os.Getenv("INVISI_SYNC_PROFILE_BASELINE")
	if root == "" {
		t.Skip("opt-in actual-data catch-up comparison")
	}
	candidate := os.Getenv("INVISI_SYNC_PROFILE_CANDIDATE")
	if candidate == "" {
		t.Fatal("candidate fixture API required")
	}
	from, e := strconv.ParseUint(os.Getenv("INVISI_SYNC_PROFILE_FROM"), 10, 64)
	if e != nil {
		t.Fatal(e)
	}
	count, e := strconv.ParseUint(os.Getenv("INVISI_SYNC_PROFILE_BLOCKS"), 10, 64)
	if e != nil {
		t.Fatal(e)
	}
	target := from + count - 1
	report := []map[string]any{}
	wantDigest := ""
	for round := 0; round < 3; round++ {
		cases := []struct {
			name, url string
			prefetch  bool
		}{{"baseline", root, false}, {"rpc-only", candidate, false}, {"explorer-only", root, true}, {"combined", candidate, true}}
		if round%2 != 0 {
			cases[0], cases[3] = cases[3], cases[0]
			cases[1], cases[2] = cases[2], cases[1]
		}
		for _, test := range cases {
			func() {
				ctx, stop := context.WithTimeout(context.Background(), 3*time.Minute)
				defer stop()
				store, e := readmodel.Open(filepath.Join(t.TempDir(), "explorer.sqlite"))
				if e != nil {
					t.Fatal(e)
				}
				defer store.Close()
				syncer := &Syncer{Store: store, Client: Client()}
				var deployment struct {
					Deployment struct {
						ChainID  string `json:"chainId"`
						Protocol struct {
							Address         string `json:"address"`
							RuntimeCodeHash string `json:"runtimeCodeHash"`
							FromBlock       uint64 `json:"fromBlock"`
						} `json:"protocol"`
					} `json:"deployment"`
				}
				endpoint := test.url + "/v1/chains/97"
				if e = Call(ctx, syncer.Client, endpoint, "invisi_getDeployment", []any{}, &deployment); e != nil {
					t.Fatal(e)
				}
				c := config.Chain{ChainID: deployment.Deployment.ChainID, ProtocolAddress: deployment.Deployment.Protocol.Address, CodeHash: deployment.Deployment.Protocol.RuntimeCodeHash, StartBlock: deployment.Deployment.Protocol.FromBlock, RPCURL: endpoint}
				hash, e := syncer.Header(ctx, c, from-1)
				if e != nil {
					t.Fatal(e)
				}
				if e = store.Stage(ctx, Scope(c), nil, "", strconv.FormatUint(from-1, 10)); e != nil {
					t.Fatal(e)
				}
				if e = store.Publish(ctx, Scope(c), model.Anchor{BlockNumber: strconv.FormatUint(from-1, 10), BlockHash: hash}); e != nil {
					t.Fatal(e)
				}
				before := onlineStats(t, test.url)
				start := time.Now()
				for checkpointNumber(t, syncer, c) < target {
					if test.prefetch {
						e = syncer.CatchUp(ctx, c)
					} else {
						e = syncer.Tick(ctx, c)
					}
					if e != nil {
						t.Fatal(test.name, e)
					}
				}
				elapsed := time.Since(start)
				after := onlineStats(t, test.url)
				rows, e := store.DB.QueryContext(ctx, "SELECT payload FROM operations WHERE canonical=1 ORDER BY id")
				if e != nil {
					t.Fatal(e)
				}
				digest := sha256.New()
				operations := 0
				for rows.Next() {
					var payload string
					rows.Scan(&payload)
					digest.Write([]byte(payload))
					operations++
				}
				if e = rows.Err(); e != nil {
					t.Fatal(e)
				}
				rows.Close()
				value := hex.EncodeToString(digest.Sum(nil))
				if wantDigest == "" {
					wantDigest = value
				} else if value != wantDigest {
					t.Fatal("actual transaction facts differ across sync paths")
				}
				if os.Getenv("INVISI_SYNC_PROFILE_REQUIRE_TRANSACTIONS") == "1" && operations == 0 {
					t.Fatal("sample omitted expected real transactions")
				}
				sample := map[string]any{"round": round + 1, "mode": test.name, "fromBlock": from, "toBlock": target, "blocks": count, "elapsedMs": elapsed.Milliseconds(), "blocksPerSecond": float64(count) / elapsed.Seconds(), "nativeRequests": after.Requests - before.Requests, "nativeLogicalCalls": after.LogicalCalls - before.LogicalCalls, "operations": operations, "operationPayloadSHA256": value}
				report = append(report, sample)
				raw, _ := json.Marshal(sample)
				fmt.Println(string(raw))
			}()
		}
	}
	if name := os.Getenv("INVISI_SYNC_PROFILE_REPORT"); name != "" {
		raw, _ := json.MarshalIndent(report, "", "  ")
		if e = os.WriteFile(name, append(raw, '\n'), 0600); e != nil {
			t.Fatal(e)
		}
	}
}
