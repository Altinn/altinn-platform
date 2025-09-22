package version

var (
	// Dummy dev values, replaced during build
	// with ldflags
	version = "dev"
	commit  = ""
	date    = "1970-01-01T00:00:00Z"
)

type VersionInfo struct {
	Version string
	Commit  string
	Date    string
}

func Get() VersionInfo {
	return VersionInfo{
		Version: version,
		Commit:  commit,
		Date:    date,
	}
}
