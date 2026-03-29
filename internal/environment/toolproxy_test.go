package environment

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestFindManifestForWorkingDirMatchesNestedProjectPath(t *testing.T) {
	t.Parallel()

	projectRoot := t.TempDir()
	created, err := Create(CreateOptions{
		Name:        "proxy-wordpress-demo",
		Preset:      "wordpress",
		ProjectRoot: projectRoot,
		Installer:   stubApplicationInstaller{},
	})
	if err != nil {
		t.Fatalf("create environment: %v", err)
	}

	nestedDir := filepath.Join(projectRoot, "wp-content", "plugins")
	if err := os.MkdirAll(nestedDir, 0o755); err != nil {
		t.Fatalf("create nested directory: %v", err)
	}

	manifest, found, err := FindManifestForWorkingDir(projectRoot, nestedDir)
	if err != nil {
		t.Fatalf("find manifest for working dir: %v", err)
	}
	if !found {
		t.Fatal("expected manifest to be found for nested project path")
	}
	if manifest.Name != created.Manifest.Name {
		t.Fatalf("unexpected manifest matched: %s", manifest.Name)
	}
}

func TestBuildToolProxyInvocationRewritesWordPressPathForWPCLI(t *testing.T) {
	t.Parallel()

	projectRoot := t.TempDir()
	created, err := Create(CreateOptions{
		Name:        "proxy-path-demo",
		Preset:      "wordpress",
		ProjectRoot: projectRoot,
		Installer:   stubApplicationInstaller{},
	})
	if err != nil {
		t.Fatalf("create environment: %v", err)
	}

	hostPath := filepath.Join(projectRoot, "wp-content", "themes")
	invocation, supported, err := BuildToolProxyInvocation(created.Manifest, "wp", projectRoot, []string{"option", "get", "home", "--path=" + hostPath}, true)
	if err != nil {
		t.Fatalf("build tool proxy invocation: %v", err)
	}
	if !supported {
		t.Fatal("expected wp tool proxy to be supported for wordpress environment")
	}

	joinedArgs := strings.Join(invocation.Args, " ")
	if !strings.Contains(joinedArgs, " -e XDEBUG_MODE=") {
		t.Fatalf("expected wp proxy to define XDEBUG_MODE for host-driven wp invocations: %s", joinedArgs)
	}
	if !strings.Contains(joinedArgs, "--user www-data") {
		t.Fatalf("expected wp proxy to run as www-data: %s", joinedArgs)
	}
	if !strings.Contains(joinedArgs, "--path=/var/www/html/wp-content/themes") {
		t.Fatalf("expected wp proxy to rewrite host path to container path: %s", joinedArgs)
	}
	if !strings.Contains(joinedArgs, " exec -T -e XDEBUG_MODE=off --user www-data -w /var/www/html web wp ") {
		t.Fatalf("expected wp proxy to target the php service with TTY disabled: %s", joinedArgs)
	}
}

func TestBuildToolProxyInvocationRewritesPublishedDatabasePort(t *testing.T) {
	t.Parallel()

	projectRoot := t.TempDir()
	created, err := Create(CreateOptions{
		Name:        "proxy-db-demo",
		Preset:      "wordpress",
		ProjectRoot: projectRoot,
		Installer:   stubApplicationInstaller{},
	})
	if err != nil {
		t.Fatalf("create environment: %v", err)
	}

	invocation, supported, err := BuildToolProxyInvocation(created.Manifest, "mysql", projectRoot, []string{"-h", "127.0.0.1", "-P", strconv.Itoa(created.Manifest.Network.DatabasePort)}, true)
	if err != nil {
		t.Fatalf("build db proxy invocation: %v", err)
	}
	if !supported {
		t.Fatal("expected mysql proxy to be supported")
	}

	joinedArgs := strings.Join(invocation.Args, " ")
	if !strings.Contains(joinedArgs, " exec -T db sh -lc ") {
		t.Fatalf("expected db proxy to exec into the database service: %s", joinedArgs)
	}
	if !strings.Contains(joinedArgs, " -h 127.0.0.1") {
		t.Fatalf("expected db proxy to rewrite local database hosts to the current container: %s", joinedArgs)
	}
	if !strings.Contains(joinedArgs, " -P 3306") {
		t.Fatalf("expected db proxy to rewrite the published host port to the container port: %s", joinedArgs)
	}
	if !strings.Contains(joinedArgs, `exec mariadb "$@"`) {
		t.Fatalf("expected db proxy to use the MariaDB/MySQL compatibility shell: %s", joinedArgs)
	}
}

func TestBuildToolProxyInvocationRewritesDatabaseServiceHostToContainerLocalhost(t *testing.T) {
	t.Parallel()

	projectRoot := t.TempDir()
	created, err := Create(CreateOptions{
		Name:        "proxy-db-host-demo",
		Preset:      "wordpress",
		ProjectRoot: projectRoot,
		Installer:   stubApplicationInstaller{},
	})
	if err != nil {
		t.Fatalf("create environment: %v", err)
	}

	invocation, supported, err := BuildToolProxyInvocation(created.Manifest, "mariadb", projectRoot, []string{"--host=db", "--user=elk", "--database=elk"}, true)
	if err != nil {
		t.Fatalf("build db proxy invocation: %v", err)
	}
	if !supported {
		t.Fatal("expected mariadb proxy to be supported")
	}

	joinedArgs := strings.Join(invocation.Args, " ")
	if !strings.Contains(joinedArgs, " --host=127.0.0.1") {
		t.Fatalf("expected service host to be rewritten to container localhost: %s", joinedArgs)
	}
}
