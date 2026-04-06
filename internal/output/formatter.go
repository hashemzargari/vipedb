package output

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/fatih/color"
	"github.com/hashemzargari/vipedb/pkg/vector"
)

type SearchOutput struct {
	Rank       int     `json:"rank"`
	Text       string  `json:"text"`
	Score      float32 `json:"score"`
	Source     string  `json:"source,omitempty"`
	DocumentID string  `json:"document_id"`
	Timestamp  string  `json:"timestamp,omitempty"`
}

// PrintSearchResults renders search results as colored terminal output or strict JSON.
func PrintSearchResults(w io.Writer, results []vector.SearchResult, jsonMode bool) {
	if jsonMode {
		printJSON(w, results)
		return
	}
	printColored(w, results)
}

// PrintGrepResults renders grep results as colored grep-style output or strict JSON.
func PrintGrepResults(w io.Writer, results []vector.SearchResult, jsonMode bool) {
	if jsonMode {
		printJSON(w, results)
		return
	}
	printGrepColored(w, results)
}

func printJSON(w io.Writer, results []vector.SearchResult) {
	outputs := make([]SearchOutput, len(results))
	for i, r := range results {
		outputs[i] = SearchOutput{
			Rank:       i + 1,
			Text:       r.Content,
			Score:      r.Score,
			Source:     r.Metadata["source"],
			DocumentID: r.DocumentID,
			Timestamp:  r.Metadata["timestamp"],
		}
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.Encode(outputs)
}

func scoreColor(score float32) *color.Color {
	if score >= 0.75 {
		return color.New(color.FgGreen, color.Bold)
	}
	if score >= 0.50 {
		return color.New(color.FgYellow, color.Bold)
	}
	return color.New(color.FgRed, color.Bold)
}

func printColored(w io.Writer, results []vector.SearchResult) {
	rankC := color.New(color.FgWhite, color.Bold)
	sourceC := color.New(color.FgCyan)
	contentC := color.New(color.FgWhite)
	dimC := color.New(color.FgHiBlack)

	for i, r := range results {
		source := r.Metadata["source"]
		ts := r.Metadata["timestamp"]
		sc := scoreColor(r.Score)

		rankC.Fprintf(w, "  %d. ", i+1)
		sc.Fprintf(w, "[Score: %.4f] ", r.Score)
		if source != "" {
			sourceC.Fprintf(w, "%s", source)
		}
		fmt.Fprintln(w)

		if ts != "" {
			dimC.Fprintf(w, "     %s\n", ts)
		}

		contentC.Fprintf(w, "     %s\n", r.Content)
		fmt.Fprintln(w)
	}
}

func printGrepColored(w io.Writer, results []vector.SearchResult) {
	sourceC := color.New(color.FgCyan)
	sepC := color.New(color.FgHiBlack)
	contentC := color.New(color.FgWhite)

	for _, r := range results {
		source := r.Metadata["source"]
		sc := scoreColor(r.Score)

		if source != "" {
			sourceC.Fprintf(w, "%s", source)
			sepC.Fprintf(w, ":")
		}
		sc.Fprintf(w, "[%.4f]", r.Score)
		sepC.Fprintf(w, ":")
		contentC.Fprintf(w, "%s\n", r.Content)
	}
}
