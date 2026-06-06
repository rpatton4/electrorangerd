package domain

// WindowState records the last-known dimensions of the application window so
// they can be restored on next launch.
type WindowState struct {
	Width  int
	Height int
}

// ConnectionProfile holds a named database connection string for the user's
// connection picker.
type ConnectionProfile struct {
	Name string
	DSN  string
}

// Config is the persisted application configuration, written to disk between
// sessions.
type Config struct {
	ConnectionProfiles []ConnectionProfile
	RecentFiles        []string
	Window             WindowState
}
