package controllers

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/devfile/library/v2/pkg/devfile/parser/data/v2/common"
	appstudiov1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
	"github.com/redhat-appstudio/application-service/pkg/devfile"
	"github.com/redhat-appstudio/application-service/pkg/github"
	"go.uber.org/zap/zapcore"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var appModelDevfile = `
metadata:
  attributes:
    appModelRepository.context: ./
    appModelRepository.url: https://github.com/test-org/fakerepo
    gitOpsRepository.context: ./
    gitOpsRepository.url: https://github.com/test-org/fakerepo
  description: application definition for petclinic-app
  name: petclinic
schemaVersion: 2.2.0
`

var invalidAppModelDevfileNoGitOps = `
metadata:
  attributes:
    appModelRepository.context: ./
    appModelRepository.url: https://github.com/test-org/fakerepo
  description: application definition for petclinic-app
  name: petclinic
schemaVersion: 2.2.0
`

var invalidAppModelDevfileNoAppModel = `
metadata:
  attributes:
    gitOpsRepository.context: ./
    gitOpsRepository.url: https://github.com/test-org/fakerepo
  description: application definition for petclinic-app
  name: petclinic
schemaVersion: 2.2.0
`

func TestEnsureGitOpsRepoExists(t *testing.T) {
	application := appstudiov1alpha1.Application{
		ObjectMeta: v1.ObjectMeta{
			Name:      "myapp",
			Namespace: "default",
		},
		Spec: appstudiov1alpha1.ApplicationSpec{
			DisplayName: "SampleApp",
			Description: "My application",
		},
	}

	emptyGitStruct := appstudiov1alpha1.ApplicationGitRepository{}

	mockTokenClient := github.MockGitHubTokenClient{}
	mockGHClient, _ := mockTokenClient.GetNewGitHubClient("")
	tests := []struct {
		name    string
		adapter ApplicationAdapter
		wantErr bool
	}{
		{
			name: "Simple application component, no errors",
			adapter: ApplicationAdapter{
				Application:    &application,
				NamespacedName: types.NamespacedName{Name: "myapp", Namespace: "default"},
				GithubOrg:      "test-org",
				GitHubClient:   mockGHClient,
				Client:         fake.NewClientBuilder().Build(),
				Ctx:            context.Background(),
			},
			wantErr: false,
		},
		{
			name: "Simple application component - gitops repo creation failure",
			adapter: ApplicationAdapter{
				Application: &appstudiov1alpha1.Application{
					ObjectMeta: v1.ObjectMeta{
						Name:      "test-error-response",
						Namespace: "default",
					},
					Spec: appstudiov1alpha1.ApplicationSpec{
						DisplayName: "SampleApp",
						Description: "My application",
					},
				},
				NamespacedName: types.NamespacedName{Name: "test-error-response", Namespace: "default"},
				GithubOrg:      "test-error-response",
				GitHubClient:   mockGHClient,
				Client:         fake.NewClientBuilder().Build(),
				Ctx:            context.Background(),
				Log: zap.New(zap.UseFlagOptions(&zap.Options{
					Development: true,
					TimeEncoder: zapcore.ISO8601TimeEncoder,
				})),
			},
			wantErr: true,
		},
		{
			name: "Simple application component - both git repositories set",
			adapter: ApplicationAdapter{
				Application: &appstudiov1alpha1.Application{
					ObjectMeta: v1.ObjectMeta{
						Name:      "myapp",
						Namespace: "default",
					},
					Spec: appstudiov1alpha1.ApplicationSpec{
						DisplayName: "SampleApp",
						Description: "My application",
						GitOpsRepository: appstudiov1alpha1.ApplicationGitRepository{
							URL:     "https://github.com/myorg/mygitops",
							Context: "path/",
							Branch:  "otherbranch",
						},
						AppModelRepository: appstudiov1alpha1.ApplicationGitRepository{
							URL:     "https://github.com/myorg/appmodel",
							Context: "path/",
							Branch:  "otherbranch",
						},
					},
				},
				NamespacedName: types.NamespacedName{Name: "myapp", Namespace: "default"},
				GithubOrg:      "test-org",
				GitHubClient:   mockGHClient,
				Client:         fake.NewClientBuilder().Build(),
				Ctx:            context.Background(),
			},
			wantErr: false,
		},
		{
			name: "Application with devfile model already set",
			adapter: ApplicationAdapter{
				Application: &appstudiov1alpha1.Application{
					ObjectMeta: v1.ObjectMeta{
						Name:      "myapp",
						Namespace: "default",
					},
					Spec: appstudiov1alpha1.ApplicationSpec{
						DisplayName: "SampleApp",
						Description: "My application",
					},
					Status: appstudiov1alpha1.ApplicationStatus{
						Devfile: appModelDevfile,
					},
				},
				NamespacedName: types.NamespacedName{Name: "myapp", Namespace: "default"},
				GithubOrg:      "test-org",
				GitHubClient:   mockGHClient,
				Client:         fake.NewClientBuilder().Build(),
				Ctx:            context.Background(),
				Log: zap.New(zap.UseFlagOptions(&zap.Options{
					Development: true,
					TimeEncoder: zapcore.ISO8601TimeEncoder,
				})),
			},
			wantErr: false,
		},
		{
			name: "Application with invalid devfile model - no gitops repository",
			adapter: ApplicationAdapter{
				Application: &appstudiov1alpha1.Application{
					ObjectMeta: v1.ObjectMeta{
						Name:      "myapp",
						Namespace: "default",
					},
					Spec: appstudiov1alpha1.ApplicationSpec{
						DisplayName: "SampleApp",
						Description: "My application",
					},
					Status: appstudiov1alpha1.ApplicationStatus{
						Devfile: invalidAppModelDevfileNoGitOps,
					},
				},
				NamespacedName: types.NamespacedName{Name: "myapp", Namespace: "default"},
				GithubOrg:      "test-org",
				GitHubClient:   mockGHClient,
				Client:         fake.NewClientBuilder().Build(),
				Ctx:            context.Background(),
				Log: zap.New(zap.UseFlagOptions(&zap.Options{
					Development: true,
					TimeEncoder: zapcore.ISO8601TimeEncoder,
				})),
			},
			wantErr: true,
		},
		{
			name: "Application with invalid devfile model - no appmodel repository",
			adapter: ApplicationAdapter{
				Application: &appstudiov1alpha1.Application{
					ObjectMeta: v1.ObjectMeta{
						Name:      "myapp",
						Namespace: "default",
					},
					Spec: appstudiov1alpha1.ApplicationSpec{
						DisplayName: "SampleApp",
						Description: "My application",
					},
					Status: appstudiov1alpha1.ApplicationStatus{
						Devfile: invalidAppModelDevfileNoAppModel,
					},
				},
				NamespacedName: types.NamespacedName{Name: "myapp", Namespace: "default"},
				GithubOrg:      "test-org",
				GitHubClient:   mockGHClient,
				Client:         fake.NewClientBuilder().Build(),
				Ctx:            context.Background(),
				Log: zap.New(zap.UseFlagOptions(&zap.Options{
					Development: true,
					TimeEncoder: zapcore.ISO8601TimeEncoder,
				})),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.adapter.EnsureGitOpsRepoExists()
			if (err != nil) != tt.wantErr {
				t.Errorf("TestEnsureGitOpsRepoExists(): unexpected error: %v", err)
			}
			if err == nil {
				// GitOps and AppModel repositories should be set properly
				if tt.adapter.GitOpsRepository == emptyGitStruct {
					t.Error("TestEnsureGitOpsRepoExists(): expected GitOps Repository to not be nil")
				}
				if tt.adapter.Application.Spec.GitOpsRepository == emptyGitStruct {
					if !strings.Contains(tt.adapter.GitOpsRepository.URL, tt.adapter.GithubOrg) {
						t.Errorf("TestEnsureGitOpsRepoExists(): expected GitOps Repository struct to not be empty")
					}
				} else {
					if !reflect.DeepEqual(tt.adapter.Application.Spec.GitOpsRepository, tt.adapter.GitOpsRepository) {
						t.Errorf("TestEnsureGitOpsRepoExists(): expected GitOpsRepository to be %v, but got %v", tt.adapter.Application.Spec.GitOpsRepository, tt.adapter.GitOpsRepository)
					}
				}
				if tt.adapter.Application.Spec.AppModelRepository == emptyGitStruct {
					if !reflect.DeepEqual(tt.adapter.GitOpsRepository, tt.adapter.AppModelRepository) {
						t.Errorf("TestEnsureGitOpsRepoExists(): expected AppModelRepository to be %v, but got %v", tt.adapter.GitOpsRepository, tt.adapter.AppModelRepository)
					}
				} else {
					if !reflect.DeepEqual(tt.adapter.Application.Spec.AppModelRepository, tt.adapter.AppModelRepository) {
						t.Errorf("TestEnsureGitOpsRepoExists(): expected AppModelRepository to be %v, but got %v", tt.adapter.Application.Spec.AppModelRepository, tt.adapter.AppModelRepository)
					}
				}
			}
		})
	}

}

