package gitopsjob

import (
	"context"
	"fmt"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

var gitopsJobImage = "quay.io/redhat-appstudio/gitops-generator:latest"

type GitOpsOperation string

const (
	GenerateBase     GitOpsOperation = "generate-base"
	GenerateOverlays GitOpsOperation = "generate-overlays"
)

type GitOpsJobConfig struct {
	OperationType GitOpsOperation
	RepositoryURL string
	ResourceName  string
	Branch        string
	Context       string
}

func (o GitOpsOperation) String() string {
	switch o {
	case GenerateBase:
		return "generate-base"
	case GenerateOverlays:
		return "generate-overlays"
	}
	return "unknown"
}

// CreateGitOpsJob creates a Kubernetes Job to run
func CreateGitOpsJob(ctx context.Context, client ctrlclient.Client, gitToken, jobName, jobNamespace, resourceNamespace string, gitopsConfig GitOpsJobConfig) error {
	gitopsJob := batchv1.Job{
		TypeMeta: v1.TypeMeta{
			APIVersion: "batch/v1",
			Kind:       "Job",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      jobName,
			Namespace: jobNamespace,
			Labels: map[string]string{
				"resourceName": gitopsConfig.ResourceName,
				"operation":    gitopsConfig.OperationType.String(),
			},
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					ServiceAccountName: "application-service-controller-manager",
					RestartPolicy:      corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:            "gitops-generator",
							Image:           gitopsJobImage,
							ImagePullPolicy: corev1.PullAlways,
							Env: []corev1.EnvVar{
								{
									Name:  "OPERATION",
									Value: gitopsConfig.OperationType.String(),
								},
								{
									Name:  "REPOURL",
									Value: gitopsConfig.RepositoryURL,
								},
								{
									Name:  "RESOURCE",
									Value: gitopsConfig.ResourceName,
								},
								{
									Name:  "BRANCH",
									Value: gitopsConfig.Branch,
								},
								{
									Name:  "CONTEXT",
									Value: gitopsConfig.Context,
								},
								{
									Name:  "NAMESPACE",
									Value: resourceNamespace,
								},
								{
									Name: "GITHUB_TOKEN",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "has-github-token",
											},
											Key: "token",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	err := client.Create(ctx, &gitopsJob)
	return err
}

// WaitForJob waits for the given Kubernetes job to complete.
// If it errors out, or the given timeout is reached, an error is returned
func WaitForJob(ctx context.Context, client ctrlclient.Client, jobName string, jobNamespace string, timeout time.Duration) error {
	var job batchv1.Job
	var err error
	for stay, timeout := true, time.After(timeout); stay; {
		err = client.Get(context.Background(), types.NamespacedName{Namespace: jobNamespace, Name: jobName}, &job)
		if err != nil {
			// If the error is anything but a isnotfound error, return the error
			// If the resource wasn't found, keep trying up to the timeout, in case the job hasn't appeared yet
			if !k8sErrors.IsNotFound(err) {
				return err
			}
		}

		// The CompletionTime in the job's status will only get set when the Job completes successfully, so check for its presence
		if job.Status.CompletionTime != nil {
			return nil
		}
		if job.Status.Failed >= 5 {
			return fmt.Errorf("job failed to complete due to error")
		}
		time.Sleep(1 * time.Second)
		select {
		case <-timeout:
			stay = false
		default:
		}
	}

	if err != nil && k8sErrors.IsNotFound(err) {
		return fmt.Errorf("gitops generation job was not found after timeout reached")
	} else if err != nil {
		return err
	}

	// ToDo: Capture pod logs
	return fmt.Errorf("gitops generation job did not complete in time")

}

// Delete job takes in the given gitops job and attempts to delete it
func DeleteJob(ctx context.Context, client ctrlclient.Client, jobName string, jobNamespace string) error {
	job := batchv1.Job{
		TypeMeta: v1.TypeMeta{
			APIVersion: "batch/v1",
			Kind:       "Job",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      jobName,
			Namespace: jobNamespace,
		},
	}
	return client.Delete(context.Background(), &job, &ctrlclient.DeleteOptions{})
}
