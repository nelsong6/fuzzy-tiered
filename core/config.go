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
	// Search depth limit — 0 = unlimited (default), N = only search N levels deep from current scope
	SearchDepth int
	// Frontend identity — populated by ecosystem layer, not the engine
	FrontendName     string
	FrontendVersion  string
	FrontendCommands []CommandItem
	// Provider for lazy tree loading (e.g. DirProvider for file picker mode)
	Provider TreeProvider
	// FocusedDir is a path to pre-expand when using a Provider (e.g. --focused-dir)
	FocusedDir string
}
