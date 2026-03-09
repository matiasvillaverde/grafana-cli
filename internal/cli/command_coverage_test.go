package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

type commandCoverageManifest struct {
	Shards []commandCoverageShard `json:"shards"`
}

type commandCoverageShard struct {
	Name     string   `json:"name"`
	Test     string   `json:"test"`
	Commands []string `json:"commands"`
}

func TestIntegrationCommandCoverageManifest(t *testing.T) {
	manifestPath := filepath.Join("..", "..", "test", "integration", "command-coverage.json")
	workflowPath := filepath.Join("..", "..", ".github", "workflows", "integration.yml")
	scriptRoot := filepath.Join("..", "..", "cmd", "grafana", "testdata", "integration")

	var manifest commandCoverageManifest
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read command coverage manifest: %v", err)
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("decode command coverage manifest: %v", err)
	}
	if len(manifest.Shards) == 0 {
		t.Fatal("command coverage manifest has no shards")
	}

	workflowData, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatalf("read integration workflow: %v", err)
	}
	workflow := string(workflowData)

	shardNames := map[string]struct{}{}
	covered := map[string]string{}
	for _, shard := range manifest.Shards {
		if shard.Name == "" {
			t.Fatal("command coverage manifest has shard with empty name")
		}
		if shard.Test == "" {
			t.Fatalf("command coverage manifest shard %q has empty test name", shard.Name)
		}
		if _, ok := shardNames[shard.Name]; ok {
			t.Fatalf("duplicate shard name in command coverage manifest: %s", shard.Name)
		}
		shardNames[shard.Name] = struct{}{}
		if len(shard.Commands) == 0 {
			t.Fatalf("command coverage manifest shard %q has no commands", shard.Name)
		}

		scriptPath := filepath.Join(scriptRoot, shard.Name, "workflow.txtar")
		if _, err := os.Stat(scriptPath); err != nil {
			t.Fatalf("missing integration script for shard %q: %v", shard.Name, err)
		}
		if !strings.Contains(workflow, shard.Name) {
			t.Fatalf("integration workflow does not reference shard name %q", shard.Name)
		}
		if !strings.Contains(workflow, shard.Test) {
			t.Fatalf("integration workflow does not reference shard test %q", shard.Test)
		}

		for _, command := range shard.Commands {
			if command == "" {
				t.Fatalf("command coverage manifest shard %q has empty command entry", shard.Name)
			}
			if prev, ok := covered[command]; ok {
				t.Fatalf("command %q is assigned to multiple shards: %s and %s", command, prev, shard.Name)
			}
			covered[command] = shard.Name
		}
	}

	discovered := map[string]struct{}{}
	for _, item := range flattenDiscoveryPaths(discoveryCatalog(), nil) {
		if len(item.command.Subcommands) > 0 {
			continue
		}
		discovered[strings.Join(item.path, " ")] = struct{}{}
	}

	missing := make([]string, 0)
	for command := range discovered {
		if _, ok := covered[command]; !ok {
			missing = append(missing, command)
		}
	}

	extra := make([]string, 0)
	for command := range covered {
		if _, ok := discovered[command]; !ok {
			extra = append(extra, command)
		}
	}

	sort.Strings(missing)
	sort.Strings(extra)

	if len(missing) > 0 || len(extra) > 0 {
		t.Fatalf("command coverage manifest drifted; missing=%v extra=%v", missing, extra)
	}
}
