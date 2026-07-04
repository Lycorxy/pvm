package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/pvm/pvm/internal/config"
	"github.com/pvm/pvm/internal/registry"
)

func runListRemote(args []string) error {
	if len(args) == 0 {
		fmt.Println("Usage: pvm list-remote <runtime> [--all] [--mirror]")
		fmt.Println("Runtimes:", strings.Join(config.SupportedRuntimes, ", "))
		return nil
	}

	rt := strings.ToLower(args[0])
	if !config.IsSupportedRuntime(rt) {
		return fmt.Errorf("unsupported runtime: %s, supported: %s",
			rt, strings.Join(config.SupportedRuntimes, ", "))
	}

	showAll := false
	useMirror := false
	for _, arg := range args[1:] {
		switch arg {
		case "--all", "-a":
			showAll = true
		case "--mirror", "-m":
			useMirror = true
		}
	}

	fmt.Fprintf(os.Stderr, "  → Fetching available %s versions...\n", rt)

	versions, err := registry.ListRemoteVersions(rt, useMirror)
	if err != nil {
		return fmt.Errorf("fetch remote versions: %w", err)
	}

	if len(versions) == 0 {
		fmt.Printf("  No versions found for %s\n", rt)
		return nil
	}

	limit := 20
	if showAll {
		limit = len(versions)
	}
	if limit > len(versions) {
		limit = len(versions)
	}

	fmt.Printf("  %s (showing %d of %d):\n", rt, limit, len(versions))
	for i := 0; i < limit; i++ {
		v := versions[i]
		extra := ""
		if v.LTS {
			extra = " (LTS)"
		}
		if v.Date != "" {
			extra += fmt.Sprintf(" [%s]", v.Date)
		}
		fmt.Printf("    %s%s\n", v.Version, extra)
	}

	if !showAll && len(versions) > limit {
		fmt.Printf("\n  ... and %d more. Use --all to see all versions.\n", len(versions)-limit)
	}

	return nil
}
