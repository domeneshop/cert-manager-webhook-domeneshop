package domeneshop

import (
	"fmt"
	"os"
	"runtime"
)

// Version information fetched from environment variables in docker build process
var (
	version   = "1.0.0"
	gitCommit = os.Getenv("GIT_COMMIT")
	buildDate = os.Getenv("BUILD_DATE")
)

// VersionInfo represents the current running version
type VersionInfo struct {
	Version   string `json:"version"`
	GitCommit string `json:"gitCommit"`
	BuildDate string `json:"buildDate"`
	GoVersion string `json:"goVersion"`
	Compiler  string `json:"compiler"`
	Platform  string `json:"platform"`
}

// GetVersion returns the current running version
func GetVersion() VersionInfo {
	return VersionInfo{
		Version:   version,
		GitCommit: gitCommit,
		BuildDate: buildDate,
		GoVersion: runtime.Version(),
		Compiler:  runtime.Compiler,
		Platform:  fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
	}
}
