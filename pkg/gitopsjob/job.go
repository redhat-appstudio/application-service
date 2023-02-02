//
// Copyright 2023 Red Hat, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package gitopsjob

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/go-logr/logr"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"

	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
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
	backOffLimit := int32(5)

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
			BackoffLimit: &backOffLimit,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					ServiceAccountName: "application-service-controller-manager",
					RestartPolicy:      corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:            "gitops-generator",
							Image:           gitopsJobImage,
							ImagePullPolicy: corev1.PullAlways,
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("10m"),
									corev1.ResourceMemory: resource.MustParse("50Mi"),
								},
							},
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
func WaitForJob(log logr.Logger, ctx context.Context, client ctrlclient.Client, clientset kubernetes.Interface, jobName string, jobNamespace string, timeout time.Duration) error {
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
			// Attempt to get the Pod logs
			errMsg, err := GetPodLogs(context.Background(), client, clientset, jobName, jobNamespace)
			if err != nil {
				log.Error(err, "unable to retrieve pod logs for job "+jobName)
				return fmt.Errorf("gitops generation job failed")
			} else {
				return fmt.Errorf(errMsg)
			}
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

func GetPodLogs(ctx context.Context, client ctrlclient.Client, clientset kubernetes.Interface, jobName string, jobNamespace string) (string, error) {
	jobPodList, err := clientset.CoreV1().Pods(jobNamespace).List(ctx, v1.ListOptions{LabelSelector: "job-name=" + jobName})
	if len(jobPodList.Items) == 0 || err != nil {
		return "", fmt.Errorf("unable to find pod associated with the Kubernetes job %q in namespace %q", jobName, jobNamespace)
	}

	// Retrieve the pod name
	pod := jobPodList.Items[0]
	podName := pod.Name

	podLogRequest := clientset.CoreV1().Pods(jobNamespace).GetLogs(podName, &corev1.PodLogOptions{})
	stream, err := podLogRequest.Stream(context.TODO())
	if err != nil {
		return "", err
	}
	defer stream.Close()
	var logs string
	for {
		buf := make([]byte, 2000)
		numBytes, err := stream.Read(buf)
		if numBytes == 0 {
			break
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		logs = logs + string(buf[:numBytes])
	}
	return logs, nil

}
