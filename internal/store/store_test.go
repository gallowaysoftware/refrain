package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// newTestStore returns a store on a real git repo in a temp dir — the
// git interaction IS the logic here, so no fake.
func newTestStore(t *testing.T) *Store {
	t.Helper()
	s := New(t.TempDir())
	if err := s.EnsureRepo(); err != nil {
		t.Fatalf("EnsureRepo: %v", err)
	}
	return s
}

func commitCount(t *testing.T, s *Store) int {
	t.Helper()
	out, err := s.git("rev-list", "--count", "HEAD")
	if err != nil {
		t.Fatalf("rev-list: %v", err)
	}
	n := 0
	for _, r := range strings.TrimSpace(out) {
		n = n*10 + int(r-'0')
	}
	return n
}

func TestEnsureRepoIdempotent(t *testing.T) {
	s := newTestStore(t)
	if err := s.EnsureRepo(); err != nil {
		t.Fatalf("second EnsureRepo: %v", err)
	}
	if got := commitCount(t, s); got != 1 {
		t.Errorf("commits after double init = %d, want 1", got)
	}
}

func TestCreateExpertAndScaffold(t *testing.T) {
	s := newTestStore(t)
	if err := s.CreateExpert("x4", "X4 Foundations campaign companion"); err != nil {
		t.Fatalf("CreateExpert: %v", err)
	}
	if err := s.CreateExpert("x4", "again"); err == nil {
		t.Fatal("duplicate CreateExpert did not error")
	}

	files, err := s.ListFiles("x4")
	if err != nil {
		t.Fatalf("ListFiles: %v", err)
	}
	want := []string{"profile.md", "goals.md", "decisions.md", "log.md"}
	if len(files) != len(want) {
		t.Fatalf("files = %v, want %v", files, want)
	}

	experts, err := s.ListExperts()
	if err != nil {
		t.Fatalf("ListExperts: %v", err)
	}
	if len(experts) != 1 || experts[0] != "x4" {
		t.Errorf("experts = %v, want [x4]", experts)
	}
}

func TestExpertNameValidation(t *testing.T) {
	s := newTestStore(t)
	cases := []string{"../escape", "X4", "a/b", "", strings.Repeat("a", 80)}
	for _, name := range cases {
		if err := s.CreateExpert(name, "d"); err == nil {
			t.Errorf("CreateExpert(%q) accepted invalid name", name)
		}
	}
}

func TestReadFileValidation(t *testing.T) {
	s := newTestStore(t)
	if err := s.CreateExpert("x4", "d"); err != nil {
		t.Fatalf("CreateExpert: %v", err)
	}
	for _, f := range []string{"../../etc/passwd", "notes/../profile.md", "other.md", "notes/UP.md"} {
		if _, err := s.ReadFile("x4", f); err == nil {
			t.Errorf("ReadFile(%q) accepted invalid path", f)
		}
	}
	if _, err := s.ReadFile("x4", "notes/missing.md"); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing note error = %v, want ErrNotFound", err)
	}
}

func TestRecordDecisionRequiresWhy(t *testing.T) {
	s := newTestStore(t)
	if err := s.CreateExpert("x4", "d"); err != nil {
		t.Fatalf("CreateExpert: %v", err)
	}
	if err := s.RecordDecision("x4", "build a wharf", " "); err == nil {
		t.Fatal("decision without why accepted")
	}
	if err := s.RecordDecision("x4", "build a wharf in Segaris", "closer to Terran economy"); err != nil {
		t.Fatalf("RecordDecision: %v", err)
	}
	content, err := s.ReadFile("x4", "decisions.md")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(content, "build a wharf in Segaris") || !strings.Contains(content, "**Why:** closer to Terran economy") {
		t.Errorf("decisions.md missing entry:\n%s", content)
	}
}

