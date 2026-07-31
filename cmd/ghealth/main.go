package main

import (
	"fmt"
	"os"
	"runtime/debug"
	"strings"

	"github.com/rudrankriyam/Google-Health-CLI/cmd"
)

var (
	version = "devel"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	buildInfo, _ := debug.ReadBuildInfo()
	resolvedVersion, resolvedCommit, resolvedDate := resolveBuildMetadata(version, commit, date, buildInfo)
	os.Exit(cmd.Run(os.Args[1:], fmt.Sprintf("%s (%s, %s)", resolvedVersion, resolvedCommit, resolvedDate)))
}

func resolveBuildMetadata(
	configuredVersion string,
	configuredCommit string,
	configuredDate string,
	buildInfo *debug.BuildInfo,
) (string, string, string) {
	if buildInfo == nil {
		return configuredVersion, configuredCommit, configuredDate
	}
	if configuredVersion == "devel" && buildInfo.Main.Version != "" && buildInfo.Main.Version != "(devel)" {
		configuredVersion = strings.TrimPrefix(buildInfo.Main.Version, "v")
	}
	for _, setting := range buildInfo.Settings {
		switch setting.Key {
		case "vcs.revision":
			if configuredCommit == "unknown" && setting.Value != "" {
				configuredCommit = setting.Value
			}
		case "vcs.time":
			if configuredDate == "unknown" && setting.Value != "" {
				configuredDate = setting.Value
			}
		}
	}
	return configuredVersion, configuredCommit, configuredDate
}
