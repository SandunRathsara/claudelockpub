package public

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func scriptPath(t *testing.T, name string) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to resolve test file path")
	}
	return filepath.Join(filepath.Dir(file), name)
}

func writeExecutable(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0755); err != nil {
		t.Fatalf("write executable %s: %v", path, err)
	}
}

func fakeToolEnv(t *testing.T) (string, []string) {
	t.Helper()
	binDir := t.TempDir()

	writeExecutable(t, filepath.Join(binDir, "uname"), `#!/bin/sh
case "$1" in
  -s) printf 'Darwin\n' ;;
  -m) printf 'arm64\n' ;;
  *) printf 'Darwin\n' ;;
esac
`)
	writeExecutable(t, filepath.Join(binDir, "curl"), `#!/bin/sh
out=''
while [ "$#" -gt 0 ]; do
  case "$1" in
    -o)
      out="$2"
      shift 2
      ;;
    *)
      shift
      ;;
  esac
done
if [ -n "$out" ]; then
  : >"$out"
  exit 0
fi
printf '{"tag_name":"v1.2.3","assets":[{"name":"claudelock-cli-v1.2.3-darwin-arm64.tar.gz"}]}'
`)
	writeExecutable(t, filepath.Join(binDir, "tar"), `#!/bin/sh
dest=''
while [ "$#" -gt 0 ]; do
  case "$1" in
    -C)
      dest="$2"
      shift 2
      ;;
    *)
      shift
      ;;
  esac
done
path="$dest/claudelock-cli-v1.2.3-darwin-arm64"
printf '#!/bin/sh\nexit 0\n' >"$path"
chmod +x "$path"
`)
	writeExecutable(t, filepath.Join(binDir, "install"), `#!/bin/sh
last=''
for arg in "$@"; do
  last="$arg"
done
case "$1" in
  -d)
    mkdir -p "$last"
    exit 0
    ;;
esac
exit 0
`)
	writeExecutable(t, filepath.Join(binDir, "sudo"), `#!/bin/sh
exec "$@"
`)

	pathEnv := binDir + string(os.PathListSeparator) + os.Getenv("PATH")
	return binDir, []string{"PATH=" + pathEnv}
}

func runScript(t *testing.T, path string, env []string, stdin string) (string, error) {
	t.Helper()
	cmd := exec.Command("sh", path)
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdin = strings.NewReader(stdin)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestInstallAddsManagedAliasForZsh(t *testing.T) {
	_, toolEnv := fakeToolEnv(t)
	homeDir := t.TempDir()
	rcPath := filepath.Join(homeDir, ".zshrc")

	output, err := runScript(t, scriptPath(t, "install.sh"), append(toolEnv, "HOME="+homeDir, "SHELL=/bin/zsh"), "alice\nsecret\n\n")
	if err != nil {
		t.Fatalf("install.sh failed: %v\n%s", err, output)
	}

	rcData, err := os.ReadFile(rcPath)
	if err != nil {
		t.Fatalf("read zshrc: %v", err)
	}
	rcContent := string(rcData)
	if !strings.Contains(rcContent, "# claudelock managed start") {
		t.Fatalf("expected managed alias block in zshrc, got:\n%s", rcContent)
	}
	if !strings.Contains(rcContent, `alias claude="claudelock run -- claude"`) {
		t.Fatalf("expected claude alias in zshrc, got:\n%s", rcContent)
	}
	if !strings.Contains(output, "shell_reload:") {
		t.Fatalf("expected shell reload reminder, got:\n%s", output)
	}
}
