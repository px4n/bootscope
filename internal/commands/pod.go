package commands

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	intOutput "github.com/px4n/bootscope/internal/output"
	"github.com/px4n/bootscope/pkg/analyzer"
	"github.com/px4n/bootscope/pkg/collector"
	"github.com/px4n/bootscope/pkg/config"
	"github.com/px4n/bootscope/pkg/errors"
)

var (
	// Kubernetes name validation regex
	// Must be lowercase alphanumeric or '-', start/end with alphanumeric
	kubeNameRegex = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)
	maxNameLength = 253
)

func validateKubernetesName(name, resourceType string) error {
	if name == "" {
		return fmt.Errorf("%s name cannot be empty", resourceType)
	}
	if len(name) > maxNameLength {
		return fmt.Errorf("%s name too long (max %d characters)", resourceType, maxNameLength)
	}
	if !kubeNameRegex.MatchString(name) {
		return fmt.Errorf("invalid %s name: must be lowercase alphanumeric or '-', and start/end with alphanumeric", resourceType)
	}
	return nil
}

func AnalyzePod(ctx context.Context, coll *collector.Collector, anal *analyzer.Analyzer, cfg *config.Config, namespace, podName string, watch bool, timeout, outputFormat string, simple, debug bool) error {
	if err := validateKubernetesName(namespace, "namespace"); err != nil {
		return err
	}
	if err := validateKubernetesName(podName, "pod"); err != nil {
		return err
	}

	var podInfo *collector.PodInfo
	var err error

	// Collect pod information
	if watch {
		// Use timeout from flag or fall back to config
		timeoutStr := timeout
		if timeoutStr == "" {
			timeoutStr = cfg.Operations.DefaultWatchTimeout
		}

		timeoutDuration, parseErr := time.ParseDuration(timeoutStr)
		if parseErr != nil {
			return fmt.Errorf("invalid timeout: %w", parseErr)
		}

		// Validate timeout is reasonable (between 1s and 1h)
		if timeoutDuration < time.Second {
			return fmt.Errorf("timeout too short (minimum 1s)")
		}
		if timeoutDuration > time.Hour {
			return fmt.Errorf("timeout too long (maximum 1h)")
		}

		fmt.Printf("Watching pod %s/%s for up to %s...\n", namespace, podName, timeoutStr)
		podInfo, err = coll.WatchPod(ctx, namespace, podName, timeoutDuration)
		if err != nil {
			return errors.WrapFailure("watch pod", err)
		}
	} else {
		podInfo, err = coll.CollectPodInfo(ctx, namespace, podName)
		if err != nil {
			return errors.WrapFailure("collect pod info", err)
		}
	}

	profile, err := anal.AnalyzePod(podInfo)
	if err != nil {
		return errors.WrapFailure("analyze pod", err)
	}

	if debug {
		intOutput.Debug(podInfo, profile)
		fmt.Println("\n" + strings.Repeat("=", 80) + "\n")
	}

	switch outputFormat {
	case "json":
		return intOutput.JSON(profile)
	case "yaml":
		return intOutput.YAML(profile)
	default:
		if simple {
			return intOutput.Simple(profile, cfg)
		}
		return intOutput.Text(profile, cfg)
	}
}
