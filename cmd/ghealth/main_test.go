package main

import (
	"runtime/debug"
	"testing"
)

func TestResolveBuildMetadataUsesGoModuleAndVCSValues(t *testing.T) {
	info := &debug.BuildInfo{
		Main: debug.Module{Version: "v1.1.1"},
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "release-commit"},
			{Key: "vcs.time", Value: "2026-07-31T15:00:00Z"},
		},
	}
	version, commit, date := resolveBuildMetadata("devel", "unknown", "unknown", info)
	if version != "1.1.1" || commit != "release-commit" || date != "2026-07-31T15:00:00Z" {
		t.Fatalf("resolved metadata = %q, %q, %q", version, commit, date)
	}
}

func TestResolveBuildMetadataUsesModuleVersionWithoutVCSValues(t *testing.T) {
	info := &debug.BuildInfo{Main: debug.Module{Version: "v1.1.1"}}
	version, commit, date := resolveBuildMetadata("devel", "unknown", "unknown", info)
	if version != "1.1.1" || commit != "unknown" || date != "unknown" {
		t.Fatalf("resolved metadata = %q, %q, %q", version, commit, date)
	}
}

func TestResolveBuildMetadataPreservesGoReleaserValues(t *testing.T) {
	info := &debug.BuildInfo{
		Main: debug.Module{Version: "v9.9.9"},
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "go-commit"},
			{Key: "vcs.time", Value: "go-time"},
		},
	}
	version, commit, date := resolveBuildMetadata("1.1.1", "release-commit", "release-time", info)
	if version != "1.1.1" || commit != "release-commit" || date != "release-time" {
		t.Fatalf("resolved metadata = %q, %q, %q", version, commit, date)
	}
}

func TestResolveBuildMetadataHandlesMissingBuildInfo(t *testing.T) {
	version, commit, date := resolveBuildMetadata("devel", "unknown", "unknown", nil)
	if version != "devel" || commit != "unknown" || date != "unknown" {
		t.Fatalf("resolved metadata = %q, %q, %q", version, commit, date)
	}
}
