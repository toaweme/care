// Command care reports repository health and scaffolds config files for the current repo's
// ecosystem.
package main

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/toaweme/cli"
	"github.com/toaweme/http"

	"github.com/toaweme/care/cmd/care/output"
	"github.com/toaweme/care/eco/shared/sync"
)

// GetConfig drives the generic `care get` file sync: fetch a file from a source and
// write it into the repo, optionally rewriting placeholder tokens on the way in.
type GetConfig struct {
	Source  string   `arg:"0" env:"CARE_GET_SOURCE" help:"Source to fetch: a local path, a github/gist url, or the owner/repo/path shorthand"`
	Out     string   `arg:"out" short:"o" env:"CARE_GET_OUT" help:"Destination path relative to cwd; defaults to the source's filename"`
	Token   string   `arg:"token" short:"t" env:"GITHUB_TOKEN" help:"GitHub token for private sources; defaults to the GITHUB_TOKEN env"`
	Replace []string `arg:"r" short:"r" sep:"" env:"CARE_GET_REPLACE" help:"Replace a placeholder with a value as token=value. Repeat -r for more; the value may contain any character, the token may not contain '='"`
	Force   bool     `arg:"force" env:"CARE_GET_FORCE" default:"false" help:"Overwrite an existing destination file"`
}

// GetCommand fetches one file from a source and writes it into the current repo,
// applying any -r placeholder substitutions before the write.
type GetCommand struct {
	cli.BaseCommand[GetConfig]
	client http.Client
}

var _ cli.Command[GetConfig] = (*GetCommand)(nil)

// NewGetCommand builds the get command from the http client.
func NewGetCommand(client http.Client) *GetCommand {
	return &GetCommand{
		BaseCommand: cli.NewBaseCommand[GetConfig](),
		client:      client,
	}
}

// Run fetches the source, applies placeholder substitutions, and writes the result.
func (c *GetCommand) Run(options cli.GlobalFlags, _ cli.Unknowns) error {
	if c.Inputs.Source == "" {
		return fmt.Errorf("a source is required: care get <owner/repo/path> [--out <path>]")
	}

	replacements, err := parseReplacements(c.Inputs.Replace)
	if err != nil {
		return err
	}

	out := c.Inputs.Out
	if out == "" {
		out = filepath.Base(c.Inputs.Source)
	}
	dest := resolveDest(options.Cwd, out)

	engine := sync.NewEngine(sync.NewFetcher(c.client, c.Inputs.Token))
	src, err := engine.Resolve(c.Inputs.Source, filepath.Base(dest))
	if err != nil {
		return fmt.Errorf("failed to resolve source %q: %w", c.Inputs.Source, err)
	}
	content, err := engine.Bytes(context.Background(), src)
	if err != nil {
		return fmt.Errorf("failed to fetch %s: %w", c.Inputs.Source, err)
	}
	content = applyReplacements(content, replacements)

	wrote, err := sync.WriteFile(dest, content, c.Inputs.Force)
	if err != nil {
		return fmt.Errorf("failed to write %s: %w", dest, err)
	}
	reportSync(sync.Result{Dest: dest, Source: src.String(), Bytes: len(content), Skipped: !wrote})
	return nil
}

// Help returns the get command's usage text.
func (c *GetCommand) Help() string {
	return "Fetch a config file into the current repo from a source (owner/repo/path, a github/gist url, or a local path). Rewrite placeholder tokens with -r token=value; the destination defaults to the source's filename unless --out is given."
}

// replacement is one placeholder-to-value substitution applied to fetched bytes.
type replacement struct {
	token string
	value string
}

// parseReplacements parses each "token=value" entry into a replacement, splitting
// on the first "=" so a value may itself contain "=". An entry without an "=" or
// with an empty token is a usage error.
func parseReplacements(entries []string) ([]replacement, error) {
	out := make([]replacement, 0, len(entries))
	for _, e := range entries {
		token, value, ok := strings.Cut(e, "=")
		if !ok || token == "" {
			return nil, fmt.Errorf("invalid replacement %q: expected token=value", e)
		}
		out = append(out, replacement{token: token, value: value})
	}
	return out, nil
}

// applyReplacements rewrites every occurrence of each token in content, in order.
func applyReplacements(content []byte, replacements []replacement) []byte {
	if len(replacements) == 0 {
		return content
	}
	s := string(content)
	for _, r := range replacements {
		s = strings.ReplaceAll(s, r.token, r.value)
	}
	return []byte(s)
}

// resolveDest joins a destination relative to cwd, leaving absolute paths as-is.
func resolveDest(cwd, out string) string {
	if filepath.IsAbs(out) {
		return out
	}
	return filepath.Join(cwd, out)
}

// reportSync prints the outcome of a sync in the shared get style.
func reportSync(res sync.Result) {
	if res.Skipped {
		fmt.Printf("%s %s already exists; pass --force to overwrite\n", output.WarnStyle.Render("•"), res.Dest)
		return
	}
	fmt.Printf("%s wrote %s\n", output.OKStyle.Render("✓"), res.Dest)
	fmt.Printf("%s\n", output.DimStyle.Render("source: "+res.Source))
}
