package main

import (
	"context"
	"dagger/ci/internal/dagger"
	"fmt"
	"runtime"
	"strings"
)

type Ci struct {
	GoVersion   string
	NodeVersion string
	Ldflags     string
}

const (
	goModCachePath        = "/go/.cache/go-mod"
	goBuildCachePath      = "/go/.cache/go-build"
	golangciLintCachePath = "/go/.cache/golangci-lint"

	goModCacheVolumeKey        = "go-mod"
	goBuildCacheVolumeKey      = "go-build"
	golangciLintCacheVolumeKey = "golangci-lint"
)

func New(
	// +optional
	// +default="1.25.6"
	goVersion string,
	// +optional
	// +default="24.13.0"
	nodeVersion string,
	// +optional
	// +default="v2.8.0"
	golangciLintVersion string,
	// +optional
	// +default="-s -w"
	ldflags string,
) *Ci {
	return &Ci{
		GoVersion:   goVersion,
		NodeVersion: nodeVersion,
		Ldflags:     ldflags,
	}
}

func (m *Ci) nodeModules(src *dagger.Directory) *dagger.Directory {
	deps := dag.Directory().
		WithFile("package.json", src.File("package.json")).
		WithFile("package-lock.json", src.File("package-lock.json"))

	return dag.Container().
		From("node:"+m.NodeVersion).
		WithMountedCache("/root/.npm", dag.CacheVolume("npm")).
		WithDirectory("/src", deps).
		WithWorkdir("/src").
		WithExec([]string{"npm", "ci"}).
		Directory("/src/node_modules")
}

// Build Tailwind CSS
func (m *Ci) BuildCSS(
	// +defaultPath="/"
	// +ignore=[
	//   "*",
	//   "!package.json",
	//   "!package-lock.json",
	//   "!assets/css/",
	//   "!templates/html/",
	//   "!templates/css/"
	// ]
	src *dagger.Directory,
) *dagger.Directory {
	nodeModules := m.nodeModules(src)
	return dag.Container().
		From("node:"+m.NodeVersion).
		WithDirectory("/src", src).
		WithDirectory("/src/node_modules", nodeModules).
		WithWorkdir("/src").
		WithExec([]string{"npm", "run", "build:css"}).
		Directory("/src/assets/css")
}

func (m *Ci) srcWithCSS(src *dagger.Directory) *dagger.Directory {
	css := m.BuildCSS(src)
	return src.WithDirectory("assets/css", css)
}

func (m *Ci) goEnv(src *dagger.Directory) *dagger.Container {
	deps := dag.Directory().
		WithFile("go.mod", src.File("go.mod")).
		WithFile("go.sum", src.File("go.sum"))
	return dag.Container().From("golang:"+m.GoVersion).
		WithDirectory("/go/src", deps).
		WithWorkdir("/go/src").
		WithEnvVariable("GOMODCACHE", goModCachePath).
		WithEnvVariable("GOCACHE", goBuildCachePath).
		WithMountedCache(goModCachePath, dag.CacheVolume(goModCacheVolumeKey)).
		WithMountedCache(goBuildCachePath, dag.CacheVolume(goBuildCacheVolumeKey)).
		WithExec([]string{"go", "mod", "download"}).
		WithDirectory("/go/src", src)
}

// Build Go binary
func (m *Ci) BuildBinary(
	ctx context.Context,
	// +defaultPath="/"
	// +ignore=[
	//   "*",
	//   "!**/*.go",
	//   "!go.sum",
	//   "!go.mod",
	//   "!package.json",
	//   "!package-lock.json",
	//   "!assets/",
	//   "!migrations/",
	//   "!templates/"
	// ]
	src *dagger.Directory,
	// +optional
	goos string,
	// +optional
	goarch string,
) *dagger.File {
	if goos == "" {
		goos = runtime.GOOS
	}
	if goarch == "" {
		goarch = runtime.GOARCH
	}
	return m.goEnv(m.srcWithCSS(src)).
		WithEnvVariable("CGO_ENABLED", "0").
		WithEnvVariable("GOOS", goos).
		WithEnvVariable("GOARCH", goarch).
		WithExec([]string{"go", "build", "-trimpath", "-buildvcs=false", "-ldflags", m.Ldflags, "-o", "/go/bin/web", "./cmd/web"}).
		File("/go/bin/web")
}

