package commands

import (
	"context"
	"fmt"
	"os"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	intOutput "github.com/px4n/bootscope/internal/output"
	"github.com/px4n/bootscope/pkg/analyzer"
	"github.com/px4n/bootscope/pkg/collector"
	"github.com/px4n/bootscope/pkg/config"
	"github.com/px4n/bootscope/pkg/types"
)

func AnalyzeDeployment(ctx context.Context, client kubernetes.Interface, coll *collector.Collector, anal *analyzer.Analyzer, cfg *config.Config, namespace, deploymentName string) error {
	if err := validateKubernetesName(namespace, "namespace"); err != nil {
		return err
	}
	if err := validateKubernetesName(deploymentName, "deployment"); err != nil {
		return err
	}

	pods, err := getDeploymentPods(ctx, client, namespace, deploymentName)
	if err != nil {
		return err
	}

	profiles, err := analyzeDeploymentPods(ctx, coll, anal, namespace, pods)
	if err != nil {
		return err
	}

	return intOutput.DeploymentAnalysisWithNamespace(namespace, deploymentName, profiles, cfg)
}

func getDeploymentPods(ctx context.Context, client kubernetes.Interface, namespace, deploymentName string) ([]v1.Pod, error) {
	deployment, err := client.AppsV1().Deployments(namespace).Get(ctx, deploymentName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get deployment: %w", err)
	}

	labelSelector := metav1.FormatLabelSelector(deployment.Spec.Selector)
	pods, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	if len(pods.Items) == 0 {
		return nil, fmt.Errorf("no pods found for deployment %s", deploymentName)
	}

	return pods.Items, nil
}

func analyzeDeploymentPods(ctx context.Context, coll *collector.Collector, anal *analyzer.Analyzer, namespace string, pods []v1.Pod) ([]*types.PodStartupProfile, error) {
	var profiles []*types.PodStartupProfile
	for _, pod := range pods {
		podInfo, err := coll.CollectPodInfo(ctx, namespace, pod.Name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to collect info for pod %s: %v\n", pod.Name, err)
			continue
		}

		profile, err := anal.AnalyzePod(podInfo)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to analyze pod %s: %v\n", pod.Name, err)
			continue
		}

		profiles = append(profiles, profile)
	}

	if len(profiles) == 0 {
		return nil, fmt.Errorf("failed to analyze any pods")
	}

	return profiles, nil
}
