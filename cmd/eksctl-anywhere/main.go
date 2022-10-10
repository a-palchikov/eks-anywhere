package main

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/aws/eks-anywhere/cmd/eksctl-anywhere/cmd"
	"github.com/aws/eks-anywhere/pkg/eksctl"
	"github.com/aws/eks-anywhere/pkg/logger"
)

func main() {
	sigChannel := make(chan os.Signal, 1)
	signal.Notify(sigChannel, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChannel
		logger.Info("Warning: Terminating this operation may leave the cluster in an irrecoverable state")
		os.Exit(-1)
	}()
	if eksctl.Enabled() {
		err := eksctl.ValidateVersion()
		if err != nil {
			fmt.Println(err)
			os.Exit(-1)
		}
	}
	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "[!]Error: %+v\n", errors.Unwrap(err))
		os.Exit(-1)
	}
	os.Exit(0)
}