func TestEnsureApplicationDevfile(t *testing.T) {
	log := zap.New(zap.UseFlagOptions(&zap.Options{
		Development: true,
		TimeEncoder: zapcore.ISO8601TimeEncoder,
	}))

	application := appstudiov1alpha1.Application{
		ObjectMeta: v1.ObjectMeta{
			Name:      "myapp",
			Namespace: "default",
		},
		Spec: appstudiov1alpha1.ApplicationSpec{
			DisplayName: "SampleApp",
			Description: "My application",
		},
	}

	mockTokenClient := github.MockGitHubTokenClient{}
	mockGHClient, _ := mockTokenClient.GetNewGitHubClient("")
	tests := []struct {
		name    string
		adapter ApplicationAdapter
		wantErr bool
	}{
		{
			name: "Simple application with no components, no errors",
			adapter: ApplicationAdapter{
				Application:    &application,
				NamespacedName: types.NamespacedName{Name: "myapp", Namespace: "default"},
				GithubOrg:      "fakeorg",
				GitHubClient:   mockGHClient,
				Client:         fake.NewClientBuilder().Build(),
				Ctx:            context.Background(),
				GitOpsRepository: appstudiov1alpha1.ApplicationGitRepository{
					URL: "https://github.com/fakeorg/fakerepo",
				},
				AppModelRepository: appstudiov1alpha1.ApplicationGitRepository{
					URL: "https://github.com/fakeorg/fakerepo",
				},
			},
			wantErr: false,
		},
		{
			name: "Simple application with two git components, no errors",
			adapter: ApplicationAdapter{
				Application: &application,
				Components: []appstudiov1alpha1.Component{
					{
						Spec: appstudiov1alpha1.ComponentSpec{
							ComponentName: "test-component1",
							Application:   "myapp",
							Source: appstudiov1alpha1.ComponentSource{
								ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
									GitSource: &appstudiov1alpha1.GitSource{
										URL: "https://github.com/fake/fake",
									},
								},
							},
						},
					},
					{
						Spec: appstudiov1alpha1.ComponentSpec{
							ComponentName: "test-component2",
							Application:   "myapp",
							Source: appstudiov1alpha1.ComponentSource{
								ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
									GitSource: &appstudiov1alpha1.GitSource{
										URL: "https://github.com/fake/faketwo",
									},
								},
							},
						},
					},
				},
				NamespacedName: types.NamespacedName{Name: "myapp", Namespace: "default"},
				GithubOrg:      "fakeorg",
				GitHubClient:   mockGHClient,
				Client:         fake.NewClientBuilder().Build(),
				Ctx:            context.Background(),
				GitOpsRepository: appstudiov1alpha1.ApplicationGitRepository{
					URL: "https://github.com/fakeorg/fakerepo",
				},
				AppModelRepository: appstudiov1alpha1.ApplicationGitRepository{
					URL: "https://github.com/fakeorg/fakerepo",
				},
				Log: log,
			},
			wantErr: false,
		},
		{
			name: "Application with multiple git and image components, no errors",
			adapter: ApplicationAdapter{
				Application: &application,
				Components: []appstudiov1alpha1.Component{
					{
						Spec: appstudiov1alpha1.ComponentSpec{
							ComponentName: "test-component1",
							Application:   "myapp",
							Source: appstudiov1alpha1.ComponentSource{
								ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
									GitSource: &appstudiov1alpha1.GitSource{
										URL: "https://github.com/fake/fake",
									},
								},
							},
						},
					},
					{
						Spec: appstudiov1alpha1.ComponentSpec{
							ComponentName: "test-component2",
							Application:   "myapp",
							Source: appstudiov1alpha1.ComponentSource{
								ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
									GitSource: &appstudiov1alpha1.GitSource{
										URL: "https://github.com/fake/faketwo",
									},
								},
							},
						},
					},
					{
						Spec: appstudiov1alpha1.ComponentSpec{
							ComponentName:  "test-component3",
							Application:    "myapp",
							ContainerImage: "quay.io/fake/fakeimage:latest",
						},
					},
					{
						Spec: appstudiov1alpha1.ComponentSpec{
							ComponentName:  "test-component4",
							Application:    "myapp",
							ContainerImage: "quay.io/fake/fakeimagefour:latest",
						},
					},
				},
				NamespacedName: types.NamespacedName{Name: "myapp", Namespace: "default"},
				GithubOrg:      "fakeorg",
				GitHubClient:   mockGHClient,
				Client:         fake.NewClientBuilder().Build(),
				Ctx:            context.Background(),
				GitOpsRepository: appstudiov1alpha1.ApplicationGitRepository{
					URL: "https://github.com/fakeorg/fakerepo",
				},
				AppModelRepository: appstudiov1alpha1.ApplicationGitRepository{
					URL: "https://github.com/fakeorg/fakerepo",
				},
				Log: log,
			},
			wantErr: false,
		},
		{
			name: "Simple application with duplicate git components, should return error",
			adapter: ApplicationAdapter{
				Application: &application,
				Components: []appstudiov1alpha1.Component{
					{
						Spec: appstudiov1alpha1.ComponentSpec{
							ComponentName: "test-component",
							Application:   "myapp",
							Source: appstudiov1alpha1.ComponentSource{
								ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
									GitSource: &appstudiov1alpha1.GitSource{
										URL: "https://github.com/fake/fake",
									},
								},
							},
						},
					},
					{
						Spec: appstudiov1alpha1.ComponentSpec{
							ComponentName: "test-component",
							Application:   "myapp",
							Source: appstudiov1alpha1.ComponentSource{
								ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
									GitSource: &appstudiov1alpha1.GitSource{
										URL: "https://github.com/fake/fake",
									},
								},
							},
						},
					},
				},
				NamespacedName: types.NamespacedName{Name: "myapp", Namespace: "default"},
				GithubOrg:      "fakeorg",
				GitHubClient:   mockGHClient,
				Client:         fake.NewClientBuilder().Build(),
				Ctx:            context.Background(),
				GitOpsRepository: appstudiov1alpha1.ApplicationGitRepository{
					URL: "https://github.com/fakeorg/fakerepo",
				},
				AppModelRepository: appstudiov1alpha1.ApplicationGitRepository{
					URL: "https://github.com/fakeorg/fakerepo",
				},
				Log: log,
			},
			wantErr: true,
		},
		{
			name: "Simple application with duplicate image components, should return error",
			adapter: ApplicationAdapter{
				Application: &application,
				Components: []appstudiov1alpha1.Component{
					{
						Spec: appstudiov1alpha1.ComponentSpec{
							ComponentName:  "test-component",
							Application:    "myapp",
							ContainerImage: "quay.io/fake/fakeimage:latest",
						},
					},
					{
						Spec: appstudiov1alpha1.ComponentSpec{
							ComponentName:  "test-component",
							Application:    "myapp",
							ContainerImage: "quay.io/fake/fakeimage:latest",
						},
					},
				},
				NamespacedName: types.NamespacedName{Name: "myapp", Namespace: "default"},
				GithubOrg:      "fakeorg",
				GitHubClient:   mockGHClient,
				Client:         fake.NewClientBuilder().Build(),
				Ctx:            context.Background(),
				GitOpsRepository: appstudiov1alpha1.ApplicationGitRepository{
					URL: "https://github.com/fakeorg/fakerepo",
				},
				AppModelRepository: appstudiov1alpha1.ApplicationGitRepository{
					URL: "https://github.com/fakeorg/fakerepo",
				},
				Log: log,
			},
			wantErr: true,
		},
		{
			name: "Application with commponents with no source, should return error",
			adapter: ApplicationAdapter{
				Application: &application,
				Components: []appstudiov1alpha1.Component{
					{
						Spec: appstudiov1alpha1.ComponentSpec{
							ComponentName: "test-component",
						},
					},
					{
						Spec: appstudiov1alpha1.ComponentSpec{
							ComponentName: "test-component2",
						},
					},
				},
				NamespacedName: types.NamespacedName{Name: "myapp", Namespace: "default"},
				GithubOrg:      "fakeorg",
				GitHubClient:   mockGHClient,
				Client:         fake.NewClientBuilder().Build(),
				Ctx:            context.Background(),
				GitOpsRepository: appstudiov1alpha1.ApplicationGitRepository{
					URL: "https://github.com/fakeorg/fakerepo",
				},
				AppModelRepository: appstudiov1alpha1.ApplicationGitRepository{
					URL: "https://github.com/fakeorg/fakerepo",
				},
				Log: log,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.adapter.EnsureApplicationDevfile()
			if (err != nil) != tt.wantErr {
				t.Errorf("TestEnsureApplicationDevfile(): unexpected error: %v", err)
			}

			if err == nil {
				if tt.adapter.Application.Status.Devfile == "" {
					t.Error("TestEnsureApplicationDevfile(): expected Application devfile model to not be empty")
				}

				// Parse the devfile and retrieve the list of components
				// GitSource components will be listed under "Projects"
				// Image source components will be attributes prefixed with "containerImage/"
				devfileStr := tt.adapter.Application.Status.Devfile
				devfileSrc := devfile.DevfileSrc{
					Data: devfileStr,
				}
				devfileData, err := devfile.ParseDevfile(devfileSrc)
				if err != nil {
					t.Errorf("TestEnsureApplicationDevfile(): unexpected error parsing Application devfile model: %v", err)
				}
				devfileProjects, err := devfileData.GetProjects(common.DevfileOptions{})
				if err != nil {
					t.Errorf("TestEnsureApplicationDevfile(): unexpected error parsing Application devfile projects: %v", err)
				}
				devfileAttributes, err := devfileData.GetAttributes()
				if err != nil {
					t.Errorf("TestEnsureApplicationDevfile(): unexpected error parsing Application devfile attributes: %v", err)
				}
				devfileStrAttributes := devfileAttributes.Strings(&err)
				if err != nil {
					t.Errorf("TestEnsureApplicationDevfile(): unexpected error parsing Application devfile string attributes: %v", err)
				}

				for _, component := range tt.adapter.Components {
					compFound := false
					if component.Spec.Source.GitSource != nil {
						for _, project := range devfileProjects {
							if project.Git != nil && project.Git.Remotes["origin"] == component.Spec.Source.GitSource.URL {
								compFound = true
								break
							}
						}
						if !compFound {
							t.Errorf("TestEnsureApplicationDevfile(): git source component %v not found in Application devfile model %v", component.Spec.Source.GitSource.URL, devfileStr)
						}
					} else if component.Spec.ContainerImage != "" {
						for key := range devfileStrAttributes {
							if ("containerImage/"+component.Spec.ComponentName) == key && devfileStrAttributes[key] == component.Spec.ContainerImage {
								compFound = true
								break
							}
						}
						if !compFound {
							t.Errorf("TestEnsureApplicationDevfile(): image component component %v not found in Application devfile model %v", component.Spec.ContainerImage, devfileStr)
						}
					}
				}

			}
		})
	}

}

