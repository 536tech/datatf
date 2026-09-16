package scaffold

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"golang.org/x/mod/semver"
)

var publicModule = regexp.MustCompile(`^(?:registry\.terraform\.io/)?([A-Za-z0-9_-]+/[A-Za-z0-9_-]+/[A-Za-z0-9_-]+)$`)

// PublicRegistryModule reports whether latest can resolve this source.
func PublicRegistryModule(source string) bool { return publicModule.MatchString(source) }

// LatestVersions resolves each public Registry source to its newest stable release.
// The default export never calls this endpoint. No workspace information is sent.
func LatestVersions(ctx context.Context, sources []string) (map[string]string, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	client := &http.Client{Timeout: 10 * time.Second}
	versions := make(map[string]string, len(sources))
	for _, source := range sources {
		version, err := latestVersion(ctx, client, source)
		if err != nil {
			return nil, err
		}
		versions[source] = version
	}
	return versions, nil
}

func latestVersion(ctx context.Context, client *http.Client, source string) (string, error) {
	parts := publicModule.FindStringSubmatch(source)
	if parts == nil {
		return "", fmt.Errorf("latest requires a public Terraform Registry module source")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://registry.terraform.io/v1/modules/"+parts[1]+"/versions", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")
	res, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("resolve module version for %s: %w", source, err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("resolve module version for %s: Registry HTTP %d", source, res.StatusCode)
	}
	var listing struct {
		Modules []struct {
			Versions []struct {
				Version string `json:"version"`
			} `json:"versions"`
		} `json:"modules"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 4<<20)).Decode(&listing); err != nil {
		return "", fmt.Errorf("resolve module version for %s: invalid Registry response", source)
	}
	latest := ""
	if len(listing.Modules) > 0 {
		for _, release := range listing.Modules[0].Versions {
			v := "v" + release.Version
			// Canonical stable versions have all three numeric components and no prefix.
			if semver.Canonical(v) != v || semver.Prerelease(v) != "" {
				continue
			}
			if latest == "" || semver.Compare(v, "v"+latest) > 0 {
				latest = strings.TrimPrefix(v, "v")
			}
		}
	}
	if latest == "" {
		return "", fmt.Errorf("resolve module version for %s: no stable release", source)
	}
	return latest, nil
}
