package gitopsjob

import (
	"context"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap/zapcore"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	testclient "k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

type Client struct {
	Clientset kubernetes.Interface
}

func TestCreateGitOpsJob(t *testing.T) {

	jobName := "test-job"
	jobNamespace := "test-namespace"

	fakeClient := fake.NewClientBuilder().Build()

	// Create an additional Kube client where the job already exists, so that CreateGitOpsJob() will return err
	fakeErrClient := fake.NewClientBuilder().WithObjects(&batchv1.Job{
		ObjectMeta: v1.ObjectMeta{
			Name:      jobName,
			Namespace: jobNamespace,
		},
	}).Build()

	tests := []struct {
		name    string
		client  ctrlclient.Client
		wantErr bool
	}{
		{
			name:    "simple gitops job configuration, no errors",
			client:  fakeClient,
			wantErr: false,
		},
		{
			name:    "job already exists, err returned",
			client:  fakeErrClient,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CreateGitOpsJob(context.Background(), tt.client, "", jobName, jobNamespace, jobNamespace, GitOpsJobConfig{})
			if tt.wantErr != (err != nil) {
				t.Errorf("TestCreateGitOpsJob() unexpected error: %v", err)
			}
		})
	}

}

func TestWaitForJob(t *testing.T) {

	jobNamespace := "default"
	log := zap.New(zap.UseFlagOptions(&zap.Options{
		Development: true,
		TimeEncoder: zapcore.ISO8601TimeEncoder,
	}))

	fakeClient := fake.NewClientBuilder().Build()
	testEnv := &envtest.Environment{}

	cfg, err := testEnv.Start()
	if err != nil {
		panic(err)
	}
	fakeClientSet, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		panic(err)
	}

	fakeLogsClientset := Client{
		Clientset: testclient.NewSimpleClientset(),
	}

	tests := []struct {
		name          string
		componentName string
		client        ctrlclient.Client
		clientset     kubernetes.Interface
		timeout       time.Duration
		wantErr       bool
		wantErrMsg    string
	}{
		{
			name:          "simple gitops job configuration, no errors",
			componentName: "test-gitops-success",
			client:        fakeClient,
			clientset:     fakeClientSet,
			timeout:       time.Minute,
			wantErr:       false,
		},
		{
			name:          "gitops job fails - unable to retrieve logs, default error message shown",
			componentName: "test-gitops-fail",
			client:        fakeClient,
			clientset:     fakeLogsClientset.Clientset,
			timeout:       time.Minute,
			wantErr:       true,
			wantErrMsg:    "gitops generation job failed",
		},
		{
			name:          "gitops job fails - logs retrieved",
			componentName: "test-gitops-fail-two",
			client:        fakeClient,
			clientset:     fakeLogsClientset.Clientset,
			timeout:       time.Minute,
			wantErr:       true,
			wantErrMsg:    "fake log",
		},
		{
			name:          "timeout failure - not found",
			componentName: "test-timeout-fail",
			client:        fakeClient,
			clientset:     fakeClientSet,
			timeout:       time.Second,
			wantErr:       true,
			wantErrMsg:    "gitops generation job was not found after timeout reached",
		},
		{
			name:          "timeout failure - never finishes",
			componentName: "test-never-finishes",
			client:        fakeClient,
			clientset:     fakeClientSet,
			timeout:       time.Second,
			wantErr:       true,
			wantErrMsg:    "gitops generation job did not complete in time",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var err error
			if tt.name != "timeout failure - not found" {
				err := CreateGitOpsJob(context.Background(), tt.client, "", tt.componentName, jobNamespace, jobNamespace, GitOpsJobConfig{
					ResourceName:  tt.componentName,
					OperationType: "generate-base",
				})
				if err != nil {
					t.Errorf("TestCreateGitOpsJob() unexpected error: %v", err)
				}

				// Mark the job as completed
				if tt.name != "timeout failure - never finishes" {
					go updateJobToCompleteOrPanic(fakeClient, fakeClientSet, tt.componentName, jobNamespace, "generate-base", !tt.wantErr)
				}

				// In the scenario where
				if tt.name == "gitops job fails - logs retrieved" {
					jobPod := corev1.Pod{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "v1",
							Kind:       "Pod",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:      tt.componentName,
							Namespace: jobNamespace,
							Labels: map[string]string{
								"job-name": tt.componentName,
							},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "container",
									Image: "image:latest",
								},
							},
						},
					}

					_, err := tt.clientset.CoreV1().Pods(jobNamespace).Create(context.Background(), &jobPod, metav1.CreateOptions{})
					if err != nil {
						t.Errorf("TestDeleteJob() unexpected error: %v", err)
					}
				}

			}

			err = WaitForJob(log, context.Background(), tt.client, tt.clientset, tt.componentName, jobNamespace, tt.timeout)
			if tt.wantErr != (err != nil) {
				t.Errorf("TestWaitForJob() unexpected error: %v", err)
			}
			if tt.wantErr {
				if !strings.Contains(err.Error(), tt.wantErrMsg) {
					t.Errorf("TestWaitForJob() expected error message: %v, got %v", tt.wantErrMsg, err.Error())
				}
			}
		})
	}

}

