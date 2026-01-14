package main

import (
	"context"
	"dagger/ci/internal/dagger"
	"fmt"
	"strings"
)

type Ci struct{}

const (
	goModCachePath        = "/go/.cache/go-mod"
	goBuildCachePath      = "/go/.cache/go-build"
	golangciLintCachePath = "/go/.cache/golangci-lint"

	goModCacheVolumeKey        = "go-mod"
	goBuildCacheVolumeKey      = "go-build"
	golangciLintCacheVolumeKey = "golangci-lint"
)

func (m *Ci) nodeModules(
	src *dagger.Directory,
	nodeVersion string,
) *dagger.Directory {
	files := dag.Directory().
		WithFile("package.json", src.File("package.json")).
		WithFile("package-lock.json", src.File("package-lock.json"))

	return dag.Container().
		From("node:"+nodeVersion).
		WithMountedCache("/root/.npm", dag.CacheVolume("npm")).
		WithDirectory("/src", files).
		WithWorkdir("/src").
		WithExec([]string{"npm", "ci"}).
		Directory("/src/node_modules")
}

// Build Tailwind CSS
func (m *Ci) BuildCSS(
	// +optional
	// +defaultPath="/"
	src *dagger.Directory,
	// +optional
	// +default="24.12.0"
	nodeVersion string,
) *dagger.Directory {
	nodeModules := m.nodeModules(src, nodeVersion)
	return dag.Container().
		From("node:"+nodeVersion).
		WithDirectory("/src", src).
		WithDirectory("/src/node_modules", nodeModules).
		WithWorkdir("/src").
		WithExec([]string{"npm", "run", "build:css"}).
		Directory("/src/assets/css")
}

func (m *Ci) srcWithCSS(
	src *dagger.Directory,
	nodeVersion string,
) *dagger.Directory {
	css := m.BuildCSS(src, nodeVersion)
	return src.WithDirectory("assets/css", css)
}

func (m *Ci) goEnv(
	src *dagger.Directory,
	goversion string,
) *dagger.Container {
	return dag.Container().From("golang:"+goversion).
		WithDirectory("/go/src", src).
		WithWorkdir("/go/src").
		WithEnvVariable("GOMODCACHE", goModCachePath).
		WithEnvVariable("GOCACHE", goBuildCachePath).
		WithMountedCache(goModCachePath, dag.CacheVolume(goModCacheVolumeKey)).
		WithMountedCache(goBuildCachePath, dag.CacheVolume(goBuildCacheVolumeKey))
}

// Build Go binary
func (m *Ci) BuildBinary(
	ctx context.Context,
	// +optional
	// +defaultPath="/"
	src *dagger.Directory,
	// +optional
	// +default="1.25.5"
	goVersion string,
	// +optional
	// +default="24.12.0"
	nodeVersion string,
	// +optional
	// +default="linux"
	goos string,
	// +optional
	// +default="amd64"
	goarch string,
	// +optional
	// +default="-s -w"
	ldflags string,
) *dagger.File {
	return m.goEnv(m.srcWithCSS(src, nodeVersion), goVersion).
		WithEnvVariable("CGO_ENABLED", "0").
		WithEnvVariable("GOOS", goos).
		WithEnvVariable("GOARCH", goarch).
		WithExec([]string{"go", "build", "-ldflags", ldflags, "-o", "/go/bin/web", "./cmd/web"}).
		File("/go/bin/web")
}

// Run golangci-lint
func (m *Ci) Lint(
	ctx context.Context,
	// +optional
	// +defaultPath="/"
	src *dagger.Directory,
	// +optional
	// +default="24.12.0"
	nodeVersion string,
	// +optional
	// +default="1.25.5"
	goVersion string,
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
		WithDirectory("/go/src", m.srcWithCSS(src, nodeVersion)).
		WithWorkdir("/go/src").
		WithExec([]string{"golangci-lint", "run", "-v", "./..."}).
		CombinedOutput(ctx)
}