func TestEnsureApplicationStatus(t *testing.T) {
	application := &appstudiov1alpha1.Application{
		TypeMeta: v1.TypeMeta{
			APIVersion: "appstudio.redhat.com/v1alpha1",
			Kind:       "Application",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      "myapp-one",
			Namespace: "default",
		},
		Spec: appstudiov1alpha1.ApplicationSpec{
			DisplayName: "SampleApp",
			Description: "My application",
		},
	}

	applicationWithStatus := &appstudiov1alpha1.Application{
		TypeMeta: v1.TypeMeta{
			APIVersion: "appstudio.redhat.com/v1alpha1",
			Kind:       "Application",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      "myapp-other",
			Namespace: "default",
		},
		Spec: appstudiov1alpha1.ApplicationSpec{
			DisplayName: "SampleApp",
			Description: "My application",
		},
		Status: appstudiov1alpha1.ApplicationStatus{
			Conditions: []v1.Condition{
				{
					Type:   "Created",
					Status: v1.ConditionTrue,
				},
			},
		},
	}

	fakeClient := NewFakeClient(t, application, applicationWithStatus)
	log := zap.New(zap.UseFlagOptions(&zap.Options{
		Development: true,
		TimeEncoder: zapcore.ISO8601TimeEncoder,
	}))

	mockTokenClient := github.MockGitHubTokenClient{}
	mockGHClient, _ := mockTokenClient.GetNewGitHubClient("")
	tests := []struct {
		name    string
		adapter ApplicationAdapter
		err     error
	}{
		{
			name: "Simple application component, no errors",
			adapter: ApplicationAdapter{
				Application:    application,
				NamespacedName: types.NamespacedName{Name: "myapp-one", Namespace: "default"},
				GithubOrg:      "test-org",
				GitHubClient:   mockGHClient,
				Client:         fakeClient,
				Ctx:            context.Background(),
				Log:            log,
			},
		},
		{
			name: "Application component with a status condition already",
			adapter: ApplicationAdapter{
				Application:    applicationWithStatus,
				NamespacedName: types.NamespacedName{Name: "myapp-other", Namespace: "default"},
				GithubOrg:      "test-org",
				GitHubClient:   mockGHClient,
				Client:         fakeClient,
				Ctx:            context.Background(),
				Log:            log,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.adapter.EnsureApplicationStatus()
			if err != nil {
				t.Errorf("TestEnsureApplicationStatus(): unexpected error: %v", err)
			}
			if err == nil {
				updatedApplication := appstudiov1alpha1.Application{}
				tt.adapter.Client.Get(tt.adapter.Ctx, tt.adapter.NamespacedName, &updatedApplication, &client.GetOptions{})
				if len(tt.adapter.Application.Status.Conditions) == 1 {
					if len(updatedApplication.Status.Conditions) != 1 {
						t.Errorf("TestEnsureApplicationStatus(): Expected Application status conditions %v to have length %v, but got %v", updatedApplication.Status.Conditions, 1, len(updatedApplication.Status.Conditions))
					}
				} else {
					if len(updatedApplication.Status.Conditions) != 2 {
						t.Errorf("TestEnsureApplicationStatus(): Expected Application status conditions %v to have length %v, but got %v", updatedApplication.Status.Conditions, 2, len(updatedApplication.Status.Conditions))
					}
				}
				tt.adapter.Client.Delete(tt.adapter.Ctx, &updatedApplication, &client.DeleteOptions{})
			}

		})
	}

}
