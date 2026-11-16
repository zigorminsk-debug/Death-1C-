package version

import "fmt"

var (
	// Version is the semantic version stamped at build time via -ldflags.
	Version = "1.0.0-dev"

	// BuildNumber is the sequential GitHub Actions run number stamped at build time.
	BuildNumber = "local"

	// Commit is the short Git SHA stamped at build time.
	Commit = "dev"

	// BuildDate is the UTC timestamp stamped at build time.
	BuildDate = ""
)

const (
	AppName        = "Death-1C"
	AppID          = "by.csl.death1c"
	AppTitle       = "Death-1C — Управление сеансами сервера 1С"
	DeveloperName  = "Zakharevich Igor"
	DeveloperEmail = "ziv@csl.by"
	DeveloperPhone = "+375293371412"
)

// DisplayBuild returns a user-friendly build label in Russian.
func DisplayBuild() string {
	if BuildNumber == "" || BuildNumber == "local" {
		return fmt.Sprintf("Версия %s (локальная сборка)", Version)
	}
	return fmt.Sprintf("Сборка #%s (v%s)", BuildNumber, Version)
}

// WindowTitle returns the full application window title including sequential build number.
func WindowTitle() string {
	if BuildNumber == "" || BuildNumber == "local" {
		return AppTitle
	}
	return fmt.Sprintf("%s [Сборка #%s]", AppTitle, BuildNumber)
}
