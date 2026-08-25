package k8sorchestrator

import (
	"context"
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
)

type SnapsentryOrchestrator struct {
	client kubernetes.Interface
}

func NewSnapsentryOrchestrator(config *rest.Config) (*SnapsentryOrchestrator, error) {
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &SnapsentryOrchestrator{client: client}, nil
}

func (s *SnapsentryOrchestrator) listDeploymentHelper(
	ctx context.Context,
	namespace string,
	options metav1.ListOptions,

) ([]appsv1.Deployment, error) {

	deploymentClient := s.client.AppsV1().Deployments(namespace)

	snapsentryDeployments, err := deploymentClient.List(ctx, options)
	if err != nil {
		return nil, err
	}
	return snapsentryDeployments.Items, nil
}

func (s *SnapsentryOrchestrator) CreateOrUpdateCloudsSecret(
	ctx context.Context,
	namespace string,
	projectInfo ProjectInfo,
	openstackCloudData string,
) (*corev1.Secret, error) {

	secretClient := s.client.CoreV1().Secrets(namespace)
	secretName := fmt.Sprintf("snapsentry-%s", projectInfo.ProjectID)

	labels := map[string]string{
		"project-id":   strings.ToLower(projectInfo.ProjectID),
		"project-name": strings.ToLower(projectInfo.ProjectName),
		"domain-id":    strings.ToLower(projectInfo.DomainID),
	}

	secret := &corev1.Secret{
		Type: corev1.SecretTypeOpaque,
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
			Labels:    labels,
		},
		StringData: map[string]string{
			"clouds.yaml": openstackCloudData,
		},
	}

	existingSecret, err := secretClient.Get(ctx, secret.Name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return secretClient.Create(ctx, secret, metav1.CreateOptions{})
		}
		return nil, err
	}

	// Secret exists, update it.
	// We must set the ResourceVersion to the existing one to allow the update
	secret.ResourceVersion = existingSecret.ResourceVersion
	return secretClient.Update(ctx, secret, metav1.UpdateOptions{})

}

func (s *SnapsentryOrchestrator) ListDeployments(
	ctx context.Context,
	namespace string,
) ([]appsv1.Deployment, error) {

	// All of the snapsentry deployments created with orchestrator uses the label
	// app=snapsentry-go.
	options := metav1.ListOptions{
		LabelSelector: "app=snapsentry-go",
	}
	return s.listDeploymentHelper(ctx, namespace, options)
}

func (s *SnapsentryOrchestrator) ListProjectDeployment(
	ctx context.Context,
	namespace string,
	projectInfo ProjectInfo,
) ([]appsv1.Deployment, error) {

	labelSelector := fmt.Sprintf(
		"app=snapsentry-go,project-id=%s,project-name=%s,domain-id=%s",
		strings.ToLower(projectInfo.ProjectID),
		strings.ToLower(projectInfo.ProjectName),
		strings.ToLower(projectInfo.DomainID),
	)
	options := metav1.ListOptions{
		LabelSelector: labelSelector,
	}

	// Ideally only one deployment should be returned for a given project,
	// but just in case we return a list future usecases.
	return s.listDeploymentHelper(ctx, namespace, options)
}

func (s *SnapsentryOrchestrator) CreateSnapsentryController(
	ctx context.Context,
	config *DeploymentConfig,
) error {

	if err := config.Validate(); err != nil {
		return fmt.Errorf("CreateSnapsentryController validation failed for %s: %s", config.ProjectInfo, err.Error())
	}

	labels := map[string]string{
		"project-name": strings.ToLower(config.ProjectInfo.ProjectName),
		"project-id":   strings.ToLower(config.ProjectInfo.ProjectID),
		"domain-id":    strings.ToLower(config.ProjectInfo.DomainID),
		"app":          "snapsentry-go",
	}

	deploymentClient := s.client.AppsV1().Deployments(config.Namespace)

	reqCPUQuantity, err := resource.ParseQuantity(config.RequestsCPU)
	if err != nil {
		return fmt.Errorf("invalid requests CPU quantity: %w", err)
	}
	reqMemQuantity, err := resource.ParseQuantity(config.RequestsMemory)
	if err != nil {
		return fmt.Errorf("invalid requests memory quantity: %w", err)
	}
	limCPUQuantity, err := resource.ParseQuantity(config.LimitCPU)
	if err != nil {
		return fmt.Errorf("invalid limits CPU quantity: %w", err)
	}
	limMemQuantity, err := resource.ParseQuantity(config.LimitMemory)
	if err != nil {
		return fmt.Errorf("invalid limits memory quantity: %w", err)
	}

	deploymentName := fmt.Sprintf("snapsentry-%s", config.ProjectInfo.ProjectID)
	cloudName := fmt.Sprintf(
		"snapsentry-%s-%s", config.ProjectInfo.ProjectName, config.ProjectInfo.ProjectID,
	) // Opinitated. Secret should be created like this.
	entryPoint := []string{
		"daemon",
		"--cloud",
		cloudName,
		"--log-level",
		config.LogLevel,
	}

	if config.CreationCron != "" {
		entryPoint = append(entryPoint, "--create-schedule", config.CreationCron)
	}

	if config.ExpiryCron != "" {
		entryPoint = append(entryPoint, "--expire-schedule", config.ExpiryCron)
	}

	if config.WebhookProvider.URL != "" {
		entryPoint = append(entryPoint, "--webhook-url", config.WebhookProvider.URL)
	}

	if config.WebhookProvider.URL != "" && config.WebhookProvider.Username != "" && config.WebhookProvider.Password != "" {
		entryPoint = append(
			entryPoint,
			"--webhook-username", config.WebhookProvider.Username,
			"--webhook-password", config.WebhookProvider.Password,
		)
	}

	snapsentryController := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName,
			Namespace: config.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.To(int32(1)),
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RecreateDeploymentStrategyType,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            "snapsentry-go",
							Image:           config.Image,
							Args:            entryPoint,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    limCPUQuantity,
									corev1.ResourceMemory: limMemQuantity,
								},
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    reqCPUQuantity,
									corev1.ResourceMemory: reqMemQuantity,
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
									SecretName:  fmt.Sprintf("snapsentry-%s", config.ProjectInfo.ProjectID),
									DefaultMode: ptr.To(int32(0644)),
								},
							},
						},
					},
				},
			},
		},
	}

	_, err = deploymentClient.Create(ctx, snapsentryController, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create deployment: %w", err)
	}

	return nil
}
