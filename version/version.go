package version

import "runtime"

var (
	Version   = "v0.0.0-dev"
	Revision  = "undefined"
	Branch    string
	BuildUser string
	BuildDate string
	GoVersion = runtime.Version()
)