// Run golangci-lint
func (m *Ci) GolangciLint(
	ctx context.Context,
	// +defaultPath="/"
	// +ignore=[
	//   "*",
	//   "!**/*.go",
	//   "!go.sum",
	//   "!go.mod",
	//   "!package.json",
	//   "!package-lock.json",
	//   "!assets/",
	//   "!migrations/",
	//   "!templates/"
	// ]
	src *dagger.Directory,
	// +optional
	// +default="v2.8.0"
	golangciLintVersion string,
) (string, error) {
	return dag.Container().
		From("golangci/golangci-lint:"+golangciLintVersion).
		WithEnvVariable("GOMODCACHE", goModCachePath).
		WithEnvVariable("GOCACHE", goBuildCachePath).
		WithEnvVariable("GOLANGCI_LINT_CACHE", golangciLintCachePath).
		WithMountedCache(goModCachePath, dag.CacheVolume(goModCacheVolumeKey)).
		WithMountedCache(goBuildCachePath, dag.CacheVolume(goBuildCacheVolumeKey)).
		WithMountedCache(golangciLintCachePath, dag.CacheVolume(golangciLintCacheVolumeKey)).
		WithDirectory("/go/src", m.srcWithCSS(src)).
		WithWorkdir("/go/src").
		WithExec([]string{"golangci-lint", "run", "-v", "./..."}).
		CombinedOutput(ctx)
}

// Run Go vulnerability check
func (m *Ci) Govulncheck(
	ctx context.Context,
	// +defaultPath="/"
	// +ignore=[
	//   "*",
	//   "!**/*.go",
	//   "!go.sum",
	//   "!go.mod",
	//   "!assets/",
	//   "!migrations/",
	//   "!templates/"
	// ]
	src *dagger.Directory,
) (string, error) {
	return m.goEnv(src).
		WithExec([]string{"go", "install", "golang.org/x/vuln/cmd/govulncheck@latest"}).
		WithExec([]string{"govulncheck", "--show", "verbose", "./..."}).
		CombinedOutput(ctx)
}

// Run Postgres database service
func (m *Ci) PostgresService(
	version string,
	dbUser string,
	dbPassword string,
	dbName string,
) *dagger.Service {
	return dag.Container().
		From("postgres:"+version).
		WithEnvVariable("POSTGRES_USER", dbUser).
		WithEnvVariable("POSTGRES_PASSWORD", dbPassword).
		WithEnvVariable("POSTGRES_DB", dbName).
		WithExposedPort(5432).
		AsService()
}

// Run Go tests
func (m *Ci) Test(
	ctx context.Context,
	// +defaultPath="/"
	// +ignore=[
	//   "*",
	//   "!**/*.go",
	//   "!go.sum",
	//   "!go.mod",
	//   "!package.json",
	//   "!package-lock.json",
	//   "!assets/",
	//   "!migrations/",
	//   "!templates/"
	// ]
	src *dagger.Directory,
) (string, error) {
	dbUser := "testuser"
	dbPassword := "testpassword"
	dbName := "testdb"
	db := m.PostgresService("18", dbUser, dbPassword, dbName)

	return m.goEnv(m.srcWithCSS(src)).
		WithServiceBinding("db", db).
		WithEnvVariable("TEST_DB_DSN", fmt.Sprintf("postgres://%s:%s@db:5432/%s?sslmode=disable", dbUser, dbPassword, dbName)).
		WithExec([]string{"go", "test", "-cover", "-v", "./cmd/web"}).
		CombinedOutput(ctx)
}

