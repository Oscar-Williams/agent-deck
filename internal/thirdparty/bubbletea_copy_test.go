package thirdparty

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// third_party/bubbletea is bubbletea v1.3.10 with one patch (#2692). These are
// the SHA-256 of every upstream file kept in the copy, from the module that
// go.sum verified. tty.go is checked with the patch hunk taken out.
var bubbleteaUpstream = map[string]string{
	"LICENSE":                "69793670485e4ae8bf28fcbe5639141f604256d5bb6c1862571f3a32823fa221",
	"commands.go":            "e4733645ed4a3afe7b90585bc4014198f937d140d099b069ff16164d8e9fb949",
	"exec.go":                "80739d13c56f1a95ee08568fb667beb1edc9423f0d704afc9b32b97dba24f2d0",
	"focus.go":               "ac27153c7127a434edf501816150c1919fb6b8d9c8ddd542f4293a7e053a50d3",
	"go.mod":                 "903f2904b09b37dd74092f437ad306794e7633329ddc5af7b201d469e2d937fb",
	"go.sum":                 "5fe23fd458f3a5c92a77aa933e0a6d83de545a0b59b33c698a9c33e39d057755",
	"inputreader_other.go":   "eb0ae77b31947e3e996b23de0b85b606c969b87a7a456ad564a3b1c3c3800b13",
	"inputreader_windows.go": "5e2fb998078e1455e9f8a5ec871f0ed950e219c93bbc04af17daa228074ba7ba",
	"key.go":                 "364667133bcd72e7bfe22e30eb92d462f18aadaffa96a4c32cdccbe00368b43f",
	"key_other.go":           "ae456d473d43a8def40ca31ec7826a40392b84d47b8c27bfe3f12006bcf3c2ff",
	"key_sequences.go":       "a552fc2ce1e0eec029fef551f981a50bfd4166289aa378ec4bf8906f05395e48",
	"key_windows.go":         "a84bc6eb41a98580539236061f91c5b83fb2ffb61dd1dc4b8e80cf14e02943b5",
	"logging.go":             "af7eaaf5b8ae2a4edc631a8750e93644459dae91bcb25e3c3de94a3b9e6593ad",
	"mouse.go":               "d91927f56a91dde0fe7f62ebcb26f954cd86bc68ae4006270e34417f8c2bc983",
	"nil_renderer.go":        "6dcf82b44a05df0af91ce6b70443bdc88da2bc2a429276ca1811790dd71102cd",
	"options.go":             "4f78a834d8f82ca45dda6f292bcc54b53f198c089a059d207ff02f8e606891ca",
	"renderer.go":            "2c53589b014d3b7bc2c74d4c332d29862eb064971b150a66c0fa38f6fef61f43",
	"screen.go":              "13539b00d61af207d9c968ed198a0b57cff428d86d65509f54c9f146546523cb",
	"signals_unix.go":        "3954426922ae0a399a29769ed8e85ace7c451f33bb835b7009336ed086e65bf3",
	"signals_windows.go":     "25437273fa773a70b2a11c8f6f63674d7483460f65d0352a2c5ca1d2c1677c4a",
	"standard_renderer.go":   "fe30321708716b962f78965cab407c4edccac2bc22f9dd5e72ac506420a1b4d6",
	"tea.go":                 "93a3744362223921e3f82ded63c65bf8029ff77d2e4a46604b77fbd37f87162e",
	"tea_init.go":            "87e7bcd11967ab1e5b0e541c5b89d3a0b4baf5695899f08dc1eeb6c94a4a5e91",
	"tty.go":                 "db99d0fe947fa401d079429dad62daaf4889c19f6121bb89258a132a87214144",
	"tty_unix.go":            "1206fdac0fc2d6940ec957f4c530638df2b6611de4a123cfdb99375bd7545f3f",
	"tty_windows.go":         "af4316838b7cac5e9230b82a93b4b407ef91b755a57005135d93f0eb2c8b6aad",
}

// bubbleteaPatch is the only change allowed on top of upstream (tty.go).
const bubbleteaPatch = `	// agent-deck patch (#2692): close the reader being replaced. Its kqueue
	// (macOS) or epoll (Linux) descriptor is a raw fd no finalizer closes, so
	// every RestoreTerminal (each tea.Exec) leaked one. Only once its read loop
	// has stopped: closing under a running loop would fail that read.
	if p.cancelReader != nil && p.readLoopDone != nil {
		select {
		case <-p.readLoopDone:
			_ = p.cancelReader.Close()
		default:
		}
	}

`

// TestBubbleteaCopyMatchesUpstream fails when the copy drifts from upstream
// v1.3.10 beyond the one patch: a file added, removed or changed.
func TestBubbleteaCopyMatchesUpstream(t *testing.T) {
	dir := filepath.Join("..", "..", "third_party", "bubbletea")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	var got []string
	for _, e := range entries {
		if e.Name() == "README.md" {
			continue // ours: names the upstream version and the patch
		}
		got = append(got, e.Name())
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		want, ok := bubbleteaUpstream[e.Name()]
		if !ok {
			t.Errorf("%s is not an upstream v1.3.10 file", e.Name())
			continue
		}
		if e.Name() == "tty.go" {
			if strings.Count(string(data), bubbleteaPatch) != 1 {
				t.Errorf("tty.go must carry the #2692 patch exactly once")
			}
			data = []byte(strings.Replace(string(data), bubbleteaPatch, "", 1))
		}
		if sum := sha256.Sum256(data); hex.EncodeToString(sum[:]) != want {
			t.Errorf("%s differs from upstream v1.3.10 (beyond the #2692 patch)", e.Name())
		}
	}
	var want []string
	for name := range bubbleteaUpstream {
		want = append(want, name)
	}
	sort.Strings(got)
	sort.Strings(want)
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("file set differs from upstream:\n got  %v\n want %v", got, want)
	}
}
