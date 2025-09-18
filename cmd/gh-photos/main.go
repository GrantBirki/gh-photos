package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/grantbirki/gh-photos/internal/version"
)

var (
	noColor        = flag.Bool("no-color", false, "disable colored output")
	dryRun         = flag.Bool("dry-run", false, "preview changes without writing files")
	recursive      = flag.Bool("recursive", true, "scan directories recursively")
	pervasive      = flag.Bool("pervasive", false, "scan all YAML files, not just docker-compose files")
	expandRegistry = flag.Bool("expand-registry", false, "expand short image names to fully qualified registry names")
	showVersion    = flag.Bool("version", false, "show version information")
	algo           = flag.String("algo", "sha256", "digest algorithm to check for (sha256, sha512, etc.)")
	forceMode      = flag.String("mode", "", "force processing mode: 'docker' for containers only, 'actions' for GitHub Actions only")
	quiet          = flag.Bool("quiet", false, "suppress informational messages when no changes are needed")
	platform       = flag.String("platform", "", "pin to platform-specific manifest digest (e.g., linux/amd64, linux/arm/v7)")
)

func main() {
	flag.Parse()

	// Show version and exit if requested
	if *showVersion {
		fmt.Println(version.String())
		os.Exit(0)
	}

	// Disable colors if requested
	if *noColor {
		color.NoColor = true
	}

	if len(flag.Args()) == 0 {
		fmt.Fprintf(os.Stderr, "Usage: %s [--version] [--dry-run] [--no-color] TODO...\n", os.Args[0])
		os.Exit(1)
	}
}
