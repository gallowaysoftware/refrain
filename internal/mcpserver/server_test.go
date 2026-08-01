package mcpserver

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/gallowaysoftware/refrain/internal/store"
)

func TestDigestHandler(t *testing.T) {
	st := store.New(t.TempDir())
	if err := st.EnsureRepo(); err != nil {
		t.Fatalf("EnsureRepo: %v", err)
	}
	if err := st.CreateExpert("x4", "Terran playthrough"); err != nil {
		t.Fatalf("CreateExpert: %v", err)
	}
	h := DigestHandler(Deps{Store: st, Version: "test"})

	cases := []struct {
		name       string
		url        string
		wantStatus int
		wantBody   string
	}{
		{"missing expert param", "/digest", 400, ""},
		{"unknown expert", "/digest?expert=nope", 404, ""},
		{"known expert", "/digest?expert=x4", 200, "# Memory digest: x4"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest("GET", tc.url, nil))
			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			if tc.wantBody != "" && !strings.Contains(rec.Body.String(), tc.wantBody) {
				t.Errorf("body missing %q:\n%s", tc.wantBody, rec.Body.String())
			}
		})
	}
}

// TestStateToolsRoundTrip drives set_state/get_state through the real
// Streamable HTTP transport with an in-process client — the wire shape
// (argument names, structured output) IS the contract etude depends on.
func TestStateToolsRoundTrip(t *testing.T) {
	st := store.New(t.TempDir())
	if err := st.EnsureRepo(); err != nil {
		t.Fatalf("EnsureRepo: %v", err)
	}
	if err := st.CreateExpert("x4", "Terran playthrough"); err != nil {
		t.Fatalf("CreateExpert: %v", err)
	}
	srv := httptest.NewServer(Handler(Deps{Store: st, Version: "test"}))
	defer srv.Close()

	ctx := context.Background()
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v0"}, nil)
	cs, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: srv.URL}, nil)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer cs.Close()

	mastery := map[string]any{
		"course_slug": "x4-econ",
		"coverage":    map[string]any{"questions": 12, "mastered": 3},
		"due":         4,
	}
	set, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "set_state",
		Arguments: map[string]any{"expert": "x4", "key": "mastery", "value": mastery},
	})
	if err != nil {
		t.Fatalf("set_state call: %v", err)
	}
	if set.IsError {
		t.Fatalf("set_state failed: %+v", set.Content)
	}

	get, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "get_state",
		Arguments: map[string]any{"expert": "x4", "key": "mastery"},
	})
	if err != nil {
		t.Fatalf("get_state call: %v", err)
	}
	if get.IsError {
		t.Fatalf("get_state failed: %+v", get.Content)
	}
	out, err := json.Marshal(get.StructuredContent)
	if err != nil {
		t.Fatalf("marshal structured content: %v", err)
	}
	for _, want := range []string{`"key":"mastery"`, `"course_slug":"x4-econ"`, `"due":4`} {
		if !strings.Contains(string(out), want) {
			t.Errorf("get_state output missing %s: %s", want, out)
		}
	}

	// A never-set key is a tool error, not a protocol failure.
	missing, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "get_state",
		Arguments: map[string]any{"expert": "x4", "key": "nope"},
	})
	if err != nil {
		t.Fatalf("get_state missing call: %v", err)
	}
	if !missing.IsError {
		t.Error("get_state on unset key did not report an error")
	}
}

func TestMain(m *testing.M) {
	if _, err := exec.LookPath("git"); err != nil {
		os.Exit(0)
	}
	os.Exit(m.Run())
}
