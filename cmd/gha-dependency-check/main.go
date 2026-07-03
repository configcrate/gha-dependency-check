package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	githubclient "github.com/configcrate/gha-dependency-check/internal/github"
	"github.com/configcrate/gha-dependency-check/internal/model"
	"github.com/configcrate/gha-dependency-check/internal/workflow"
)

var version = "dev"

type output struct {
	Version      string         `json:"version"`
	ScannedFiles int            `json:"scanned_files"`
	Dependencies int            `json:"dependencies"`
	Results      []model.Result `json:"results"`
	Summary      summary        `json:"summary"`
}

type summary struct {
	Healthy  int `json:"healthy"`
	Findings int `json:"findings"`
	Errors   int `json:"errors"`
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(arguments []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("gha-dependency-check", flag.ContinueOnError)
	flags.SetOutput(stderr)
	format := flags.String("format", "text", "output format: text or json")
	apiURL := flags.String("api-url", "", "GitHub API base URL (defaults to GITHUB_API_URL or api.github.com)")
	timeout := flags.Duration("timeout", 10*time.Second, "timeout for each GitHub API request")
	showVersion := flags.Bool("version", false, "print version and exit")
	flags.Usage = func() {
		fmt.Fprintln(stderr, "Usage: gha-dependency-check [options] [path]")
		fmt.Fprintln(stderr, "Scan a repository, workflow directory, or workflow file.")
		fmt.Fprintln(stderr)
		flags.PrintDefaults()
	}
	if err := flags.Parse(arguments); err != nil {
		return 2
	}
	if *showVersion {
		fmt.Fprintln(stdout, version)
		return 0
	}
	if *format != "text" && *format != "json" {
		fmt.Fprintln(stderr, "error: --format must be text or json")
		return 2
	}
	if *timeout <= 0 {
		fmt.Fprintln(stderr, "error: --timeout must be greater than zero")
		return 2
	}
	if flags.NArg() > 1 {
		flags.Usage()
		return 2
	}

	path := "."
	if flags.NArg() == 1 {
		path = flags.Arg(0)
	}
	scan, err := workflow.Scan(path)
	if err != nil {
		fmt.Fprintf(stderr, "scan error: %v\n", err)
		return 2
	}

	results := append([]model.Result{}, scan.Invalid...)
	client := githubclient.NewClient(*apiURL, *timeout)
	ctx := context.Background()
	cache := make(map[string]model.Result)
	for _, dependency := range scan.Dependencies {
		key := strings.Join(
			[]string{dependency.Owner, dependency.Repo, dependency.Path, dependency.Ref},
			"\x00",
		)
		checked, exists := cache[key]
		if !exists {
			checked = client.Check(ctx, dependency)
			cache[key] = checked
		} else {
			checked.Dependency = dependency
		}
		results = append(results, checked)
	}

	report := output{
		Version:      version,
		ScannedFiles: len(scan.Files),
		Dependencies: len(scan.Dependencies) + len(scan.Invalid),
		Results:      results,
	}
	for _, checked := range results {
		switch {
		case checked.Status == model.StatusHealthy:
			report.Summary.Healthy++
		case checked.IsOperationalError():
			report.Summary.Errors++
		default:
			report.Summary.Findings++
		}
	}

	if *format == "json" {
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(report); err != nil {
			fmt.Fprintf(stderr, "output error: %v\n", err)
			return 2
		}
	} else {
		printText(stdout, scan.Files, report)
	}

	if report.Summary.Errors > 0 {
		return 2
	}
	if report.Summary.Findings > 0 {
		return 1
	}
	return 0
}

func printText(writer io.Writer, files []string, report output) {
	fmt.Fprintf(
		writer,
		"gha-dependency-check: scanned %d workflow file(s), found %d remote dependency reference(s)\n",
		report.ScannedFiles,
		report.Dependencies,
	)
	for _, checked := range report.Results {
		location := checked.Dependency.File
		if relative, err := filepath.Rel(".", location); err == nil {
			location = relative
		}
		location = filepath.ToSlash(location)
		if checked.Dependency.Line > 0 {
			location = fmt.Sprintf("%s:%d", location, checked.Dependency.Line)
		}
		fmt.Fprintf(
			writer,
			"%-12s %s  %s  %s\n",
			strings.ToUpper(string(checked.Status)),
			safeText(checked.Dependency.Uses),
			safeText(location),
			safeText(checked.Message),
		)
	}
	fmt.Fprintf(
		writer,
		"summary: %d healthy, %d finding(s), %d API error(s)\n",
		report.Summary.Healthy,
		report.Summary.Findings,
		report.Summary.Errors,
	)
}

func safeText(value string) string {
	return strings.Map(func(character rune) rune {
		if unicode.IsControl(character) {
			return '?'
		}
		return character
	}, value)
}