// Build an image for a specific platform, e.g. linux/amd64 or linux/arm64
func (m *Ci) BuildImage(
	// +defaultPath="/"
	// +ignore=[
	//   "*",
	//   "!**/*.go",
	//   "!Dockerfile",
	//   "!.dockerignore",
	//   "!go.sum",
	//   "!go.mod",
	//   "!package.json",
	//   "!package-lock.json",
	//   "!assets/",
	//   "!migrations/",
	//   "!templates/"
	// ]
	src *dagger.Directory,
	// +optional
	platform dagger.Platform,
) *dagger.Container {
	if platform == "" {
		platform = dagger.Platform(fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH))
	}

	return src.DockerBuild(dagger.DirectoryDockerBuildOpts{
		Platform: platform,
		BuildArgs: []dagger.BuildArg{
			{
				Name:  "LDFLAGS",
				Value: m.Ldflags,
			},
			{
				Name:  "GO_VERSION",
				Value: m.GoVersion,
			},
			{
				Name:  "NODE_VERSION",
				Value: m.NodeVersion,
			},
		},
	})
}

// Export OCI tarball with multi-platform support to the specified path
func (m *Ci) ExportOciTarball(
	// +defaultPath="/"
	// +ignore=[
	//   "*",
	//   "!**/*.go",
	//   "!Dockerfile",
	//   "!.dockerignore",
	//   "!go.sum",
	//   "!go.mod",
	//   "!package.json",
	//   "!package-lock.json",
	//   "!assets/",
	//   "!migrations/",
	//   "!templates/"
	// ]
	src *dagger.Directory,
) *dagger.File {
	amd64 := m.BuildImage(src, dagger.Platform("linux/amd64"))
	arm64 := m.BuildImage(src, dagger.Platform("linux/arm64"))

	return amd64.AsTarball(dagger.ContainerAsTarballOpts{
		PlatformVariants: []*dagger.Container{arm64},
	})
}

// Push multi-platform image to registry
func (m *Ci) PushImage(
	ctx context.Context,
	// +defaultPath="/"
	// +ignore=[
	//   "*",
	//   "!**/*.go",
	//   "!Dockerfile",
	//   "!.dockerignore",
	//   "!go.sum",
	//   "!go.mod",
	//   "!package.json",
	//   "!package-lock.json",
	//   "!assets/",
	//   "!migrations/",
	//   "!templates/"
	// ]
	src *dagger.Directory,
	repo string,
	tags []string,
	username string,
	token *dagger.Secret,
) ([]string, error) {
	if len(tags) == 0 {
		tags = []string{"latest"}
	}

	amd64 := m.BuildImage(src, dagger.Platform("linux/amd64"))
	arm64 := m.BuildImage(src, dagger.Platform("linux/arm64"))

	registry := getRegistryFromImageRepo(repo)

	amd64 = amd64.WithRegistryAuth(registry, username, token)
	arm64 = arm64.WithRegistryAuth(registry, username, token)

	output := []string{}

	primaryTag := tags[0]
	ref, err := amd64.Publish(ctx, repo+":"+primaryTag, dagger.ContainerPublishOpts{
		PlatformVariants: []*dagger.Container{arm64},
	})
	if err != nil {
		return nil, err
	}

	ref = strings.TrimSpace(ref)
	output = append(output, ref)

	craneCtr := dag.Container().
		From("gcr.io/go-containerregistry/crane:debug").
		WithMountedSecret("/tmp/token", token).
		WithExec([]string{
			"sh", "-lc",
			fmt.Sprintf(
				"cat /tmp/token | crane auth login %s --username %s --password-stdin",
				registry, username,
			),
		})

	if len(tags) > 1 {
		for _, tag := range tags[1:] {
			out, err := craneCtr.WithExec([]string{
				"sh", "-lc",
				fmt.Sprintf(
					"crane tag %s %s", ref, tag,
				),
			}).CombinedOutput(ctx)
			if err != nil {
				return nil, err
			}

			out = strings.TrimSpace(out)

			output = append(output, out)
		}
	}

	return output, nil
}
