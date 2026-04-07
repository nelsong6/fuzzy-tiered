package core

import "github.com/gdamore/tcell/v2"

// Semantic color constants for all fzt renderers.
// The terminal frontend uses these directly as tcell colors.
// The web frontend maps equivalent roles via CSS custom properties.
//
// CSS equivalents (Catppuccin Mocha palette):
//   SelectionBg    → palette[4] blue     → #89b4fa
//   HighlightFg    → palette[2] green    → #a6e3a1
//   FolderIconFg   → palette[3] yellow   → #f9e2af
//   FolderNameFg   → palette[6] cyan     → #94e2d5
//   FileIconFg     → palette[7] white    → #bac2de
//   BorderFg       → palette[8] gray     → #585b70
//   HintFg         → palette[8] gray     → #585b70
//   PromptActiveFg → palette[3] yellow   → #f9e2af
//   QueryFg        → palette[7] white    → #bac2de
//   GhostFg        → palette[8] gray     → #585b70
//   HeaderFg       → palette[6] cyan     → #94e2d5
//   NavModeFg      → palette[6] cyan     → #94e2d5
//   SearchModeFg   → palette[3] yellow   → #f9e2af
//   PromptSurfaceBg → 256-color #236     → #303030
//   TitleFg        → palette[6] cyan     → #94e2d5
var (
	SelectionBg    = tcell.ColorDarkBlue
	HighlightFg    = tcell.ColorGreen
	FolderIconFg   = tcell.ColorYellow
	FolderNameFg   = tcell.ColorDarkCyan
	FileIconFg     = tcell.ColorWhite
	BorderFg       = tcell.ColorDarkGray
	HintFg         = tcell.ColorDarkGray
	PromptActiveFg = tcell.ColorYellow
	QueryFg        = tcell.ColorWhite
	GhostFg        = tcell.ColorDarkGray
	HeaderFg       = tcell.ColorDarkCyan
	NavModeFg      = tcell.ColorDarkCyan
	SearchModeFg   = tcell.ColorYellow
	TitleFg        = tcell.ColorDarkCyan
	PromptSurfaceBg = tcell.ColorValid + 236 // 256-color: #303030
	SeparatorFg    = tcell.ColorDarkGray
	BreadcrumbFg   = tcell.ColorDarkCyan
	VersionFg      = tcell.ColorDarkGray
	LabelFg        = tcell.ColorDarkGray
	PathFg         = tcell.ColorDarkGray
)
