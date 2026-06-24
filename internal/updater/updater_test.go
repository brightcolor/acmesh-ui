package updater

import "testing"

func TestFindChecksum(t *testing.T) {
	sums := "abc123  acmesh-ui-linux-amd64\ndef456  acmesh-ui-linux-arm64\n"
	if c, ok := findChecksum(sums, "acmesh-ui-linux-arm64"); !ok || c != "def456" {
		t.Fatalf("arm64: got %q ok=%v", c, ok)
	}
	if c, ok := findChecksum(sums, "acmesh-ui-linux-amd64"); !ok || c != "abc123" {
		t.Fatalf("amd64: got %q ok=%v", c, ok)
	}
	if _, ok := findChecksum(sums, "missing"); ok {
		t.Fatalf("expected missing asset to not be found")
	}
	// BSD-style "*name" prefix
	if c, ok := findChecksum("deadbeef *acmesh-ui-linux-amd64\n", "acmesh-ui-linux-amd64"); !ok || c != "deadbeef" {
		t.Fatalf("star-prefix: got %q ok=%v", c, ok)
	}
}

func TestAssetName(t *testing.T) {
	if AssetName() == "" {
		t.Fatal("asset name empty")
	}
}
