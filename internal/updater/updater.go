// Package updater implements self-update: it checks the latest GitHub release,
// downloads the matching Linux binary, verifies its SHA-256 checksum against the
// published SHA256SUMS file, and atomically replaces the running executable.
package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Repo is the GitHub repository self-updates are pulled from. It is a compile
// time constant so the update source can never be influenced by user input.
const Repo = "brightcolor/acmesh-ui"

const (
	apiLatest = "https://api.github.com/repos/" + Repo + "/releases/latest"
	dlBase    = "https://github.com/" + Repo + "/releases/download"
	userAgent = "acmesh-ui-updater"
)

// CheckResult describes the available vs. installed version.
type CheckResult struct {
	Current          string `json:"current"`
	Latest           string `json:"latest"`
	UpdateAvailable  bool   `json:"update_available"`
	CanSelfUpdate    bool   `json:"can_self_update"`
	RestartSupported bool   `json:"restart_supported"`
	Asset            string `json:"asset"`
	Note             string `json:"note,omitempty"`
}

func httpClient() *http.Client { return &http.Client{Timeout: 60 * time.Second} }

// AssetName is the release asset for the running OS/arch.
func AssetName() string {
	return fmt.Sprintf("acmesh-ui-%s-%s", runtime.GOOS, runtime.GOARCH)
}

// LatestTag queries the GitHub API for the latest release tag.
func LatestTag(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiLatest, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := httpClient().Do(req)
	if err != nil {
		return "", fmt.Errorf("query latest release: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("github API returned status %d", resp.StatusCode)
	}
	var rel struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&rel); err != nil {
		return "", fmt.Errorf("decode release: %w", err)
	}
	if rel.TagName == "" {
		return "", fmt.Errorf("no tag in latest release")
	}
	return rel.TagName, nil
}

// Check compares the current version against the latest release.
func Check(ctx context.Context, current string) (CheckResult, error) {
	res := CheckResult{Current: current, Asset: AssetName(), CanSelfUpdate: CanSelfUpdate()}
	latest, err := LatestTag(ctx)
	if err != nil {
		return res, err
	}
	res.Latest = latest
	res.UpdateAvailable = latest != "" && latest != current
	if !res.CanSelfUpdate {
		res.Note = "The binary's directory is not writable by this process; run 'sudo acmesh-ui update' on the host."
	}
	return res, nil
}

// Apply downloads the latest release for this platform, verifies its checksum
// and replaces the running executable. It returns the tag that was installed.
//
// If targetTag is empty the latest release is used.
func Apply(ctx context.Context, targetTag string) (string, error) {
	if runtime.GOOS != "linux" {
		return "", fmt.Errorf("self-update only ships binaries for linux (this is %s)", runtime.GOOS)
	}
	tag := targetTag
	if tag == "" {
		var err error
		if tag, err = LatestTag(ctx); err != nil {
			return "", err
		}
	}

	asset := AssetName()
	base := dlBase + "/" + tag
	binData, err := download(ctx, base+"/"+asset)
	if err != nil {
		return "", fmt.Errorf("download %s: %w", asset, err)
	}

	// Verify against SHA256SUMS (required - we never install an unverified binary).
	sums, err := download(ctx, base+"/SHA256SUMS")
	if err != nil {
		return "", fmt.Errorf("download checksums: %w", err)
	}
	want, ok := findChecksum(string(sums), asset)
	if !ok {
		return "", fmt.Errorf("no checksum for %s in SHA256SUMS", asset)
	}
	sum := sha256.Sum256(binData)
	got := hex.EncodeToString(sum[:])
	if !strings.EqualFold(got, want) {
		return "", fmt.Errorf("checksum mismatch (want %s, got %s)", want, got)
	}

	if err := replaceExecutable(binData); err != nil {
		return "", err
	}
	return tag, nil
}

// ExecutablePath returns the resolved path of the running binary.
func ExecutablePath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		return resolved, nil
	}
	return exe, nil
}

// CanSelfUpdate reports whether the directory holding the binary is writable by
// this process (a prerequisite for an in-place replace).
func CanSelfUpdate() bool {
	exe, err := ExecutablePath()
	if err != nil {
		return false
	}
	dir := filepath.Dir(exe)
	f, err := os.CreateTemp(dir, ".acmesh-ui-wtest-*")
	if err != nil {
		return false
	}
	name := f.Name()
	f.Close()
	os.Remove(name)
	return true
}

// replaceExecutable writes data to a temp file next to the current binary and
// atomically renames it over the executable. On Linux a running binary can be
// replaced this way; the running process keeps the old inode until it restarts.
func replaceExecutable(data []byte) error {
	exe, err := ExecutablePath()
	if err != nil {
		return err
	}
	dir := filepath.Dir(exe)
	tmp, err := os.CreateTemp(dir, ".acmesh-ui-new-*")
	if err != nil {
		return fmt.Errorf("create temp in %s: %w (need write permission on the binary directory)", dir, err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op if the rename succeeded

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write new binary: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, 0o755); err != nil {
		return err
	}
	if err := os.Rename(tmpName, exe); err != nil {
		return fmt.Errorf("replace %s: %w", exe, err)
	}
	return nil
}

func download(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d for %s", resp.StatusCode, url)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 100<<20))
}

// findChecksum returns the hex checksum for asset from a SHA256SUMS body.
func findChecksum(sums, asset string) (string, bool) {
	for _, line := range strings.Split(sums, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && strings.TrimPrefix(fields[1], "*") == asset {
			return fields[0], true
		}
	}
	return "", false
}
