package main

import "testing"

func TestResolvedVersionUsesInjectedVersion(t *testing.T) {
	original := version
	version = "1.2.3"
	defer func() { version = original }()
	if got := resolvedVersion(); got != "1.2.3" {
		t.Fatalf("resolvedVersion = %q", got)
	}
}

func TestResolvedDistribution(t *testing.T) {
	original := version
	version = "1.2.3"
	defer func() { version = original }()
	if got := resolvedDistribution(); got != "release" {
		t.Fatalf("resolvedDistribution = %q", got)
	}
}

func TestResolvedDistributionUsesNodeLauncherContext(t *testing.T) {
	original := version
	version = "1.2.3"
	defer func() { version = original }()
	t.Setenv("EPISMO_DISTRIBUTION", "node")
	if got := resolvedDistribution(); got != "node" {
		t.Fatalf("resolvedDistribution = %q", got)
	}
}
