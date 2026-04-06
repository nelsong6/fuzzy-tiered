package core

// Config holds all options derived from CLI flags.
// Rendering-specific fields (Layout, Border, Title, etc.) are ignored by core
// state functions but kept here to avoid splitting the struct prematurely.
type Config struct {
	Layout       string // "reverse" or "default"
	Border       bool
	HeaderLines  int
	Nth          []int // 1-based field indices for search scope
	AcceptNth    []int // 1-based field indices for output
	Prompt       string
	Delimiter    string
	Tiered       bool
	DepthPenalty int
	SearchCols   []int // 1-based, overrides Nth for scoring
	Height       int   // percentage of terminal height (0 = full)
	ShowScores   bool  // annotate filter output with scores
	ANSI         bool  // preserve ANSI colors from input
	Title        string
	TitlePos     string
	TreeMode     bool
	Label        string
}
