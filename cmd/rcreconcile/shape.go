package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
)

// runShape walks a captured /bulk JSON file and prints its key-path structure
// - field names and JSON types only, never values - so a human can safely
// share it (no PII, no secrets) to correct bulkEntries in rcmodel.go against
// the real schema.
func runShape(argv []string) error {
	fs := flag.NewFlagSet("shape", flag.ContinueOnError)
	var bulkFile, out string
	fs.StringVar(&bulkFile, "bulk-file", "", "path to a captured /bulk (or similar) JSON file (required)")
	fs.StringVar(&out, "out", "", "also write the shape listing to this path")
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, "usage: rcreconcile shape --bulk-file PATH [--out PATH]\n\nflags:\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(argv); err != nil {
		return err
	}
	if bulkFile == "" {
		return fmt.Errorf("shape: --bulk-file is required")
	}

	b, err := os.ReadFile(bulkFile)
	if err != nil {
		return fmt.Errorf("read %q: %w", bulkFile, err)
	}
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return fmt.Errorf("decode %q: %w", bulkFile, err)
	}

	shape := map[string]string{}
	collectShape(v, "", shape)

	paths := make([]string, 0, len(shape))
	for p := range shape {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	var sb strings.Builder
	for _, p := range paths {
		fmt.Fprintf(&sb, "%s: %s\n", p, shape[p])
	}

	fmt.Print(sb.String())
	if out == "" {
		return nil
	}
	if err := os.WriteFile(out, []byte(sb.String()), 0o644); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "wrote", out)
	return nil
}

// collectShape records, for every leaf reachable from v, its dotted path and
// JSON type - "object{}" / "array{}" for empty containers, never a value.
// Arrays are sampled at their first element only: index [0] stands in for
// "array of this shape", which is enough to describe the structure without
// walking every element of e.g. a large /bulk entries array.
func collectShape(v any, path string, out map[string]string) {
	switch t := v.(type) {
	case map[string]any:
		if len(t) == 0 {
			out[label(path)] = "object{}"
			return
		}
		for k, child := range t {
			p := k
			if path != "" {
				p = path + "." + k
			}
			collectShape(child, p, out)
		}
	case []any:
		p := path + "[]"
		if len(t) == 0 {
			out[label(p)] = "array{}"
			return
		}
		collectShape(t[0], p, out)
	case string:
		out[label(path)] = "string"
	case float64:
		out[label(path)] = "number"
	case bool:
		out[label(path)] = "bool"
	case nil:
		out[label(path)] = "null"
	}
}

func label(path string) string {
	if path == "" {
		return "."
	}
	return path
}
