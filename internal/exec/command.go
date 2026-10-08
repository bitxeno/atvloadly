package exec

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/bitxeno/atvloadly/internal/log"
)

var ErrCommandTimeout = errors.New("command execute timeout")

// secretPlaceholder replaces a secret argument in the debug log.
const secretPlaceholder = "***"

// Cmd represents a Cmd to be executed.
type Cmd struct {
	Name    string
	Args    []string
	Timeout time.Duration
	Dir     string
	Env     []string
	Stdout  io.Writer
	Stderr  io.Writer
	Stdin   io.Reader
	ctx     context.Context
	secrets []string
}

// Command creates a new command instance.
func Command(name string, args ...string) *Cmd {
	return &Cmd{
		Name:    name,
		Args:    args,
		Timeout: 10 * time.Minute,
	}
}

// NewCommand creates a new command instance.
func NewCommand(name string, args ...string) *Cmd {
	return Command(name, args...)
}

// CommandContext creates a new command instance with context.
func CommandContext(ctx context.Context, name string, args ...string) *Cmd {
	c := Command(name, args...)
	c.ctx = ctx
	return c
}

// WithTimeout set the timeout for the command.
func (c *Cmd) WithTimeout(timeout time.Duration) *Cmd {
	c.Timeout = timeout
	return c
}

// WithDir sets the working directory for the command.
func (c *Cmd) WithDir(dir string) *Cmd {
	c.Dir = dir
	return c
}

// WithEnv sets the environment variables for the command.
func (c *Cmd) WithEnv(env []string) *Cmd {
	c.Env = env
	return c
}

// WithStdout sets the stdout writer for the command.
func (c *Cmd) WithStdout(stdout io.Writer) *Cmd {
	c.Stdout = stdout
	return c
}

// WithStderr sets the stderr writer for the command.
func (c *Cmd) WithStderr(stderr io.Writer) *Cmd {
	c.Stderr = stderr
	return c
}

// WithStdin sets the stdin reader for the command.
func (c *Cmd) WithStdin(stdin io.Reader) *Cmd {
	c.Stdin = stdin
	return c
}

// WithSecret marks values that must never be written to a log. An argument
// equal to one of them is replaced by a placeholder in the debug log; the
// argument list passed to the process is unchanged. Values are matched exactly
// and empty values are ignored.
//
// A value is used rather than a flag position on purpose: PlumeImpactor reuses
// -p for the password of certificate import/export and account login, but for
// the package path of sign, which is not a secret.
func (c *Cmd) WithSecret(values ...string) *Cmd {
	c.secrets = append(c.secrets, values...)
	return c
}

// logLine returns the command line for the debug log with every secret
// argument masked.
func (c *Cmd) logLine() string {
	if len(c.secrets) == 0 {
		return c.Name + " " + strings.Join(c.Args, " ")
	}
	args := make([]string, len(c.Args))
	for i, arg := range c.Args {
		args[i] = arg
		for _, secret := range c.secrets {
			if secret != "" && arg == secret {
				args[i] = secretPlaceholder
				break
			}
		}
	}
	return c.Name + " " + strings.Join(args, " ")
}

// Run executes the command and waits for it to finish.
func (c *Cmd) Run() error {
	ctx := c.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()

	log.Debugf(">> %s", c.logLine())
	cmd := exec.CommandContext(ctx, c.Name, c.Args...)
	c.setupCmd(cmd)

	err := cmd.Run()
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			_ = cmd.Process.Kill()
			return fmt.Errorf("%s %w", err.Error(), ErrCommandTimeout)
		}
		return c.parseError(err, nil)
	}
	return nil
}

// CombinedOutput runs the command and returns its combined standard output and standard error.
func (c *Cmd) CombinedOutput() ([]byte, error) {
	ctx := c.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()

	log.Debugf(">> %s", c.logLine())
	cmd := exec.CommandContext(ctx, c.Name, c.Args...)
	c.setupCmd(cmd)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return output, c.parseError(err, output)
	}
	return output, err
}

func (c *Cmd) setupCmd(cmd *exec.Cmd) {
	if c.Dir != "" {
		cmd.Dir = c.Dir
	}
	if len(c.Env) > 0 {
		cmd.Env = c.Env
	} else {
		cmd.Env = os.Environ()
	}
	cmd.Stdout = c.Stdout
	cmd.Stderr = c.Stderr
	cmd.Stdin = c.Stdin
}

func (c *Cmd) parseError(err error, output []byte) error {
	if output == nil {
		return err
	}

	// Parse error output
	var found []string
	for _, line := range strings.Split(string(output), "\n") {
		s := strings.ToLower(strings.TrimSpace(line))
		if strings.HasPrefix(s, "error:") {
			found = append(found, s)
		}
	}
	if len(found) > 0 {
		return errors.New(strings.Join(found, "\n"))
	}
	return err
}
