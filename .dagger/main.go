package main

import (
	"context"
	"dagger/ci/internal/dagger"
	"fmt"
	"runtime"
	"strings"
)

type Ci struct {
	Source      *dagger.Directory
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
	// +defaultPath="/"
	// +ignore=[
	//   ".git",
	//   "**/.git",
	//   "bin",
	//   "dist",
	//   "build",
	//   "out",
	//   "coverage",
	//   "*.log",
	//   "*.out",
	//   "**/node_modules",
	//   "**/.terraform",
	//   "*.tfstate*",
	//   ".venv",
	//   "__pycache__",
	//   ".cache",
	//   ".DS_Store",
	//   ".idea",
	//   ".vscode"
	// ]
	source *dagger.Directory,
	// +optional
	// +default="1.26.2"
	goVersion string,
	// +optional
	// +default="24.14.1"
	nodeVersion string,
	// +optional
	// +default="v2.11.4"
	golangciLintVersion string,
	// +optional
	// +default="-s -w"
	ldflags string,
) *Ci {
	return &Ci{
		Source:      source,
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
func (m *Ci) BuildCSS() *dagger.Directory {
	src := m.Source
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
	css := m.BuildCSS()
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
	src := m.Source
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
	// +optional
	// +default="v2.8.0"
	golangciLintVersion string,
) (string, error) {
	src := m.Source
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
func (m *Ci) Govulncheck(ctx context.Context) (string, error) {
	src := m.Source
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
func (m *Ci) Test(ctx context.Context) (string, error) {
	src := m.Source
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

// Run Go tests and return coverage profile
func (m *Ci) TestCoverProfile(ctx context.Context) *dagger.File {
	dbUser := "testuser"
	dbPassword := "testpassword"
	dbName := "testdb"
	db := m.PostgresService("18", dbUser, dbPassword, dbName)

	src := m.Source
	return m.goEnv(m.srcWithCSS(src)).
		WithServiceBinding("db", db).
		WithEnvVariable("TEST_DB_DSN", fmt.Sprintf("postgres://%s:%s@db:5432/%s?sslmode=disable", dbUser, dbPassword, dbName)).
		WithExec([]string{"go", "test", "-coverprofile=coverage.out", "-v", "./cmd/web"}).
		File("/go/src/coverage.out")
}

// Build an image for a specific platform, e.g. linux/amd64 or linux/arm64
func (m *Ci) BuildImage(
	// +optional
	platform dagger.Platform,
) *dagger.Container {
	if platform == "" {
		platform = dagger.Platform(fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH))
	}

	src := m.Source
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

// Export OCI tarball with multi-platform support
func (m *Ci) ExportOciTarball() *dagger.File {
	amd64 := m.BuildImage(dagger.Platform("linux/amd64"))
	arm64 := m.BuildImage(dagger.Platform("linux/arm64"))

	return amd64.AsTarball(dagger.ContainerAsTarballOpts{
		PlatformVariants: []*dagger.Container{arm64},
	})
}

// Push a multi-platform image to registry
func (m *Ci) PushImage(
	ctx context.Context,
	repo string,
	tags []string,
	username string,
	token *dagger.Secret,
) ([]string, error) {
	if len(tags) == 0 {
		tags = []string{"latest"}
	}

	amd64 := m.BuildImage(dagger.Platform("linux/amd64"))
	arm64 := m.BuildImage(dagger.Platform("linux/arm64"))

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

// Run development server
func (m *Ci) RunDevServer(
	// +optional
	// +default=8080
	port int,
	// +optional
	// +default=false
	tls bool,
	// +optional
	// +defaultPath="/tls/cert.pem"
	tlsCertFile *dagger.File,
	// +optional
	// +defaultPath="/tls/key.pem"
	tlsKeyFile *dagger.File,
) *dagger.Service {
	dbUser := "devuser"
	dbPassword := "devpassword"
	dbName := "devdb"
	db := m.PostgresService("18", dbUser, dbPassword, dbName)

	src := m.Source
	env := m.goEnv(m.srcWithCSS(src)).
		WithServiceBinding("db", db).
		WithEnvVariable("DB_DSN", fmt.Sprintf("postgres://%s:%s@db:5432/%s?sslmode=disable", dbUser, dbPassword, dbName)).
		WithExec([]string{
			"go",
			"build",
			"-trimpath",
			"-buildvcs=false",
			"-ldflags",
			m.Ldflags,
			"-o",
			"/go/bin/web",
			"./cmd/web/",
		})

	srv := env.AsService(dagger.ContainerAsServiceOpts{
		Args: []string{
			"/go/bin/web",
			"-port",
			fmt.Sprintf("%d", port),
			"-base-url",
			fmt.Sprintf("http://localhost:%d", port),
		},
	})

	if tls {
		srv = env.WithMountedFile("/go/tls/cert.pem", tlsCertFile).
			WithMountedFile("/go/tls/key.pem", tlsKeyFile).
			AsService(dagger.ContainerAsServiceOpts{
				Args: []string{
					"/go/bin/web",
					"-port",
					fmt.Sprintf("%d", port),
					"-tls",
					"-tls-cert-file",
					"/go/tls/cert.pem",
					"-tls-key-file",
					"/go/tls/key.pem",
					"-base-url",
					fmt.Sprintf("https://localhost:%d", port),
				},
			})
	}

	return srv
}
