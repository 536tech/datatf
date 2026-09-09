package update

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCheckVersionsAndDailyCache(t *testing.T) {
	for _, test := range []struct{ current, latest, want string }{
		{"0.2.0", "v0.3.0", "v0.3.0"}, {"v0.9.0", "v0.10.0", "v0.10.0"},
		{"0.3.0", "v0.3.0", ""}, {"0.4.0", "v0.3.0", ""},
		{"dev", "v0.3.0", ""}, {"0.4.0-next", "v0.3.0", ""},
		{"0.3.0", "v0.4.0-rc.1", ""}, {"0.3.0", "v0.4.0\ncanary", ""},
	} {
		t.Run(test.current+"/"+test.latest, func(t *testing.T) {
			calls := 0
			old := fetchLatest
			fetchLatest = func(context.Context) (string, error) { calls++; return test.latest, nil }
			t.Cleanup(func() { fetchLatest = old })
			path := filepath.Join(t.TempDir(), "update.json")
			now := time.Now()
			for _, at := range []time.Time{now, now.Add(time.Hour)} {
				if got := Check(context.Background(), test.current, path, at); got != test.want {
					t.Fatalf("got %q, want %q", got, test.want)
				}
			}
			wantCalls := 1
			if stableVersion(test.current) == "" {
				wantCalls = 0
			}
			if calls != wantCalls {
				t.Fatalf("requests: %d, want %d", calls, wantCalls)
			}
		})
	}
}

func TestCheckFailureAndCacheRecovery(t *testing.T) {
	old := fetchLatest
	t.Cleanup(func() { fetchLatest = old })
	calls := 0
	fetchLatest = func(context.Context) (string, error) {
		calls++
		return "", fmt.Errorf("offline")
	}
	path := filepath.Join(t.TempDir(), "update.json")
	now := time.Now()
	if err := os.WriteFile(path, []byte("invalid cache"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, at := range []time.Time{now, now.Add(time.Hour)} {
		if got := Check(context.Background(), "0.2.0", path, at); got != "" {
			t.Fatal(got)
		}
	}
	if calls != 1 {
		t.Fatalf("offline requests: %d", calls)
	}
	fetchLatest = func(context.Context) (string, error) { return "v0.3.0", nil }
	if got := Check(context.Background(), "0.2.0", path, now.Add(24*time.Hour)); got != "v0.3.0" {
		t.Fatalf("expired cache: %q", got)
	}
	if got := Check(context.Background(), "0.3.0", path, now.Add(25*time.Hour)); got != "" {
		t.Fatalf("notice after upgrade: %q", got)
	}
}

func TestFetchReleaseBoundary(t *testing.T) {
	for _, test := range []struct {
		name, body string
		status     int
		want       string
	}{
		{"stable", `{"tag_name":"v0.3.0","html_url":"https://untrusted.invalid"}`, 200, "v0.3.0"},
		{"draft", `{"tag_name":"v0.3.0","draft":true}`, 200, ""},
		{"prerelease", `{"tag_name":"v0.3.0-rc.1","prerelease":true}`, 200, ""},
		{"invalid tag", `{"tag_name":"v0.3.0\u001b[31m"}`, 200, ""},
		{"truncated", `{"tag_name":`, 200, ""},
		{"trailing JSON", `{"tag_name":"v0.3.0"} {}`, 200, ""},
		{"rate limited", `{}`, 403, ""}, {"not found", `{}`, 404, ""},
		{"oversized", strings.Repeat(" ", maxResponse+1), 200, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "GET" || r.URL.RawQuery != "" || r.ContentLength > 0 {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL)
				}
				for _, header := range []string{"Authorization", "Cookie"} {
					assertEmptyHeader(t, r, header)
				}
				if r.UserAgent() != "datatf" {
					t.Errorf("user agent: %q", r.UserAgent())
				}
				w.WriteHeader(test.status)
				_, _ = fmt.Fprint(w, test.body)
			}))
			defer srv.Close()
			got, _ := fetchRelease(context.Background(), srv.URL)
			if got != test.want {
				t.Fatalf("got %q, want %q", got, test.want)
			}
		})
	}
}

func assertEmptyHeader(t *testing.T, r *http.Request, header string) {
	t.Helper()
	if r.Header.Get(header) != "" {
		t.Errorf("request contains %s", header)
	}
}

func TestCheckSkipsNetworkWithoutCacheOrAfterCancellation(t *testing.T) {
	old := fetchLatest
	t.Cleanup(func() { fetchLatest = old })
	fetchLatest = func(context.Context) (string, error) {
		t.Fatal("unexpected network request")
		return "", nil
	}
	parent := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(parent, nil, 0600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(parent, "cache")
	if got := Check(context.Background(), "0.2.0", path, time.Now()); got != "" {
		t.Fatal(got)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got := Check(ctx, "0.2.0", filepath.Join(t.TempDir(), "cache"), time.Now()); got != "" {
		t.Fatal(got)
	}
}

func TestCacheDoesNotFollowSymlinks(t *testing.T) {
	dir := t.TempDir()
	target, link := filepath.Join(dir, "target"), filepath.Join(dir, "update.json")
	if err := os.WriteFile(target, []byte("preserve me"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks are unavailable: %v", err)
	}
	if got := readCache(link); got.Latest != "" {
		t.Fatal(got)
	}
	if err := writeCache(link, cache{CheckedAt: time.Now(), Latest: "v0.3.0"}); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(target)
	if err != nil || string(contents) != "preserve me" {
		t.Fatal("cache write changed the symlink target")
	}
}

func TestFetchDoesNotRetryRedirectOrWaitPastTimeout(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		http.Redirect(w, r, "/next", http.StatusFound)
	}))
	defer srv.Close()
	if got, err := fetchRelease(context.Background(), srv.URL); got != "" || err == nil {
		t.Fatalf("redirect: %q %v", got, err)
	}
	if calls != 1 {
		t.Fatalf("requests: %d", calls)
	}
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer slow.Close()
	start := time.Now()
	if _, err := fetchRelease(context.Background(), slow.URL); err == nil {
		t.Fatal("no timeout")
	}
	if time.Since(start) > 2*Timeout {
		t.Fatal("exceeded timeout budget")
	}
}
