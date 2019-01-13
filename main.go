package main

import (
	"os"

	"github.com/spf13/pflag"

	"github.com/leoh0/kubectl-check-cert/cmd"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

func main() {
	flags := pflag.NewFlagSet("kubectl-check-cert", pflag.ExitOnError)
	pflag.CommandLine = flags

	root := cmd.NewCmdExpiration(
		genericclioptions.IOStreams{
			In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr})
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