func TestDeleteJob(t *testing.T) {

	jobName := "test-job"
	jobNamespace := "test-namespace"

	fakeClient := fake.NewClientBuilder().WithObjects(&batchv1.Job{
		ObjectMeta: v1.ObjectMeta{
			Name:      jobName,
			Namespace: jobNamespace,
		},
	}).Build()

	// Create an additional fake client without a Job to test that DeleteJob() properly errors out when no job exists
	fakeErrClient := fake.NewClientBuilder().Build()

	tests := []struct {
		name    string
		client  ctrlclient.Client
		wantErr bool
	}{
		{
			name:    "simple gitops job configuration, no errors",
			client:  fakeClient,
			wantErr: false,
		},
		{
			name:    "job already exists, err returned",
			client:  fakeErrClient,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := DeleteJob(context.Background(), tt.client, jobName, jobNamespace)
			if tt.wantErr != (err != nil) {
				t.Errorf("TestDeleteJob() unexpected error: %v", err)
			}
		})
	}

}

func TestGetPodLogs(t *testing.T) {

	jobNamespace := "default"

	fakeClient := fake.NewClientBuilder().Build()
	testEnv := &envtest.Environment{}

	cfg, err := testEnv.Start()
	if err != nil {
		panic(err)
	}
	fakeClientSet, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		panic(err)
	}

	fakeLogsClientset := Client{
		Clientset: testclient.NewSimpleClientset(),
	}
	fakeClientset := Client{
		Clientset: fakeClientSet,
	}

	tests := []struct {
		name          string
		componentName string
		jobName       string
		client        ctrlclient.Client
		clientset     kubernetes.Interface
		timeout       time.Duration
		wantErr       bool
		wantErrMsg    string
	}{
		{
			name:          "simple gitops job configuration, no errors",
			componentName: "test-gitops-success",
			jobName:       "test-gitops-success",
			client:        fakeClient,
			clientset:     fakeLogsClientset.Clientset,
			timeout:       time.Minute,
			wantErr:       false,
		},
		{
			name:          "no pod exists",
			componentName: "test-no-pod",
			jobName:       "test-no-pod",
			client:        fakeClient,
			clientset:     fakeClientset.Clientset,
			timeout:       time.Minute,
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			// Create the Job
			err := CreateGitOpsJob(context.Background(), tt.client, "", tt.jobName, jobNamespace, jobNamespace, GitOpsJobConfig{
				ResourceName:  tt.componentName,
				OperationType: "generate-overlays",
			})
			if err != nil {
				t.Errorf("TestDeleteJob() unexpected error: %v", err)
			}

			// For all test cases except where the job pod needs to be missing, create the pod
			if tt.name != "no pod exists" {
				jobPod := corev1.Pod{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Pod",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      tt.componentName,
						Namespace: "default",
						Labels: map[string]string{
							"job-name": tt.jobName,
						},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "container",
								Image: "image:latest",
							},
						},
					},
				}

				_, err := tt.clientset.CoreV1().Pods(jobNamespace).Create(context.Background(), &jobPod, metav1.CreateOptions{})
				if err != nil {
					t.Errorf("TestDeleteJob() unexpected error: %v", err)
				}
			}

			// Now try to get the logs
			_, err = GetPodLogs(context.Background(), tt.client, tt.clientset, tt.jobName, jobNamespace)
			if tt.wantErr != (err != nil) {
				t.Errorf("TestDeleteJob() unexpected error: %v", err)
			}
		})
	}

}

func updateJobToCompleteOrPanic(kubeclient client.Client, clientset *kubernetes.Clientset, componentName string, componentNamespace string, operation string, isSuccess bool) {
	jobList := &batchv1.JobList{}
	for stay, timeout := true, time.After(10*time.Second); stay; {
		err := kubeclient.List(context.Background(), jobList, &client.ListOptions{LabelSelector: labels.SelectorFromSet(labels.Set{
			"resourceName": componentName,
			"operation":    operation,
		})})

		if err != nil {
			// If the error is anything but a isnotfound error, return the error
			// If the resource wasn't found, keep trying up to the timeout, in case the job hasn't appeared yet
			if !k8sErrors.IsNotFound(err) {
				panic(err)
			}
		}
		if err == nil && len(jobList.Items) > 0 {
			break
		}

		time.Sleep(1 * time.Second)
		select {
		case <-timeout:
			stay = false
		default:
		}
	}

	gitOpsJob := jobList.Items[0]

	if isSuccess || strings.Contains(componentName, "test-git-error") {
		gitOpsJob.Status.CompletionTime = &metav1.Time{Time: time.Now()}
	} else {
		// Before setting the Job to failed, create a fake pod to get the logs from
		jobPod := corev1.Pod{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Pod",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      componentName,
				Namespace: "default",
				Labels: map[string]string{
					"job-name": gitOpsJob.Name,
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "container",
						Image: "image:latest",
					},
				},
			},
		}
		_, err := clientset.CoreV1().Pods("default").Create(context.Background(), &jobPod, metav1.CreateOptions{})
		if err != nil {
			panic(err)
		}
		gitOpsJob.Status.Failed = 5
	}

	err := kubeclient.Status().Update(context.Background(), &gitOpsJob)
	if err != nil {
		panic(err)
	}
}
