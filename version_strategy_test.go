package embeddedpostgres

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

//nolint:funlen
func Test_DefaultVersionStrategy_AllGolangDistributions(t *testing.T) {
	allGolangDistributions := map[string][]string{
		"aix/ppc64":       {"aix", "ppc64"},
		"android/386":     {"android", "386"},
		"android/amd64":   {"android", "amd64"},
		"android/arm":     {"android", "arm"},
		"android/arm64":   {"android", "arm64"},
		"darwin/amd64":    {"darwin", "amd64"},
		"darwin/arm64":    {"darwin", "arm64v8"},
		"dragonfly/amd64": {"dragonfly", "amd64"},
		"freebsd/386":     {"freebsd", "386"},
		"freebsd/amd64":   {"freebsd", "amd64"},
		"freebsd/arm":     {"freebsd", "arm"},
		"freebsd/arm64":   {"freebsd", "arm64"},
		"illumos/amd64":   {"illumos", "amd64"},
		"js/wasm":         {"js", "wasm"},
		"linux/386":       {"linux", "386"},
		"linux/amd64":     {"linux", "amd64"},
		"linux/arm":       {"linux", "arm"},
		"linux/arm64":     {"linux", "arm64v8"},
		"linux/mips":      {"linux", "mips"},
		"linux/mips64":    {"linux", "mips64"},
		"linux/mips64le":  {"linux", "mips64le"},
		"linux/mipsle":    {"linux", "mipsle"},
		"linux/ppc64":     {"linux", "ppc64"},
		"linux/ppc64le":   {"linux", "ppc64le"},
		"linux/riscv64":   {"linux", "riscv64"},
		"linux/s390x":     {"linux", "s390x"},
		"netbsd/386":      {"netbsd", "386"},
		"netbsd/amd64":    {"netbsd", "amd64"},
		"netbsd/arm":      {"netbsd", "arm"},
		"netbsd/arm64":    {"netbsd", "arm64"},
		"openbsd/386":     {"openbsd", "386"},
		"openbsd/amd64":   {"openbsd", "amd64"},
		"openbsd/arm":     {"openbsd", "arm"},
		"openbsd/arm64":   {"openbsd", "arm64"},
		"plan9/386":       {"plan9", "386"},
		"plan9/amd64":     {"plan9", "amd64"},
		"plan9/arm":       {"plan9", "arm"},
		"solaris/amd64":   {"solaris", "amd64"},
		"windows/386":     {"windows", "386"},
		"windows/amd64":   {"windows", "amd64"},
		"windows/arm":     {"windows", "arm"},
	}

	defaultConfig := DefaultConfig()

	for dist, expected := range allGolangDistributions {
		dist := dist
		expected := expected

		t.Run(fmt.Sprintf("DefaultVersionStrategy_%s", dist), func(t *testing.T) {
			osArch := strings.Split(dist, "/")

			operatingSystem, architecture, postgresVersion := defaultVersionStrategy(
				defaultConfig,
				osArch[0],
				osArch[1],
				linuxMachineName,
				func() bool {
					return false
				})()

			assert.Equal(t, expected[0], operatingSystem)
			assert.Equal(t, expected[1], architecture)
			assert.Equal(t, V14, postgresVersion)
		})
	}
}

func Test_DefaultVersionStrategy_Linux_ARM32V6(t *testing.T) {
	operatingSystem, architecture, postgresVersion := defaultVersionStrategy(
		DefaultConfig(),
		"linux",
		"arm",
		func() string {
			return "armv6l"
		}, func() bool {
			return false
		})()

	assert.Equal(t, "linux", operatingSystem)
	assert.Equal(t, "arm32v6", architecture)
	assert.Equal(t, V14, postgresVersion)
}

func Test_DefaultVersionStrategy_Linux_ARM32V7(t *testing.T) {
	operatingSystem, architecture, postgresVersion := defaultVersionStrategy(
		DefaultConfig(),
		"linux",
		"arm",
		func() string {
			return "armv7l"
		}, func() bool {
			return false
		})()

	assert.Equal(t, "linux", operatingSystem)
	assert.Equal(t, "arm32v7", architecture)
	assert.Equal(t, V14, postgresVersion)
}

func Test_DefaultVersionStrategy_Linux_Alpine(t *testing.T) {
	operatingSystem, architecture, postgresVersion := defaultVersionStrategy(
		DefaultConfig(),
		"linux",
		"amd64",
		func() string {
			return ""
		},
		func() bool {
			return true
		},
	)()

	assert.Equal(t, "linux", operatingSystem)
	assert.Equal(t, "amd64-alpine", architecture)
	assert.Equal(t, V14, postgresVersion)
}

func Test_DefaultVersionStrategy_shouldUseAlpineLinuxBuild(t *testing.T) {
	assert.NotPanics(t, func() {
		shouldUseAlpineLinuxBuild()
	})
}
