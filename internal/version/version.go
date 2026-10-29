package version

import (
	"os/exec"
	"runtime/debug"
	"strings"
	"sync"
)

var (
	// Version is injected at build-time using:
	// -ldflags "-X gomesh/internal/version.Version=v1.2.3"
	Version   = ""
	GitCommit = ""
	BuildDate = ""

	cachedVersion string
	once          sync.Once
)

// FallbackVersion is used if neither ldflags, build info, nor git tags are available.
const FallbackVersion = "v0.1.7"

// Get returns the dynamic version string (e.g. "v0.1.7").
func Get() string {
	once.Do(func() {
		if Version != "" && Version != "dev" && Version != "(devel)" {
			cachedVersion = formatVersion(Version)
			return
		}

		// 1. Try reading build info from Go runtime (e.g. go install or module builds)
		if info, ok := debug.ReadBuildInfo(); ok {
			if info.Main.Version != "" && info.Main.Version != "(devel)" {
				cachedVersion = formatVersion(info.Main.Version)
				return
			}
			for _, setting := range info.Settings {
				if setting.Key == "vcs.revision" && GitCommit == "" {
					GitCommit = setting.Value
				}
				if setting.Key == "vcs.time" && BuildDate == "" {
					BuildDate = setting.Value
				}
			}
		}

		// 2. Try git describe --tags --always (during local dev or git checkout)
		if out, err := exec.Command("git", "describe", "--tags", "--always").Output(); err == nil {
			v := strings.TrimSpace(string(out))
			if v != "" {
				cachedVersion = formatVersion(v)
				return
			}
		}

		cachedVersion = FallbackVersion
	})
	return cachedVersion
}

// Info returns detailed build metadata
func Info() map[string]string {
	return map[string]string{
		"version":    Get(),
		"git_commit": GitCommit,
		"build_date": BuildDate,
	}
}

func formatVersion(v string) string {
	v = strings.TrimSpace(v)
	if !strings.HasPrefix(v, "v") && v != "" {
		return "v" + v
	}
	return v
}