func TestDigestComposition(t *testing.T) {
	s := newTestStore(t)
	if err := s.CreateExpert("x4", "Terran playthrough"); err != nil {
		t.Fatalf("CreateExpert: %v", err)
	}
	if err := s.ReplaceFile("x4", "goals.md", "# Goals\n\n- self-sufficient Terran economy"); err != nil {
		t.Fatalf("ReplaceFile: %v", err)
	}
	for i, d := range []string{"one", "two", "three", "four", "five", "six"} {
		if err := s.RecordDecision("x4", "decision "+d, "why "+d); err != nil {
			t.Fatalf("decision %d: %v", i, err)
		}
	}
	if err := s.AppendSessionLog("x4", "Captured first Xenon K."); err != nil {
		t.Fatalf("AppendSessionLog: %v", err)
	}
	if err := s.SetSessionContext("x4", "tonight: boarding ops practice"); err != nil {
		t.Fatalf("SetSessionContext: %v", err)
	}

	digest, err := s.Digest("x4")
	if err != nil {
		t.Fatalf("Digest: %v", err)
	}
	for _, want := range []string{
		"# Memory digest: x4",
		"tonight: boarding ops practice",
		"self-sufficient Terran economy",
		"decision six",
		"Captured first Xenon K.",
	} {
		if !strings.Contains(digest, want) {
			t.Errorf("digest missing %q:\n%s", want, digest)
		}
	}
	// Only the last 5 decisions survive the digest cut.
	if strings.Contains(digest, "decision one") {
		t.Error("digest includes decision beyond the recent-5 window")
	}
}

func TestSessionContextTTL(t *testing.T) {
	s := newTestStore(t)
	if err := s.CreateExpert("x4", "d"); err != nil {
		t.Fatalf("CreateExpert: %v", err)
	}
	if err := s.SetSessionContext("x4", "boarding tonight"); err != nil {
		t.Fatalf("SetSessionContext: %v", err)
	}
	got, err := s.SessionContext("x4")
	if err != nil || got != "boarding tonight" {
		t.Fatalf("SessionContext = %q, %v", got, err)
	}
	// Age the clock past the TTL: stale scratch must not leak.
	s.now = func() time.Time { return time.Now().Add(25 * time.Hour) }
	got, err = s.SessionContext("x4")
	if err != nil || got != "" {
		t.Errorf("expired SessionContext = %q, %v; want empty", got, err)
	}
}

func TestSearch(t *testing.T) {
	s := newTestStore(t)
	if err := s.CreateExpert("x4", "d"); err != nil {
		t.Fatalf("CreateExpert: %v", err)
	}
	if err := s.RecordNote("x4", "boarding", "Use three-star marines above 40% hull."); err != nil {
		t.Fatalf("RecordNote: %v", err)
	}
	hits, err := s.Search("x4", "MARINES")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(hits) != 1 || hits[0].File != "notes/boarding.md" {
		t.Fatalf("hits = %+v, want one hit in notes/boarding.md", hits)
	}
	if _, err := s.Search("x4", "  "); err == nil {
		t.Error("empty query accepted")
	}
}

func TestExternalEditsCheckpointedSeparately(t *testing.T) {
	s := newTestStore(t)
	if err := s.CreateExpert("x4", "d"); err != nil {
		t.Fatalf("CreateExpert: %v", err)
	}
	before := commitCount(t, s)

	// A human edits goals.md directly (no tool involved)...
	goals := filepath.Join(s.root, "x4", "goals.md")
	if err := os.WriteFile(goals, []byte("# Goals\n\n- edited by hand\n"), 0o644); err != nil {
		t.Fatalf("hand edit: %v", err)
	}
	// ...then a tool write happens. The hand edit must land in its own
	// checkpoint commit, not be absorbed into the tool's commit.
	if err := s.RecordDecision("x4", "some decision", "some why"); err != nil {
		t.Fatalf("RecordDecision: %v", err)
	}
	if got := commitCount(t, s); got != before+2 {
		t.Errorf("commits = %d, want %d (checkpoint + tool write)", got, before+2)
	}
	out, err := s.git("log", "--format=%s", "-n", "2")
	if err != nil {
		t.Fatalf("git log: %v", err)
	}
	if !strings.Contains(out, "checkpoint external edits") {
		t.Errorf("log missing checkpoint commit:\n%s", out)
	}
}

