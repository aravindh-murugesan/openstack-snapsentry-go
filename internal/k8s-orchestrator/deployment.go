package k8sorchestrator

import (
	"context"
	"fmt"
	"strings"

	"github.com/aravindh-murugesan/openstack-snapsentry-go/internal/notifications"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
)

func ListSnapsentryDeployment(
	ctx context.Context,
	config *rest.Config,
	namespace string,
) ([]appsv1.Deployment, error) {

	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		return []appsv1.Deployment{}, err
	}

	deploymentClient := clientSet.AppsV1().Deployments(namespace)

	options := metav1.ListOptions{
		LabelSelector: "app=snapsentry-go",
	}

	snapsentryDeployments, err := deploymentClient.List(
		ctx,
		options,
	)

	if err != nil {
		return []appsv1.Deployment{}, err
	}

	return snapsentryDeployments.Items, nil
}

func GetSnapsentryDeployment(
	ctx context.Context,
	config *rest.Config,
	namespace string,
	tenantID string,
	tenantName string,
	domainID string,
) ([]appsv1.Deployment, error) {

	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		return []appsv1.Deployment{}, err
	}

	deploymentClient := clientSet.AppsV1().Deployments(namespace)

	options := metav1.ListOptions{
		LabelSelector: fmt.Sprintf("app=snapsentry-go,project-id=%s,project-name=%s,domain-id=%s", tenantID, tenantName, domainID),
	}

	snapsentryDeployments, err := deploymentClient.List(
		ctx,
		options,
	)

	if err != nil {
		return []appsv1.Deployment{}, err
	}

	return snapsentryDeployments.Items, nil
}

func CreateSnapsentrySecret(
	ctx context.Context,
	config *rest.Config,
	namespace string,
	projectID string,
	projectName string,
	domainID string,
	data string,
) (*corev1.Secret, error) {

	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		return &corev1.Secret{}, err
	}
	secretClient := clientSet.CoreV1().Secrets(namespace)

	secretName := fmt.Sprintf("snapsentry-creds-%s", strings.ToLower(projectID))

	secret := &corev1.Secret{
		Type: corev1.SecretTypeOpaque,
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
			Labels: map[string]string{
				"project-name": strings.ToLower(projectName),
				"project-id":   projectID,
				"domain-id":    domainID,
			},
		},
		StringData: map[string]string{
			"clouds.yaml": data, // Use a descriptive key for the data
		},
	}

	createdSecret, secretCreateErr := secretClient.Create(ctx, secret, metav1.CreateOptions{})

	if errors.IsAlreadyExists(secretCreateErr) {
		return createdSecret, secretCreateErr
	}

	return createdSecret, nil
}

func CreateSnapsentryDeployment(
	ctx context.Context,
	config *rest.Config,
	namespace string,
	projectID string,
	projectName string,
	domainID string,
	rCPU string,
	rMemory string,
	lCPU string,
	lMemory string,
	snapsentryImage string,
	webhookProvider notifications.Webhook,
) (*appsv1.Deployment, error) {
	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		return &appsv1.Deployment{}, err
	}

	deploymentLabels := map[string]string{
		"project-name": strings.ToLower(projectName),
		"project-id":   projectID,
		"domain-id":    domainID,
		"app":          "snapsentry-go",
	}

	deploymentClient := clientSet.AppsV1().Deployments(namespace)

	// Some sane defaults for snapsentry
	if rCPU == "" {
		rCPU = "64m"
	}
	if rMemory == "" {
		rMemory = "32Mi"
	}
	if lCPU == "" {
		lCPU = "256m"
	}
	if lMemory == "" {
		lMemory = "128Mi"
	}

	generatedDeploymentName := fmt.Sprintf("snapsentry-%s", projectID)
	snapsentryRunCommand := []string{
		"daemon",
		"--cloud",
		fmt.Sprintf("snapsentry-%s-%s", projectName, projectID),
		"--log-level",
		"info",
		"--expire-schedule",
		"0 */1 * * *",
	}

	if webhookProvider.URL != "" {
		snapsentryRunCommand = append(snapsentryRunCommand, "--webhook-url", webhookProvider.URL)
	}

	if webhookProvider.Username != "" {
		snapsentryRunCommand = append(snapsentryRunCommand, "--webhook-username", webhookProvider.Username)
	}

	if webhookProvider.Password != "" {
		snapsentryRunCommand = append(snapsentryRunCommand, "--webhook-password", webhookProvider.Password)
	}

	snapSentryDeployment := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      generatedDeploymentName,
			Namespace: namespace,
			Labels:    deploymentLabels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.To(int32(1)),
			Selector: &metav1.LabelSelector{
				MatchLabels: deploymentLabels,
			},
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RecreateDeploymentStrategyType,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: deploymentLabels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "snapsentry-go",
							Image: snapsentryImage,
							Args:  snapsentryRunCommand,
							Env: []corev1.EnvVar{
								{Name: "GOMAXPROCS", Value: "1"},
								{Name: "GOMEMLIMIT", Value: "115MiB"},
							},
							ImagePullPolicy: corev1.PullIfNotPresent,
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse(lCPU),
									corev1.ResourceMemory: resource.MustParse(lMemory),
								},
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse(rCPU),
									corev1.ResourceMemory: resource.MustParse(rMemory),
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "openstack-clouds-vol",
									MountPath: "/etc/openstack",
									ReadOnly:  true,
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "openstack-clouds-vol",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName:  fmt.Sprintf("snapsentry-creds-%s", projectID),
									DefaultMode: ptr.To(int32(420)),
								},
							},
						},
					},
				},
			},
		},
	}

	snapDeployment, err := deploymentClient.Create(ctx, snapSentryDeployment, metav1.CreateOptions{})
	if err != nil {
		return &appsv1.Deployment{}, err
	}

	return snapDeployment, err
}
