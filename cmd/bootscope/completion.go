package main

import (
	"github.com/spf13/cobra"
)

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate completion script",
	Long: `To load completions:

Bash:

  $ source <(kubectl bootscope completion bash)

  # To load completions for each session, execute once:
  # Linux:
  $ kubectl bootscope completion bash > /etc/bash_completion.d/kubectl-bootscope
  # macOS:
  $ kubectl bootscope completion bash > $(brew --prefix)/etc/bash_completion.d/kubectl-bootscope

Zsh:

  # If shell completion is not already enabled in your environment,
  # you will need to enable it.  You can execute the following once:

  $ echo "autoload -U compinit; compinit" >> ~/.zshrc

  # To load completions for each session, execute once:
  $ kubectl bootscope completion zsh > "${fpath[1]}/_kubectl-bootscope"

  # You will need to start a new shell for this setup to take effect.

Fish:

  $ kubectl bootscope completion fish | source

  # To load completions for each session, execute once:
  $ kubectl bootscope completion fish > ~/.config/fish/completions/kubectl-bootscope.fish

PowerShell:

  PS> kubectl bootscope completion powershell | Out-String | Invoke-Expression

  # To load completions for every new session, run:
  PS> kubectl bootscope completion powershell > kubectl-bootscope.ps1
  # and source this file from your PowerShell profile.
`,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "bash":
			return cmd.Root().GenBashCompletion(cmd.OutOrStdout())
		case "zsh":
			return cmd.Root().GenZshCompletion(cmd.OutOrStdout())
		case "fish":
			return cmd.Root().GenFishCompletion(cmd.OutOrStdout(), true)
		case "powershell":
			return cmd.Root().GenPowerShellCompletionWithDesc(cmd.OutOrStdout())
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(completionCmd)
	addCustomCompletions()
}

// addCustomCompletions adds dynamic completion for pod and deployment names
func addCustomCompletions() {
	analyzeCmd.ValidArgsFunction = func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 0 {
			return []string{"pod", "deployment", "deploy"}, cobra.ShellCompDirectiveNoFileComp
		}

		if len(args) == 1 {
			// For now, return empty to allow any input
			// TODO: In future autocomplete from kube resources would be nice
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	_ = analyzeCmd.RegisterFlagCompletionFunc("output", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"text", "json", "yaml"}, cobra.ShellCompDirectiveNoFileComp
	})

	// Config generate path completion allows files
	configGenerateCmd.ValidArgsFunction = func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return nil, cobra.ShellCompDirectiveDefault
	}
}
