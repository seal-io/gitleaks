package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gitleaks/go-gitdiff/gitdiff"
)

// GitLog returns a channel of gitdiff.File objects from the
// git log -p command for the given source.
func GitLog(source string, logOpts string) (<-chan *gitdiff.File, error) {
	var args = []string{"log", "--patch", "--unified=0"}
	if logOpts != "" {
		args = append(args, strings.Split(logOpts, " ")...)
	} else {
		args = append(args, "--full-history", "--all")
	}

	var g, err = newGitter(source)
	if err != nil {
		return nil, err
	}

	var ctx, cancel = context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	bs, err := g.exec(ctx, args...)
	if err != nil {
		return nil, err
	}
	return gitdiff.Parse(bytes.NewReader(bs))
}

// GitDiff returns a channel of gitdiff.File objects from
// the git diff command for the given source.
func GitDiff(source string, staged bool) (<-chan *gitdiff.File, error) {
	var args = []string{"diff", "--unified=0"}
	if staged {
		args = append(args, "--staged", ".")
	} else {
		args = append(args, ".")
	}

	var g, err = newGitter(source)
	if err != nil {
		return nil, err
	}

	var ctx, cancel = context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	bs, err := g.exec(ctx, args...)
	if err != nil {
		return nil, err
	}
	return gitdiff.Parse(bytes.NewReader(bs))
}

func newGitter(dir string) (*gitter, error) {
	const bin = "git"
	var binPath, err = exec.LookPath(bin)
	if err != nil {
		return nil, fmt.Errorf("%s is required for executing: %w", bin, err)
	}
	dir = filepath.Clean(dir)
	dir, err = filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("%s is not an absolute path: %w", dir, err)
	}
	var g = &gitter{
		binPath: binPath,
		path:    dir,
	}

	var ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err = g.exec(ctx, "config", "--add", "--global", "safe.directory", dir)
	if err != nil {
		return nil, err
	}
	_, err = g.exec(ctx, "config", "diff.renameLimit", strconv.FormatUint(uint64(math.MaxUint16), 10))
	if err != nil {
		return nil, err
	}

	return g, nil
}

type gitter struct {
	binPath string
	path    string
}

func (g *gitter) exec(ctx context.Context, args ...string) ([]byte, error) {
	args = append([]string{"--no-pager", "-C", g.path}, args...)
	var cmd = exec.CommandContext(ctx, g.binPath, args...)
	cmd.Dir = g.path
	bs, err := cmd.Output()
	if err != nil {
		var ee exec.ExitError
		if !errors.Is(err, &ee) {
			return nil, fmt.Errorf("error executing 'git %s': %w", strings.Join(args, " "), err)
		}
		return nil, fmt.Errorf("error executing 'git %s', output: %s : %w", strings.Join(args, " "), strings.TrimSpace(string(ee.Stderr)), err)
	}
	return bs, nil
}
