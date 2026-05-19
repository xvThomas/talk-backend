package version

import "runtime/debug"

// Version is the application version. It is resolved in order:
// 1. Build-time injection via: go build -ldflags "-X github.com/xvThomas/LLMClientWrapper/talk-libs/version.Version=v1.2.3"
// 2. VCS tag embedded by Go at build time (requires a git tag on the module)
// 3. VCS revision (short commit hash)
// 4. "dev" as fallback
var Version = ""

func init() {
	if Version != "" {
		return
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		Version = "dev"
		return
	}
	// If built with a module version (e.g. go install module@v1.2.3)
	if info.Main.Version != "" && info.Main.Version != "(devel)" {
		Version = info.Main.Version
		return
	}
	// Fall back to VCS info (git commit + dirty flag)
	var revision, modified string
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			revision = s.Value
		case "vcs.modified":
			if s.Value == "true" {
				modified = "-dirty"
			}
		}
	}
	if revision != "" {
		if len(revision) > 8 {
			revision = revision[:8]
		}
		Version = revision + modified
		return
	}
	Version = "dev"
}