func TestNoOpWriteMakesNoCommit(t *testing.T) {
	s := newTestStore(t)
	if err := s.CreateExpert("x4", "d"); err != nil {
		t.Fatalf("CreateExpert: %v", err)
	}
	if err := s.ReplaceFile("x4", "goals.md", "# Goals\n\n- same"); err != nil {
		t.Fatalf("first replace: %v", err)
	}
	before := commitCount(t, s)
	if err := s.ReplaceFile("x4", "goals.md", "# Goals\n\n- same"); err != nil {
		t.Fatalf("identical replace: %v", err)
	}
	if got := commitCount(t, s); got != before {
		t.Errorf("identical write created a commit (%d -> %d)", before, got)
	}
}

func TestSetGetStateRoundTrip(t *testing.T) {
	s := newTestStore(t)
	if err := s.CreateExpert("x4", "d"); err != nil {
		t.Fatalf("CreateExpert: %v", err)
	}

	// Unset key is a clean not-found, not a read error.
	if _, err := s.GetState("x4", "mastery"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetState unset = %v, want ErrNotFound", err)
	}

	before := commitCount(t, s)
	value := json.RawMessage(`{"course_slug":"x4-econ","due":4,"coverage":{"questions":12,"mastered":3}}`)
	if err := s.SetState("x4", "mastery", value); err != nil {
		t.Fatalf("SetState: %v", err)
	}
	if got := commitCount(t, s); got != before+1 {
		t.Errorf("commits = %d, want %d (state write commits)", got, before+1)
	}
	out, err := s.git("log", "--format=%s", "-n", "1")
	if err != nil {
		t.Fatalf("git log: %v", err)
	}
	if !strings.Contains(out, "state: set mastery") {
		t.Errorf("commit message = %q, want it to name the state write", out)
	}

	raw, err := s.GetState("x4", "mastery")
	if err != nil {
		t.Fatalf("GetState: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("stored state is not JSON: %v\n%s", err, raw)
	}
	if got["course_slug"] != "x4-econ" || got["due"] != float64(4) {
		t.Errorf("round trip mismatch: %v", got)
	}
	// Stored canonical (pretty-printed), so git diffs of machine writes stay minimal.
	if !strings.Contains(string(raw), "\n  \"due\": 4") {
		t.Errorf("state not stored pretty-printed:\n%s", raw)
	}

	// Guardrails: traversal-proof keys and valid JSON only.
	if err := s.SetState("x4", "../evil", value); err == nil {
		t.Error("bad key accepted")
	}
	if err := s.SetState("x4", "broken", json.RawMessage(`{not json`)); err == nil {
		t.Error("invalid JSON accepted")
	}
	if err := s.SetState("nope", "mastery", value); !errors.Is(err, ErrNotFound) {
		t.Errorf("SetState unknown expert = %v, want ErrNotFound", err)
	}
}

func TestStateExcludedFromSearchAndReadMany(t *testing.T) {
	s := newTestStore(t)
	if err := s.CreateExpert("x4", "d"); err != nil {
		t.Fatalf("CreateExpert: %v", err)
	}
	// A token that exists ONLY inside state, never in prose memory.
	if err := s.SetState("x4", "mastery", json.RawMessage(`{"note":"zyxblindspot"}`)); err != nil {
		t.Fatalf("SetState: %v", err)
	}

	files, err := s.ListFiles("x4")
	if err != nil {
		t.Fatalf("ListFiles: %v", err)
	}
	for _, f := range files {
		if strings.HasPrefix(f, "state/") {
			t.Errorf("ListFiles surfaced state file %q", f)
		}
	}

	hits, err := s.Search("x4", "zyxblindspot")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(hits) != 0 {
		t.Errorf("search surfaced state content: %+v", hits)
	}

	// Even a glob aimed directly at the subtree must not read it back.
	if _, err := s.ReadFiles("x4", "state/*", 0); err == nil {
		t.Error("read_many matched state/*")
	}
	all, err := s.ReadFiles("x4", "*", 0)
	if err != nil {
		t.Fatalf("ReadFiles *: %v", err)
	}
	for _, f := range all {
		if strings.Contains(f.Text, "zyxblindspot") {
			t.Errorf("read_many surfaced state content in %s", f.File)
		}
	}
}

const masteryFixture = `{
  "course_slug": "x4-econ",
  "updated": "2026-08-01T12:00:00Z",
  "coverage": {"questions": 12, "attempted": 7, "mastered": 3, "mastered_pct": 25},
  "due": 4,
  "blindspots": [{"topic": "trade-routes", "note": "keeps confusing buy and sell sides"}],
  "weak_units": [
    {"unit": "station-building", "mastered_pct": 40},
    {"unit": "boarding", "mastered_pct": 10}
  ]
}`

func TestDigestMasterySection(t *testing.T) {
	s := newTestStore(t)
	if err := s.CreateExpert("x4", "d"); err != nil {
		t.Fatalf("CreateExpert: %v", err)
	}
	if err := s.SetState("x4", "mastery", json.RawMessage(masteryFixture)); err != nil {
		t.Fatalf("SetState: %v", err)
	}

	digest, err := s.Digest("x4")
	if err != nil {
		t.Fatalf("Digest: %v", err)
	}
	for _, want := range []string{
		"## Mastery",
		"3/12 questions mastered (25%; 7 attempted)",
		"Due for review: 4",
		"trade-routes — keeps confusing buy and sell sides",
	} {
		if !strings.Contains(digest, want) {
			t.Errorf("digest missing %q:\n%s", want, digest)
		}
	}
	// Weak units render weakest first regardless of file order.
	weak := strings.Index(digest, "boarding — 10% mastered")
	strong := strings.Index(digest, "station-building — 40% mastered")
	if weak < 0 || strong < 0 || weak > strong {
		t.Errorf("weak units not ordered weakest-first:\n%s", digest)
	}
}

func TestDigestMasteryDegrades(t *testing.T) {
	s := newTestStore(t)
	if err := s.CreateExpert("x4", "d"); err != nil {
		t.Fatalf("CreateExpert: %v", err)
	}

	// No mastery file: no section, no error.
	plain, err := s.Digest("x4")
	if err != nil {
		t.Fatalf("Digest without mastery: %v", err)
	}
	if strings.Contains(plain, "## Mastery") {
		t.Errorf("digest without mastery file has a Mastery section:\n%s", plain)
	}

	// Malformed file on disk (a crashed writer): digest must survive.
	stateDir := filepath.Join(s.root, "x4", "state")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(stateDir, "mastery.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatalf("write malformed: %v", err)
	}
	broken, err := s.Digest("x4")
	if err != nil {
		t.Fatalf("Digest with malformed mastery: %v", err)
	}
	if strings.Contains(broken, "## Mastery") {
		t.Errorf("malformed mastery rendered a section:\n%s", broken)
	}

	// Partial file (writer ahead of/behind the schema): renders what it has.
	if err := os.WriteFile(filepath.Join(stateDir, "mastery.json"),
		[]byte(`{"due": 9, "future_key": {"nested": true}}`), 0o644); err != nil {
		t.Fatalf("write partial: %v", err)
	}
	partial, err := s.Digest("x4")
	if err != nil {
		t.Fatalf("Digest with partial mastery: %v", err)
	}
	if !strings.Contains(partial, "Due for review: 9") || strings.Contains(partial, "future_key") {
		t.Errorf("partial mastery mis-rendered:\n%s", partial)
	}
}

func TestMain(m *testing.M) {
	if _, err := exec.LookPath("git"); err != nil {
		os.Exit(0) // environment without git: nothing here is testable
	}
	os.Exit(m.Run())
}

func TestDescribe(t *testing.T) {
	cases := []struct {
		name, content, want string
	}{
		{
			name:    "title and first prose line",
			content: "# Forestry plan\n\nScenario C deep dive: managed Acadian forest.\n\n## Details\n",
			want:    "Forestry plan — Scenario C deep dive: managed Acadian forest.",
		},
		{"title only", "# Goals\n", "Goals"},
		{"prose only", "just a line of text\n", "just a line of text"},
		{"skips deeper headings and quotes", "# T\n\n> a quote\n\n## Sub\n\nreal line\n", "T — real line"},
		{"empty", "\n\n", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := describe(tc.content); got != tc.want {
				t.Errorf("describe() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSearchHitsCarryFileDescription(t *testing.T) {
	s := newTestStore(t)
	if err := s.CreateExpert("x4", "d"); err != nil {
		t.Fatalf("CreateExpert: %v", err)
	}
	if err := s.RecordNote("x4", "boarding", "Use three-star marines above 40% hull."); err != nil {
		t.Fatalf("RecordNote: %v", err)
	}
	hits, err := s.Search("x4", "marines")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("hits = %d, want 1", len(hits))
	}
	// A note created by RecordNote is "# <topic>" then the content, so the
	// hit should be attributable without opening the file.
	if !strings.Contains(hits[0].About, "boarding") {
		t.Errorf("hit About = %q, want it to name the note", hits[0].About)
	}
}

func TestReadFileLines(t *testing.T) {
	s := newTestStore(t)
	if err := s.CreateExpert("x4", "d"); err != nil {
		t.Fatalf("CreateExpert: %v", err)
	}
	body := "# Long\n\n"
	for i := 1; i <= 50; i++ {
		body += fmt.Sprintf("line %d\n", i)
	}
	if err := s.RecordNote("x4", "long", body); err != nil {
		t.Fatalf("RecordNote: %v", err)
	}

	whole, total, err := s.ReadFileLines("x4", "notes/long.md", 0, 0)
	if err != nil {
		t.Fatalf("ReadFileLines whole: %v", err)
	}
	if !strings.Contains(whole, "line 50") || total < 50 {
		t.Fatalf("whole read wrong: total=%d", total)
	}

	window, total2, err := s.ReadFileLines("x4", "notes/long.md", 5, 3)
	if err != nil {
		t.Fatalf("ReadFileLines window: %v", err)
	}
	if got := strings.Count(window, "\n") + 1; got != 3 {
		t.Errorf("window lines = %d, want 3", got)
	}
	if total2 != total {
		t.Errorf("total changed with window: %d vs %d", total2, total)
	}
	// Past the end is empty, not an error — a caller probing the tail of a
	// file it has not measured should not get a failure.
	if past, _, err := s.ReadFileLines("x4", "notes/long.md", 9999, 10); err != nil || past != "" {
		t.Errorf("past-end read = %q, %v; want empty, nil", past, err)
	}
}

func TestReadFiles(t *testing.T) {
	s := newTestStore(t)
	if err := s.CreateExpert("x4", "d"); err != nil {
		t.Fatalf("CreateExpert: %v", err)
	}
	for _, topic := range []string{"boarding", "mining"} {
		if err := s.RecordNote("x4", topic, "content for "+topic+"\nmore\nlines\nhere\n"); err != nil {
			t.Fatalf("RecordNote %s: %v", topic, err)
		}
	}

	files, err := s.ReadFiles("x4", "notes/*", 0)
	if err != nil {
		t.Fatalf("ReadFiles: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("files = %d, want 2", len(files))
	}
	for _, f := range files {
		if f.About == "" || f.Text == "" || f.TotalLines == 0 {
			t.Errorf("incomplete MultiFile: %+v", f)
		}
	}

	// The per-file cap must mark what it cut.
	capped, err := s.ReadFiles("x4", "notes/boarding.md", 2)
	if err != nil {
		t.Fatalf("ReadFiles capped: %v", err)
	}
	if len(capped) != 1 || !capped[0].Truncated {
		t.Fatalf("expected one truncated file, got %+v", capped)
	}
	if got := strings.Count(capped[0].Text, "\n") + 1; got != 2 {
		t.Errorf("capped lines = %d, want 2", got)
	}

	if _, err := s.ReadFiles("x4", "nothing/*", 0); err == nil {
		t.Error("non-matching pattern did not error")
	}
	if _, err := s.ReadFiles("x4", "", 0); err == nil {
		t.Error("empty pattern did not error")
	}
}
