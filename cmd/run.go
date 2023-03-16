/*
Copyright © 2023 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"github-merge-manager/config"
	"github-merge-manager/merge"

	"github.com/spf13/cobra"
)

// runCmd represents the run command
var runCmd = &cobra.Command{
	Use:   "run",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		h := merge.NewManager(config.AppConfig)
		h.Handle()
	},
}

func init() {
	rootCmd.AddCommand(runCmd)
	runCmd.Flags().StringVarP(&config.AppConfig.OrgName, "org-name", "o", "sous-chefs", "The name of the organisation to mark as read, defaults to sous-chefs")
	runCmd.Flags().StringVarP(&config.AppConfig.SubjectMatcher, "subject-matcher", "s", "Automated PR: Standardising Files", "The PR subject to search for, defaults to: `Automated PR: Standardising Files`")
	runCmd.Flags().BoolVarP(&config.AppConfig.DryRun, "dry-run", "d", false, "Dry run, don't actually mark anything as read")
	runCmd.Flags().StringVarP(&config.AppConfig.Author, "author", "a", "kitchen-porter", "The author of the PR, defaults to: `kitchen-porter`")
	runCmd.Flags().StringVarP(&config.AppConfig.Action, "action", "c", "approve", "The action to take (csv) allowed are: approve,force-merge, defaults to: `approve`")
	runCmd.Flags().StringVarP(&config.AppConfig.MergeType, "merge-type", "m", "squash", "The type of merge to use if merging, allowed are: squash,merge,rebase, defaults to: `squash`")
	runCmd.Flags().StringVarP(&config.AppConfig.MergeMsgPrefix, "merge-message-prefix", "p", "", "Any prefix to add to the merge message")

	config.AppConfig.Validate()
}
