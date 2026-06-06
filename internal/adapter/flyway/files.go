package flyway

const (
	VersionedPrefix  = "V"
	RepeatablePrefix = "R"
	UndoPrefix       = "U"
	SQLExtension     = ".sql"
)

// ParseFilename attempts to parse a Flyway-style migration filename into its
// constituent parts. It returns ok=false until the real implementation is in
// place.
func ParseFilename(_ string) (prefix, version, description string, ok bool) {
	return "", "", "", false
}
