package churn

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const DefaultSince = "90 days ago"

type Runner func(repo string, args ...string) (string, error)

func GitRunner(repo string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

func Collect(run Runner, repo string, since string, roots []Root) (Report, error) {
	if since == "" {
		since = DefaultSince
	}
	raw, err := run(repo, "log", "--since="+since, "--pretty=format:COMMIT\t%an\t%ad", "--date=short", "--name-only")
	if err != nil {
		return Report{Since: since, Nodes: map[string]Stat{}, Message: err.Error()}, err
	}
	return Report{Since: since, Nodes: ParseLog(raw, roots)}, nil
}

func CacheTTL() time.Duration { return time.Minute }
