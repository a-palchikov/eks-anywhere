package executables

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/aws/eks-anywhere/pkg/config"
	"github.com/aws/eks-anywhere/pkg/constants"
	"github.com/aws/eks-anywhere/pkg/logger"
	"github.com/aws/eks-anywhere/pkg/providers/cloudstack/decoder"
)

const (
	redactMask = "*****"
)

var redactedEnvKeys = []string{
	constants.VSphereUsernameKey,
	constants.VSpherePasswordKey,
	constants.GovcUsernameKey,
	constants.GovcPasswordKey,
	decoder.CloudStackCloudConfigB64SecretKey,
	eksaGithubTokenEnv,
	githubTokenEnv,
	config.EksaAccessKeyIdEnv,
	config.EksaSecretAccessKeyEnv,
	config.AwsAccessKeyIdEnv,
	config.AwsSecretAccessKeyEnv,
	constants.SnowCredentialsKey,
	constants.SnowCertsKey,
}

type executable struct {
	cmd []string
}

type Executable interface {
	Execute(ctx context.Context, args ...string) (stdout bytes.Buffer, err error)
	ExecuteWithEnv(ctx context.Context, envs map[string]string, args ...string) (stdout bytes.Buffer, err error) // TODO: remove this from interface in favor of Command
	ExecuteWithStdin(ctx context.Context, in []byte, args ...string) (stdout bytes.Buffer, err error)            // TODO: remove this from interface in favor of Command
	Command(ctx context.Context, args ...string) *Command
	Run(cmd *Command) (stdout bytes.Buffer, err error)
}

// this should only be called through the executables.builder
func NewExecutable(cmd ...string) Executable {
	return &executable{
		cmd: cmd,
	}
}

func (e *executable) Execute(ctx context.Context, args ...string) (stdout bytes.Buffer, err error) {
	return e.Command(ctx, args...).Run()
}

func (e *executable) ExecuteWithStdin(ctx context.Context, in []byte, args ...string) (stdout bytes.Buffer, err error) {
	return e.Command(ctx, args...).WithStdIn(in).Run()
}

func (e *executable) ExecuteWithEnv(ctx context.Context, envs map[string]string, args ...string) (stdout bytes.Buffer, err error) {
	return e.Command(ctx, args...).WithEnvVars(envs).Run()
}

func (e *executable) Command(ctx context.Context, args ...string) *Command {
	return NewCommand(ctx, e, args...)
}

func (e *executable) Run(cmd *Command) (stdout bytes.Buffer, err error) {
	for k, v := range cmd.envVars {
		os.Setenv(k, v)
	}
	args := append(e.cmd[1:], cmd.args...)
	return execute(cmd.ctx, e.cmd[0], cmd.stdIn, cmd.envVars, args)
}

func (e *executable) Close(ctx context.Context) error {
	return nil
}

func RedactCreds(cmd string, envMap map[string]string) string {
	redactedEnvs := []string{}
	for _, redactedEnvKey := range redactedEnvKeys {
		if env, found := envMap[redactedEnvKey]; found {
			redactedEnvs = append(redactedEnvs, env)
		}
	}

	for _, redactedEnv := range redactedEnvs {
		cmd = strings.ReplaceAll(cmd, redactedEnv, redactMask)
	}
	return cmd
}

func execute(ctx context.Context, cli string, in []byte, envVars map[string]string, args []string) (stdout bytes.Buffer, err error) {
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, cli, args...)
	logger.V(1).Info("Executing command", "cmd", RedactCreds(cmd.String(), envVars))
	cmd.Stdout = io.MultiWriter(&stdout, os.Stdout)
	cmd.Stderr = io.MultiWriter(&stderr, os.Stderr)
	if len(in) != 0 {
		cmd.Stdin = io.TeeReader(bytes.NewReader(in), &dumpingWriter{})
	}

	err = cmd.Run()
	if err != nil {
		if stderr.Len() > 0 {
			if logger.MaxLogging() {
				logger.V(logger.MaxLoggingLevel()).Info(cli, "stderr", stderr.String())
			}
			return stdout, errors.New(stderr.String())
		} else {
			if !logger.MaxLogging() {
				logger.V(8).Info(cli, "stdout", stdout.String())
				logger.V(8).Info(cli, "stderr", stderr.String())
			}
			return stdout, errors.New(fmt.Sprint(err))
		}
	}
	if !logger.MaxLogging() {
		logger.V(8).Info(cli, "stdout", stdout.String())
		logger.V(8).Info(cli, "stderr", stderr.String())
	}
	return stdout, nil
}

func (r *dumpingWriter) Write(p []byte) (n int, err error) {
	if !r.wroteHeader {
		fmt.Println("[[[start of stdin dump]]]")
		r.wroteHeader = true
	}
	return os.Stdout.Write(p)
}

type dumpingWriter struct {
	wroteHeader bool
}
