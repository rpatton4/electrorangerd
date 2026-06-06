package domain

// WindowState records the last-known dimensions of the application window so
// they can be restored on next launch.
type WindowState struct {
	Width  int
	Height int
}

// Config is the persisted non-secret application configuration, written to
// disk between sessions. Connection profiles are NOT held here — they live in
// the encrypted vault (see VaultBlob) so plaintext DSNs never appear in the
// config file.
type Config struct {
	RecentFiles []string
	Window      WindowState
	// ThemeName selects the UI theme. Valid values: "" (follow OS preference),
	// "dark", or "light". Unknown values fall back to OS / dark default.
	ThemeName string
}
