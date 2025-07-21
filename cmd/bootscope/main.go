package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"

	intCommands "github.com/px4n/bootscope/internal/commands"
	"github.com/px4n/bootscope/pkg/analyzer"
	"github.com/px4n/bootscope/pkg/collector"
	"github.com/px4n/bootscope/pkg/config"
	"github.com/px4n/bootscope/pkg/errors"
)

var (
	kubeconfig   string
	namespace    string
	kubeContext  string
	outputFormat string
	watch        bool
	timeout      string
	simple       bool
	debug        bool
	configFile   string
)

var rootCmd = &cobra.Command{
	Use:   "kubectl-bootscope",
	Short: "Analyze Kubernetes pod startup times",
	Long: `BootScope - Kubernetes Pod Boot Profiler

⚠️  WARNING: ALPHA SOFTWARE - NOT FOR PRODUCTION USE
This is still in ALPHA stage and may contain bugs, incomplete features, and breaking changes.
Use at your own risk.

BootScope helps identify bottlenecks in pod startup and provides actionable
recommendations for faster deployments.

For more information: https://github.com/px4n/bootscope`,
}

var analyzeCmd = &cobra.Command{
	Use:   "analyze [resource] [name]",
	Short: "Analyze startup time for a pod or all pods in a deployment",
	Long: `Analyze startup time for Kubernetes resources.

Examples:
  # Analyze a single pod
  kubectl bootscope analyze pod nginx-7d4b7c6-x2f3h

  # Analyze all pods in a deployment
  kubectl bootscope analyze deployment nginx-deployment

  # Analyze with watch mode
  kubectl bootscope analyze pod my-pod --watch`,
	Args: cobra.ExactArgs(2),
	RunE: runAnalyze,
}

// Version variables are set by goreleaser
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
	builtBy = "unknown"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("bootscope %s\n", version)
		fmt.Printf("  commit: %s\n", commit)
		fmt.Printf("  built:  %s\n", date)
		fmt.Printf("  by:     %s\n", builtBy)
		fmt.Println("\n⚠️  WARNING: ALPHA SOFTWARE - NOT FOR PRODUCTION USE")
	},
}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration",
}

var configGenerateCmd = &cobra.Command{
	Use:   "generate [path]",
	Short: "Generate a default configuration file",
	Long: `Generate a default configuration file with all available options and documentation.

If no path is specified, creates bootscope.toml in the current directory.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := "./bootscope.toml"
		if len(args) > 0 {
			path = args[0]
		}

		if err := config.SaveDefaultConfig(path); err != nil {
			return errors.WrapFailure("generate config", err)
		}

		fmt.Printf("Configuration file generated at: %s\n", path)
		return nil
	},
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().StringVar(&kubeconfig, "kubeconfig", "", "Path to kubeconfig file")
	rootCmd.PersistentFlags().StringVarP(&namespace, "namespace", "n", "default", "Kubernetes namespace")
	rootCmd.PersistentFlags().StringVar(&kubeContext, "context", "", "Kubernetes context to use")
	rootCmd.PersistentFlags().StringVar(&configFile, "config", "", "Path to configuration file (default: ./bootscope.toml, ~/.kube/bootscope.toml)")

	// Analyze command flags
	analyzeCmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text, json, yaml)")
	analyzeCmd.Flags().BoolVarP(&watch, "watch", "w", false, "Watch pod until ready")
	analyzeCmd.Flags().StringVar(&timeout, "timeout", "", "Timeout for watch mode (default from config or 5m)")
	analyzeCmd.Flags().BoolVar(&simple, "simple", false, "Simple output for developers")
	analyzeCmd.Flags().BoolVar(&debug, "debug", false, "Show debug information including raw timestamps")

	rootCmd.AddCommand(analyzeCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(configCmd)

	configCmd.AddCommand(configGenerateCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runAnalyze(cmd *cobra.Command, args []string) error {
	resourceType := args[0]
	resourceName := args[1]

	// Load configuration
	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		return errors.WrapFailure("load config", err)
	}

	client, err := createK8sClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	collectorConfig := &collector.Config{
		WatchPollInterval:     cfg.GetWatchPollInterval(),
		NetworkSpeedAverage:   cfg.GetNetworkSpeedAverage(),
		NetworkSpeedFast:      cfg.GetNetworkSpeedFast(),
		FastPullThreshold:     cfg.GetFastPullThreshold(),
		LocalRegistryHosts:    cfg.Registry.LocalRegistryHosts,
		PrivateNetworkCIDRs:   cfg.Registry.PrivateNetworkCIDRs,
		ClusterDomainSuffixes: cfg.Registry.ClusterDomainSuffixes,
	}
	coll := collector.NewCollectorWithConfig(client, collectorConfig)
	anal := analyzer.NewAnalyzerWithConfig(cfg)

	ctx := context.Background()

	switch resourceType {
	case "pod":
		return intCommands.AnalyzePod(ctx, coll, anal, cfg, namespace, resourceName, watch, timeout, outputFormat, simple, debug)
	case "deployment", "deploy":
		return intCommands.AnalyzeDeployment(ctx, client, coll, anal, cfg, namespace, resourceName)
	default:
		return fmt.Errorf("unsupported resource type: %s (supported: pod, deployment)", resourceType)
	}
}

func createK8sClient() (kubernetes.Interface, error) {
	if kubeconfig == "" {
		if home := homedir.HomeDir(); home != "" {
			kubeconfig = filepath.Join(home, ".kube", "config")
		}
	}

	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if kubeconfig != "" {
		loadingRules.ExplicitPath = kubeconfig
	}

	configOverrides := &clientcmd.ConfigOverrides{}
	if kubeContext != "" {
		configOverrides.CurrentContext = kubeContext
	}

	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

	config, err := kubeConfig.ClientConfig()
	if err != nil {
		return nil, err
	}

	// If namespace wasn't explicitly set, try to get it from current context
	if namespace == "default" {
		ns, _, err := kubeConfig.Namespace()
		if err == nil && ns != "" {
			namespace = ns
		}
	}

	return kubernetes.NewForConfig(config)
}
