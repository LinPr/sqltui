package theme

import (
	"sort"
	"strings"
	"sync"
)

// DefaultName is the name of the palette used when no theme is configured.
const DefaultName = "sorbet"

// builtins holds every built-in palette keyed by its kebab-case name.
// Except for sorbet and aurora (original palettes tuned for this project),
// all hex values are taken from the published specification of each scheme.
var builtins = map[string]Palette{
	// sorbet is the default: warm pastel foregrounds — yellow and orange
	// leading, sky blue and mint supporting — kept bright and soft so they
	// stay friendly on any dark terminal background (the UI itself is
	// transparent, so Bg/BgSoft only back the stripe/status-bar fills with
	// a subtle warm tint).
	"sorbet": {
		Name:   "sorbet",
		Bg:     "#14110c",
		Fg:     "#f4f1ea",
		BgSoft: "#262117",
		FgDim:  "#a9a493",

		Header:    "#ffd97a",
		Accent:    "#ffb473",
		AccentFg:  "#2b1c0c",
		Highlight: "#ffe066",

		Error:   "#ff9494",
		Warning: "#ffc46b",
		Success: "#a5e8a5",

		Series: []string{"#ffd97a", "#ffb473", "#8fd3ff", "#a5e8a5", "#ffb3c6", "#cbbcff"},
	},
	// aurora: bright, high-saturation foregrounds and accents designed to
	// stay vivid on any dark terminal background.
	"aurora": {
		Name:   "aurora",
		Bg:     "#101319",
		Fg:     "#eef1f8",
		BgSoft: "#1d2331",
		FgDim:  "#8b93a7",

		Header:    "#4fc3f7",
		Accent:    "#4f8cff",
		AccentFg:  "#0b0e14",
		Highlight: "#ffd75f",

		Error:   "#ff5370",
		Warning: "#ffb454",
		Success: "#4ade80",

		Series: []string{"#4f9cff", "#ff5fd2", "#3ddc84", "#ffa14f", "#33d6e8", "#a78bff"},
	},
	"catppuccin-mocha": {
		Name:   "catppuccin-mocha",
		Bg:     "#1e1e2e",
		Fg:     "#cdd6f4",
		BgSoft: "#313244",
		FgDim:  "#6c7086",

		Header:    "#cba6f7",
		Accent:    "#89b4fa",
		AccentFg:  "#1e1e2e",
		Highlight: "#f9e2af",

		Error:   "#f38ba8",
		Warning: "#f9e2af",
		Success: "#a6e3a1",

		Series: []string{"#89b4fa", "#cba6f7", "#a6e3a1", "#fab387", "#94e2d5", "#f5c2e7"},
	},
	"catppuccin-macchiato": {
		Name:   "catppuccin-macchiato",
		Bg:     "#24273a",
		Fg:     "#cad3f5",
		BgSoft: "#363a4f",
		FgDim:  "#6e738d",

		Header:    "#c6a0f6",
		Accent:    "#8aadf4",
		AccentFg:  "#24273a",
		Highlight: "#eed49f",

		Error:   "#ed8796",
		Warning: "#eed49f",
		Success: "#a6da95",

		Series: []string{"#8aadf4", "#c6a0f6", "#a6da95", "#f5a97f", "#8bd5ca", "#f5bde6"},
	},
	"catppuccin-frappe": {
		Name:   "catppuccin-frappe",
		Bg:     "#303446",
		Fg:     "#c6d0f5",
		BgSoft: "#414559",
		FgDim:  "#737994",

		Header:    "#ca9ee6",
		Accent:    "#8caaee",
		AccentFg:  "#303446",
		Highlight: "#e5c890",

		Error:   "#e78284",
		Warning: "#e5c890",
		Success: "#a6d189",

		Series: []string{"#8caaee", "#ca9ee6", "#a6d189", "#ef9f76", "#81c8be", "#f4b8e4"},
	},
	"catppuccin-latte": {
		Name:   "catppuccin-latte",
		Bg:     "#eff1f5",
		Fg:     "#4c4f69",
		BgSoft: "#ccd0da",
		FgDim:  "#9ca0b0",

		Header:    "#8839ef",
		Accent:    "#1e66f5",
		AccentFg:  "#eff1f5",
		Highlight: "#df8e1d",

		Error:   "#d20f39",
		Warning: "#df8e1d",
		Success: "#40a02b",

		Series: []string{"#1e66f5", "#8839ef", "#40a02b", "#fe640b", "#179299", "#ea76cb"},
	},
	"dracula": {
		Name:   "dracula",
		Bg:     "#282a36",
		Fg:     "#f8f8f2",
		BgSoft: "#44475a",
		FgDim:  "#6272a4",

		Header:    "#bd93f9",
		Accent:    "#bd93f9",
		AccentFg:  "#282a36",
		Highlight: "#f1fa8c",

		Error:   "#ff5555",
		Warning: "#ffb86c",
		Success: "#50fa7b",

		Series: []string{"#bd93f9", "#ff79c6", "#8be9fd", "#50fa7b", "#ffb86c", "#f1fa8c"},
	},
	"nord": {
		Name:   "nord",
		Bg:     "#2e3440",
		Fg:     "#d8dee9",
		BgSoft: "#3b4252",
		FgDim:  "#4c566a",

		Header:    "#88c0d0",
		Accent:    "#88c0d0",
		AccentFg:  "#2e3440",
		Highlight: "#ebcb8b",

		Error:   "#bf616a",
		Warning: "#ebcb8b",
		Success: "#a3be8c",

		Series: []string{"#88c0d0", "#81a1c1", "#a3be8c", "#d08770", "#b48ead", "#ebcb8b"},
	},
	"gruvbox-dark": {
		Name:   "gruvbox-dark",
		Bg:     "#282828",
		Fg:     "#ebdbb2",
		BgSoft: "#3c3836",
		FgDim:  "#928374",

		Header:    "#fabd2f",
		Accent:    "#fe8019",
		AccentFg:  "#282828",
		Highlight: "#fabd2f",

		Error:   "#fb4934",
		Warning: "#fabd2f",
		Success: "#b8bb26",

		Series: []string{"#83a598", "#fabd2f", "#b8bb26", "#fe8019", "#d3869b", "#8ec07c"},
	},
	"gruvbox-light": {
		Name:   "gruvbox-light",
		Bg:     "#fbf1c7",
		Fg:     "#3c3836",
		BgSoft: "#ebdbb2",
		FgDim:  "#928374",

		Header:    "#b57614",
		Accent:    "#d65d0e",
		AccentFg:  "#fbf1c7",
		Highlight: "#b57614",

		Error:   "#9d0006",
		Warning: "#b57614",
		Success: "#79740e",

		Series: []string{"#076678", "#b57614", "#79740e", "#af3a03", "#8f3f71", "#427b58"},
	},
	"solarized-dark": {
		Name:   "solarized-dark",
		Bg:     "#002b36",
		Fg:     "#839496",
		BgSoft: "#073642",
		FgDim:  "#586e75",

		Header:    "#93a1a1",
		Accent:    "#268bd2",
		AccentFg:  "#002b36",
		Highlight: "#b58900",

		Error:   "#dc322f",
		Warning: "#b58900",
		Success: "#859900",

		Series: []string{"#268bd2", "#2aa198", "#859900", "#b58900", "#d33682", "#6c71c4"},
	},
	"solarized-light": {
		Name:   "solarized-light",
		Bg:     "#fdf6e3",
		Fg:     "#657b83",
		BgSoft: "#eee8d5",
		FgDim:  "#93a1a1",

		Header:    "#586e75",
		Accent:    "#268bd2",
		AccentFg:  "#fdf6e3",
		Highlight: "#b58900",

		Error:   "#dc322f",
		Warning: "#b58900",
		Success: "#859900",

		Series: []string{"#268bd2", "#2aa198", "#859900", "#b58900", "#d33682", "#6c71c4"},
	},
	"tokyo-night": {
		Name:   "tokyo-night",
		Bg:     "#1a1b26",
		Fg:     "#c0caf5",
		BgSoft: "#292e42",
		FgDim:  "#565f89",

		Header:    "#7aa2f7",
		Accent:    "#7aa2f7",
		AccentFg:  "#1a1b26",
		Highlight: "#e0af68",

		Error:   "#f7768e",
		Warning: "#e0af68",
		Success: "#9ece6a",

		Series: []string{"#7aa2f7", "#bb9af7", "#9ece6a", "#ff9e64", "#7dcfff", "#f7768e"},
	},
	"tokyo-night-storm": {
		Name:   "tokyo-night-storm",
		Bg:     "#24283b",
		Fg:     "#c0caf5",
		BgSoft: "#2f344d",
		FgDim:  "#565f89",

		Header:    "#7aa2f7",
		Accent:    "#7aa2f7",
		AccentFg:  "#24283b",
		Highlight: "#e0af68",

		Error:   "#f7768e",
		Warning: "#e0af68",
		Success: "#9ece6a",

		Series: []string{"#7aa2f7", "#bb9af7", "#9ece6a", "#ff9e64", "#7dcfff", "#f7768e"},
	},
	"monokai": {
		Name:   "monokai",
		Bg:     "#272822",
		Fg:     "#f8f8f2",
		BgSoft: "#3e3d32",
		FgDim:  "#75715e",

		Header:    "#66d9ef",
		Accent:    "#66d9ef",
		AccentFg:  "#272822",
		Highlight: "#e6db74",

		Error:   "#f92672",
		Warning: "#fd971f",
		Success: "#a6e22e",

		Series: []string{"#66d9ef", "#a6e22e", "#f92672", "#fd971f", "#ae81ff", "#e6db74"},
	},
	"one-dark": {
		Name:   "one-dark",
		Bg:     "#282c34",
		Fg:     "#abb2bf",
		BgSoft: "#2c313c",
		FgDim:  "#5c6370",

		Header:    "#61afef",
		Accent:    "#61afef",
		AccentFg:  "#282c34",
		Highlight: "#e5c07b",

		Error:   "#e06c75",
		Warning: "#e5c07b",
		Success: "#98c379",

		Series: []string{"#61afef", "#c678dd", "#98c379", "#d19a66", "#56b6c2", "#e06c75"},
	},
	"one-light": {
		Name:   "one-light",
		Bg:     "#fafafa",
		Fg:     "#383a42",
		BgSoft: "#f0f0f1",
		FgDim:  "#a0a1a7",

		Header:    "#4078f2",
		Accent:    "#4078f2",
		AccentFg:  "#fafafa",
		Highlight: "#c18401",

		Error:   "#e45649",
		Warning: "#c18401",
		Success: "#50a14f",

		Series: []string{"#4078f2", "#a626a4", "#50a14f", "#986801", "#0184bc", "#e45649"},
	},
	"github-dark": {
		Name:   "github-dark",
		Bg:     "#0d1117",
		Fg:     "#c9d1d9",
		BgSoft: "#161b22",
		FgDim:  "#8b949e",

		Header:    "#58a6ff",
		Accent:    "#1f6feb",
		AccentFg:  "#ffffff",
		Highlight: "#d29922",

		Error:   "#f85149",
		Warning: "#d29922",
		Success: "#3fb950",

		Series: []string{"#58a6ff", "#bc8cff", "#3fb950", "#db6d28", "#39c5cf", "#ff7b72"},
	},
	"github-light": {
		Name:   "github-light",
		Bg:     "#ffffff",
		Fg:     "#24292f",
		BgSoft: "#f6f8fa",
		FgDim:  "#57606a",

		Header:    "#0969da",
		Accent:    "#0969da",
		AccentFg:  "#ffffff",
		Highlight: "#9a6700",

		Error:   "#cf222e",
		Warning: "#9a6700",
		Success: "#1a7f37",

		Series: []string{"#0969da", "#8250df", "#1a7f37", "#bc4c00", "#1b7c83", "#cf222e"},
	},
	"ayu-dark": {
		Name:   "ayu-dark",
		Bg:     "#0b0e14",
		Fg:     "#bfbdb6",
		BgSoft: "#131721",
		FgDim:  "#565b66",

		Header:    "#59c2ff",
		Accent:    "#e6b450",
		AccentFg:  "#0b0e14",
		Highlight: "#ffb454",

		Error:   "#f07178",
		Warning: "#ffb454",
		Success: "#aad94c",

		Series: []string{"#59c2ff", "#ffb454", "#aad94c", "#f07178", "#d2a6ff", "#95e6cb"},
	},
	"ayu-mirage": {
		Name:   "ayu-mirage",
		Bg:     "#1f2430",
		Fg:     "#cbccc6",
		BgSoft: "#232834",
		FgDim:  "#707a8c",

		Header:    "#5ccfe6",
		Accent:    "#ffcc66",
		AccentFg:  "#1f2430",
		Highlight: "#ffd580",

		Error:   "#ff6666",
		Warning: "#ffa759",
		Success: "#bae67e",

		Series: []string{"#5ccfe6", "#ffcc66", "#bae67e", "#ffa759", "#d4bfff", "#f28779"},
	},
	"ayu-light": {
		Name:   "ayu-light",
		Bg:     "#fafafa",
		Fg:     "#5c6166",
		BgSoft: "#f3f4f5",
		FgDim:  "#8a9199",

		Header:    "#55b4d4",
		Accent:    "#ff9940",
		AccentFg:  "#fafafa",
		Highlight: "#f2ae49",

		Error:   "#f07171",
		Warning: "#fa8d3e",
		Success: "#86b300",

		Series: []string{"#55b4d4", "#fa8d3e", "#86b300", "#a37acc", "#4cbf99", "#f07171"},
	},
	"rose-pine": {
		Name:   "rose-pine",
		Bg:     "#191724",
		Fg:     "#e0def4",
		BgSoft: "#1f1d2e",
		FgDim:  "#6e6a86",

		Header:    "#c4a7e7",
		Accent:    "#c4a7e7",
		AccentFg:  "#191724",
		Highlight: "#f6c177",

		Error:   "#eb6f92",
		Warning: "#f6c177",
		Success: "#9ccfd8",

		Series: []string{"#c4a7e7", "#ebbcba", "#9ccfd8", "#f6c177", "#31748f", "#eb6f92"},
	},
	"rose-pine-moon": {
		Name:   "rose-pine-moon",
		Bg:     "#232136",
		Fg:     "#e0def4",
		BgSoft: "#2a273f",
		FgDim:  "#6e6a86",

		Header:    "#c4a7e7",
		Accent:    "#c4a7e7",
		AccentFg:  "#232136",
		Highlight: "#f6c177",

		Error:   "#eb6f92",
		Warning: "#f6c177",
		Success: "#9ccfd8",

		Series: []string{"#c4a7e7", "#ea9a97", "#9ccfd8", "#f6c177", "#3e8fb0", "#eb6f92"},
	},
	"rose-pine-dawn": {
		Name:   "rose-pine-dawn",
		Bg:     "#faf4ed",
		Fg:     "#575279",
		BgSoft: "#f2e9e1",
		FgDim:  "#9893a5",

		Header:    "#907aa9",
		Accent:    "#907aa9",
		AccentFg:  "#faf4ed",
		Highlight: "#ea9d34",

		Error:   "#b4637a",
		Warning: "#ea9d34",
		Success: "#56949f",

		Series: []string{"#907aa9", "#d7827e", "#56949f", "#ea9d34", "#286983", "#b4637a"},
	},
	"everforest-dark": {
		Name:   "everforest-dark",
		Bg:     "#2d353b",
		Fg:     "#d3c6aa",
		BgSoft: "#343f44",
		FgDim:  "#859289",

		Header:    "#a7c080",
		Accent:    "#a7c080",
		AccentFg:  "#2d353b",
		Highlight: "#dbbc7f",

		Error:   "#e67e80",
		Warning: "#dbbc7f",
		Success: "#a7c080",

		Series: []string{"#7fbbb3", "#a7c080", "#dbbc7f", "#e69875", "#d699b6", "#83c092"},
	},
	"everforest-light": {
		Name:   "everforest-light",
		Bg:     "#fdf6e3",
		Fg:     "#5c6a72",
		BgSoft: "#f4f0d9",
		FgDim:  "#939f91",

		Header:    "#8da101",
		Accent:    "#8da101",
		AccentFg:  "#fdf6e3",
		Highlight: "#dfa000",

		Error:   "#f85552",
		Warning: "#dfa000",
		Success: "#8da101",

		Series: []string{"#3a94c5", "#8da101", "#dfa000", "#f57d26", "#df69ba", "#35a77c"},
	},
	"kanagawa": {
		Name:   "kanagawa",
		Bg:     "#1f1f28",
		Fg:     "#dcd7ba",
		BgSoft: "#2a2a37",
		FgDim:  "#727169",

		Header:    "#7e9cd8",
		Accent:    "#7e9cd8",
		AccentFg:  "#1f1f28",
		Highlight: "#e6c384",

		Error:   "#e82424",
		Warning: "#ff9e3b",
		Success: "#98bb6c",

		Series: []string{"#7e9cd8", "#957fb8", "#98bb6c", "#ffa066", "#7aa89f", "#d27e99"},
	},
	"zenburn": {
		Name:   "zenburn",
		Bg:     "#3f3f3f",
		Fg:     "#dcdccc",
		BgSoft: "#4f4f4f",
		FgDim:  "#7f9f7f",

		Header:    "#f0dfaf",
		Accent:    "#8cd0d3",
		AccentFg:  "#3f3f3f",
		Highlight: "#f0dfaf",

		Error:   "#cc9393",
		Warning: "#dfaf8f",
		Success: "#8fb28f",

		Series: []string{"#8cd0d3", "#f0dfaf", "#8fb28f", "#dfaf8f", "#dc8cc3", "#93e0e3"},
	},
	"material-dark": {
		Name:   "material-dark",
		Bg:     "#263238",
		Fg:     "#eeffff",
		BgSoft: "#314549",
		FgDim:  "#546e7a",

		Header:    "#82aaff",
		Accent:    "#82aaff",
		AccentFg:  "#263238",
		Highlight: "#ffcb6b",

		Error:   "#f07178",
		Warning: "#ffcb6b",
		Success: "#c3e88d",

		Series: []string{"#82aaff", "#c792ea", "#c3e88d", "#f78c6c", "#89ddff", "#f07178"},
	},
	"material-light": {
		Name:   "material-light",
		Bg:     "#fafafa",
		Fg:     "#546e7a",
		BgSoft: "#eeeeee",
		FgDim:  "#90a4ae",

		Header:    "#6182b8",
		Accent:    "#6182b8",
		AccentFg:  "#fafafa",
		Highlight: "#f6a434",

		Error:   "#e53935",
		Warning: "#f6a434",
		Success: "#91b859",

		Series: []string{"#6182b8", "#7c4dff", "#91b859", "#f76d47", "#39adb5", "#e53935"},
	},
	"night-owl": {
		Name:   "night-owl",
		Bg:     "#011627",
		Fg:     "#d6deeb",
		BgSoft: "#0b2942",
		FgDim:  "#637777",

		Header:    "#82aaff",
		Accent:    "#82aaff",
		AccentFg:  "#011627",
		Highlight: "#ecc48d",

		Error:   "#ef5350",
		Warning: "#f78c6c",
		Success: "#addb67",

		Series: []string{"#82aaff", "#c792ea", "#addb67", "#f78c6c", "#7fdbca", "#ff5874"},
	},
	"palenight": {
		Name:   "palenight",
		Bg:     "#292d3e",
		Fg:     "#a6accd",
		BgSoft: "#32374d",
		FgDim:  "#676e95",

		Header:    "#82aaff",
		Accent:    "#82aaff",
		AccentFg:  "#292d3e",
		Highlight: "#ffcb6b",

		Error:   "#f07178",
		Warning: "#ffcb6b",
		Success: "#c3e88d",

		Series: []string{"#82aaff", "#c792ea", "#c3e88d", "#f78c6c", "#89ddff", "#f07178"},
	},
	"synthwave": {
		Name:   "synthwave",
		Bg:     "#2a2139",
		Fg:     "#f2f2f2",
		BgSoft: "#34294f",
		FgDim:  "#848bbd",

		Header:    "#ff7edb",
		Accent:    "#ff7edb",
		AccentFg:  "#2a2139",
		Highlight: "#fede5d",

		Error:   "#fe4450",
		Warning: "#ff8b39",
		Success: "#72f1b8",

		Series: []string{"#ff7edb", "#36f9f6", "#72f1b8", "#fede5d", "#ff8b39", "#b381c5"},
	},
	"cobalt2": {
		Name:   "cobalt2",
		Bg:     "#193549",
		Fg:     "#ffffff",
		BgSoft: "#1f4662",
		FgDim:  "#627e99",

		Header:    "#ffc600",
		Accent:    "#ffc600",
		AccentFg:  "#193549",
		Highlight: "#ffc600",

		Error:   "#ff628c",
		Warning: "#ff9d00",
		Success: "#3ad900",

		Series: []string{"#0088ff", "#ffc600", "#3ad900", "#ff9d00", "#fb94ff", "#80fcff"},
	},
	"oceanic-next": {
		Name:   "oceanic-next",
		Bg:     "#1b2b34",
		Fg:     "#c0c5ce",
		BgSoft: "#343d46",
		FgDim:  "#65737e",

		Header:    "#6699cc",
		Accent:    "#6699cc",
		AccentFg:  "#1b2b34",
		Highlight: "#fac863",

		Error:   "#ec5f67",
		Warning: "#f99157",
		Success: "#99c794",

		Series: []string{"#6699cc", "#c594c5", "#99c794", "#f99157", "#5fb3b3", "#ec5f67"},
	},
	"horizon-dark": {
		Name:   "horizon-dark",
		Bg:     "#1c1e26",
		Fg:     "#e0e0e0",
		BgSoft: "#232530",
		FgDim:  "#6c6f93",

		Header:    "#26bbd9",
		Accent:    "#e95678",
		AccentFg:  "#1c1e26",
		Highlight: "#fac29a",

		Error:   "#e95678",
		Warning: "#fab795",
		Success: "#29d398",

		Series: []string{"#26bbd9", "#b877db", "#29d398", "#fab795", "#59e1e3", "#ee64ac"},
	},
	"iceberg-dark": {
		Name:   "iceberg-dark",
		Bg:     "#161821",
		Fg:     "#c6c8d1",
		BgSoft: "#1e2132",
		FgDim:  "#6b7089",

		Header:    "#84a0c6",
		Accent:    "#84a0c6",
		AccentFg:  "#161821",
		Highlight: "#e2a478",

		Error:   "#e27878",
		Warning: "#e2a478",
		Success: "#b4be82",

		Series: []string{"#84a0c6", "#a093c7", "#b4be82", "#e2a478", "#89b8c2", "#e27878"},
	},
	"papercolor-light": {
		Name:   "papercolor-light",
		Bg:     "#eeeeee",
		Fg:     "#444444",
		BgSoft: "#e4e4e4",
		FgDim:  "#878787",

		Header:    "#005f87",
		Accent:    "#005f87",
		AccentFg:  "#eeeeee",
		Highlight: "#d75f00",

		Error:   "#af0000",
		Warning: "#d75f00",
		Success: "#008700",

		Series: []string{"#0087af", "#8700af", "#008700", "#d75f00", "#005f87", "#d70087"},
	},
	"seoul256": {
		Name:   "seoul256",
		Bg:     "#3a3a3a",
		Fg:     "#d0d0d0",
		BgSoft: "#4e4e4e",
		FgDim:  "#808080",

		Header:    "#85add4",
		Accent:    "#85add4",
		AccentFg:  "#3a3a3a",
		Highlight: "#ffd787",

		Error:   "#d68787",
		Warning: "#d8af5f",
		Success: "#87af87",

		Series: []string{"#85add4", "#d7afaf", "#87af87", "#d8af5f", "#87d7d7", "#d68787"},
	},
	"vesper": {
		Name:   "vesper",
		Bg:     "#101010",
		Fg:     "#ffffff",
		BgSoft: "#1c1c1c",
		FgDim:  "#8b8b8b",

		Header:    "#ffc799",
		Accent:    "#ffc799",
		AccentFg:  "#101010",
		Highlight: "#ffc799",

		Error:   "#ff8080",
		Warning: "#ffc799",
		Success: "#99ffe4",

		Series: []string{"#ffc799", "#99ffe4", "#ff8080", "#b3b3b3", "#ffe0c2", "#7ee6c5"},
	},
	"flexoki-dark": {
		Name:   "flexoki-dark",
		Bg:     "#100f0f",
		Fg:     "#cecdc3",
		BgSoft: "#1c1b1a",
		FgDim:  "#878580",

		Header:    "#4385be",
		Accent:    "#4385be",
		AccentFg:  "#fffcf0",
		Highlight: "#d0a215",

		Error:   "#d14d41",
		Warning: "#da702c",
		Success: "#879a39",

		Series: []string{"#4385be", "#8b7ec8", "#879a39", "#da702c", "#3aa99f", "#ce5d97"},
	},
	"flexoki-light": {
		Name:   "flexoki-light",
		Bg:     "#fffcf0",
		Fg:     "#100f0f",
		BgSoft: "#f2f0e5",
		FgDim:  "#6f6e69",

		Header:    "#205ea6",
		Accent:    "#205ea6",
		AccentFg:  "#fffcf0",
		Highlight: "#ad8301",

		Error:   "#af3029",
		Warning: "#bc5215",
		Success: "#66800b",

		Series: []string{"#205ea6", "#5e409d", "#66800b", "#bc5215", "#24837b", "#a02f6f"},
	},
}

var (
	cacheMu sync.Mutex
	cache   = make(map[string]*Theme)
)

// Builtin returns the derived Theme for a built-in palette name.
// The lookup is case-insensitive and ignores surrounding whitespace.
// Derived themes are cached, so repeated lookups are cheap.
func Builtin(name string) (*Theme, bool) {
	key := strings.ToLower(strings.TrimSpace(name))
	cacheMu.Lock()
	defer cacheMu.Unlock()
	if t, ok := cache[key]; ok {
		return t, true
	}
	p, ok := builtins[key]
	if !ok {
		return nil, false
	}
	t := New(p)
	cache[key] = t
	return t, true
}

// Names returns the names of all built-in palettes, sorted.
func Names() []string {
	names := make([]string, 0, len(builtins))
	for n := range builtins {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// Default returns the default built-in theme.
func Default() *Theme {
	t, ok := Builtin(DefaultName)
	if !ok {
		// Unreachable: DefaultName is always registered.
		panic("theme: default palette missing: " + DefaultName)
	}
	return t
}
