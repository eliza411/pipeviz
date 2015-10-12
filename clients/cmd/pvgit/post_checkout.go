package main

import "github.com/tag1consulting/pipeviz/Godeps/_workspace/src/github.com/spf13/cobra"

func postCheckoutHookCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hook-post-checkout",
		Short: "Processes a git post-checkout hook event.",
		Run:   runPostCheckout,
	}

	return cmd
}

func runPostCheckout(cmd *cobra.Command, args []string) {
	if args[2] == "0" {
		// if flag at third arg is zero, it means it's a file checkout; we do nothing
		return
	}

	repo := getRepoOrExit()
	msgmap := make(map[string]interface{})
	recordHead(msgmap, repo)
	sendMapToPipeviz(msgmap, repo)
}
