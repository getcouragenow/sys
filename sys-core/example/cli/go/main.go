package main

import (
	"github.com/spf13/cobra"
	"log"

	sharedpkg "go.amplifyedge.org/sys-share-v2/sys-core/service/go/pkg"
)

var rootCmd = &cobra.Command{
	Use:   "core",
	Short: "core cli",
}

func main() {
	rootCmd.AddCommand(
		sharedpkg.NewSysCoreProxyClient().CobraCommand(),
		sharedpkg.NewSysBusProxyClient().CobraCommand(),
	)
	if err := rootCmd.Execute(); err != nil {
		log.Fatalf("command failed: %v", err)
	}
}
