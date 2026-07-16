package cli

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"strings"

	"github.com/bjornt/my-workshop/internal/additions"
	"github.com/bjornt/my-workshop/internal/workshop"
	"github.com/bjornt/my-workshop/internal/worktree"
	"github.com/bjornt/my-workshop/internal/yamlconfig"
)

// Args holds the parsed command-line arguments.
type Args struct {
	Base   string
	Revert bool
	YAML   string
}

// ParseArgs parses the command line using the stdlib flag package with the
// same semantics as cli.py's build_parser().parse_args().
func ParseArgs(argv []string) (Args, error) {
	fs := flag.NewFlagSet("my-workshop", flag.ContinueOnError)
	var buf bytes.Buffer
	fs.SetOutput(&buf)

	var args Args
	fs.StringVar(&args.Base, "base", "", "Base image for a new workshop.yaml (default: additions config, or ubuntu@24.04)")
	fs.BoolVar(&args.Revert, "revert", false,
		"Stop ignoring the workshop YAML and exit without launching: "+
			"clears skip-worktree and restores a tracked file, or drops the local "+
			".git/info/exclude entry for an untracked one.")

	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "usage: %s [--base IMAGE] [--revert] [PATH]\n\n", fs.Name())
		fmt.Fprint(fs.Output(), "Bootstrap, ignore, and start a workshop environment.\n\n")
		fmt.Fprint(fs.Output(), "Wraps the standard 'workshop' launch flow so a project's git-tracked\n")
		fmt.Fprint(fs.Output(), "workshop YAML can be augmented with the SDKs this developer needs without\n")
		fmt.Fprint(fs.Output(), "ever appearing as a local modification in git.\n\n")
		fmt.Fprint(fs.Output(), "Use --revert to stop ignoring the YAML.\n\n")
		fmt.Fprint(fs.Output(), "Positional arguments:\n")
		fmt.Fprint(fs.Output(), "  PATH    Path to the workshop YAML file ")
		fmt.Fprint(fs.Output(), "(default: auto-detect workshop.yaml or a single file under .workshop/)\n\n")
		fmt.Fprint(fs.Output(), "Options:\n")
		fs.PrintDefaults()
	}

	// Python argparse allows flags to appear after positional arguments; the
	// stdlib flag package stops at the first non-flag by default. Reorder so
	// that flags and their values precede any positional PATH argument.
	ordered := intersperseFlags(argv)

	if err := fs.Parse(ordered); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			// argparse prints help to stdout and exits 0.
			fmt.Print(buf.String())
			return Args{}, flag.ErrHelp
		}
		// argparse prints "usage… \n prog: error: …" to stderr; surface the
		// captured flag output as the error message so Run/main can emit it.
		return Args{}, errors.New(strings.TrimRight(buf.String(), "\n"))
	}
	args.YAML = fs.Arg(0)
	return args, nil
}

// intersperseFlags moves all flag arguments (and their associated values)
// before positional arguments so that --revert after PATH is handled like
// Python argparse.
func intersperseFlags(argv []string) []string {
	var flags, positional []string
	for i := 0; i < len(argv); i++ {
		arg := argv[i]
		if !strings.HasPrefix(arg, "-") {
			positional = append(positional, arg)
			continue
		}

		flags = append(flags, arg)
		// The base flag consumes the next argument when it is not written
		// as --base=value.
		if (arg == "--base" || arg == "-base") && i+1 < len(argv) {
			flags = append(flags, argv[i+1])
			i++
		}
	}
	return append(flags, positional...)
}

// Run implements the main entry point of the CLI.
func Run(argv []string, ws workshop.Workshop, log yamlconfig.Logger) error {
	prog := "my-workshop"

	args, err := ParseArgs(argv)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil // help already printed to stdout
		}
		return err
	}

	path, err := yamlconfig.FindYAML(args.YAML)
	if err != nil {
		return err
	}

	additionsPath := additions.FindAdditions(path)

	if args.Revert {
		worktree.Revert(path, prog, worktree.Logger(log))
		if additionsPath != "" {
			worktree.Revert(additionsPath, prog, worktree.Logger(log))
		}
		return nil
	}

	if additionsPath != "" {
		log("Using additions config: " + additionsPath)
	} else {
		log("No additions config found; running as noop.")
	}

	cfg, _ := additions.LoadAdditions(path)
	base := args.Base
	if base == "" {
		base = cfg.Base
	}
	if base == "" {
		base = "ubuntu@24.04"
	}

	if err := yamlconfig.EnsureYAML(path, base, cfg.SDKs, log); err != nil {
		return err
	}

	worktree.HideInWorktree(path, prog, worktree.Logger(log))
	if additionsPath != "" {
		worktree.HideInWorktree(additionsPath, prog, worktree.Logger(log))
	}

	var backend workshop.Workshop = ws
	if backend == nil {
		backend = workshop.NewRealWorkshop(workshop.Logger(log))
	}

	host, err := workshop.Provision(backend, cfg.Provision)
	if err != nil {
		return err
	}

	log("\nTo connect, use 'workshop shell' or 'ssh workshop@" + host + "'")
	return nil
}