// Run Go vulnerability check
func (m *Ci) Govulncheck(
	ctx context.Context,
	// +optional
	// +defaultPath="/"
	src *dagger.Directory,
	// +optional
	// +default="1.25.5"
	goVersion string,
) (string, error) {
	return m.goEnv(src, goVersion).
		WithExec([]string{"go", "install", "golang.org/x/vuln/cmd/govulncheck@latest"}).
		WithExec([]string{"govulncheck", "--show", "verbose", "./..."}).
		CombinedOutput(ctx)
}

// Run Go tests
func (m *Ci) Test(
	ctx context.Context,
	// +optional
	// +defaultPath="/"
	src *dagger.Directory,
	// +optional
	// +default="24.12.0"
	nodeVersion string,
	// +optional
	// +default="1.25.5"
	goVersion string,
) (string, error) {
	dbUser := "testuser"
	dbPassword := "testpassword"
	dbName := "testdb"
	db := dag.Container().From("postgres:18").
		WithEnvVariable("POSTGRES_USER", dbUser).
		WithEnvVariable("POSTGRES_PASSWORD", dbPassword).
		WithEnvVariable("POSTGRES_DB", dbName).
		WithExposedPort(5432).
		AsService()

	return m.goEnv(m.srcWithCSS(src, nodeVersion), goVersion).
		WithServiceBinding("db", db).
		WithEnvVariable("TEST_DB_DSN", fmt.Sprintf("postgres://%s:%s@db:5432/%s?sslmode=disable", dbUser, dbPassword, dbName)).
		WithExec([]string{"go", "test", "-cover", "-v", "./cmd/web"}).
		CombinedOutput(ctx)
}

// Build an image for a specific platform, e.g. linux/amd64 or linux/arm64
func (m *Ci) BuildImage(
	// +optional
	// +defaultPath="/"
	src *dagger.Directory,
	// +optional
	// +default="linux/amd64"
	platform dagger.Platform,
	// +optional
	// +default="1.25.5"
	goVersion string,
	// +optional
	// +default="24.12.0"
	nodeVersion string,
	// +optional
	// +default="-s -w"
	ldflags string,
) *dagger.Container {
	return src.DockerBuild(dagger.DirectoryDockerBuildOpts{
		Platform: platform,
		BuildArgs: []dagger.BuildArg{
			{
				Name:  "LDFLAGS",
				Value: ldflags,
			},
			{
				Name:  "GO_VERSION",
				Value: goVersion,
			},
			{
				Name:  "NODE_VERSION",
				Value: nodeVersion,
			},
		},
	})
}

// Export OCI tarball with multi-platform support to the specified path
func (m *Ci) ExportOciTarball(
	// +optional
	// +defaultPath="/"
	src *dagger.Directory,
	// +optional
	// +default="1.25.5"
	goVersion string,
	// +optional
	// +default="24.12.0"
	nodeVersion string,
	// +optional
	// +default="-s -w"
	ldflags string,
) *dagger.File {
	amd64 := m.BuildImage(src, dagger.Platform("linux/amd64"), goVersion, nodeVersion, ldflags)
	arm64 := m.BuildImage(src, dagger.Platform("linux/arm64"), goVersion, nodeVersion, ldflags)

	return amd64.AsTarball(dagger.ContainerAsTarballOpts{
		PlatformVariants: []*dagger.Container{arm64},
	})
}

// Push multi-platform image to registry
func (m *Ci) PushImage(
	ctx context.Context,
	// +optional
	// +defaultPath="/"
	src *dagger.Directory,
	// +optional
	// +default="1.25.5"
	goVersion string,
	// +optional
	// +default="24.12.0"
	nodeVersion string,
	// +optional
	// +default="-s -w"
	ldflags string,
	repo string,
	tags []string,
	username string,
	token *dagger.Secret,
) ([]string, error) {
	if len(tags) == 0 {
		tags = []string{"latest"}
	}

	amd64 := m.BuildImage(src, dagger.Platform("linux/amd64"), goVersion, nodeVersion, ldflags)
	arm64 := m.BuildImage(src, dagger.Platform("linux/arm64"), goVersion, nodeVersion, ldflags)

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
