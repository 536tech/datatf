// Package update checks public release metadata without credentials or usage events.
package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/mod/semver"
)

const (
	Timeout     = 500 * time.Millisecond
	Releases    = "https://github.com/536tech/datatf/releases/tag/"
	releaseAPI  = "https://api.github.com/repos/536tech/datatf/releases/latest"
	maxResponse = 1024 * 1024
)

type cache struct {
	CheckedAt time.Time `json:"checked_at"`
	Latest    string    `json:"latest"`
}

var fetchLatest = func(ctx context.Context) (string, error) { return fetchRelease(ctx, releaseAPI) }

// Check returns a newer stable version, or an empty string when no notice is available.
// A cache failure disables the check. Network failures are cached for 24 hours.
func Check(ctx context.Context, current, path string, now time.Time) string {
	current = stableVersion(current)
	if current == "" || ctx.Err() != nil {
		return ""
	}
	saved := readCache(path)
	age := now.Sub(saved.CheckedAt)
	if age < 0 || age >= 24*time.Hour {
		saved.CheckedAt = now
		saved = refreshCache(ctx, path, saved)
	}
	latest := stableVersion(saved.Latest)
	if semver.Compare(latest, current) > 0 {
		return latest
	}
	return ""
}

func refreshCache(ctx context.Context, path string, saved cache) cache {
	// Record the attempt first so blocked networks do not delay every command.
	if err := writeCache(path, saved); err != nil {
		return cache{}
	}
	latest, err := fetchLatest(ctx)
	if err != nil {
		return cache{}
	}
	saved.Latest = stableVersion(latest)
	if err := writeCache(path, saved); err != nil {
		return cache{}
	}
	return saved
}

func stableVersion(value string) string {
	version := "v" + strings.TrimPrefix(value, "v")
	if len(version) > 64 || semver.Canonical(version) != version || semver.Prerelease(version) != "" {
		return ""
	}
	return version
}

func fetchRelease(ctx context.Context, endpoint string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "datatf")
	transport := &http.Transport{Proxy: http.ProxyFromEnvironment, DisableKeepAlives: true}
	defer transport.CloseIdleConnections()
	client := &http.Client{
		Transport: transport, Timeout: Timeout,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	response, err := client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("release check returned HTTP %d", response.StatusCode)
	}
	return decodeRelease(response.Body)
}

func decodeRelease(body io.Reader) (string, error) {
	data, err := io.ReadAll(io.LimitReader(body, maxResponse+1))
	if err != nil {
		return "", err
	}
	if len(data) > maxResponse {
		return "", fmt.Errorf("release response exceeds size limit")
	}
	var release struct {
		Tag        string `json:"tag_name"`
		Draft      bool   `json:"draft"`
		Prerelease bool   `json:"prerelease"`
	}
	if err := json.Unmarshal(data, &release); err != nil {
		return "", err
	}
	if release.Draft || release.Prerelease {
		return "", nil
	}
	return stableVersion(release.Tag), nil
}

func readCache(path string) cache {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > 4096 {
		return cache{}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return cache{}
	}
	var saved cache
	if json.Unmarshal(data, &saved) != nil {
		return cache{}
	}
	return saved
}

func writeCache(path string, saved cache) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".update-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	writeErr := json.NewEncoder(file).Encode(saved)
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(file.Name(), path)
}
