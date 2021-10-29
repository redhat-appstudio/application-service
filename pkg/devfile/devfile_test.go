package devfile

import (
	"reflect"
	"testing"

	"github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/api/v2/pkg/attributes"
	"github.com/devfile/api/v2/pkg/devfile"
	data "github.com/devfile/library/pkg/devfile/parser/data"
	v2 "github.com/devfile/library/pkg/devfile/parser/data/v2"
	appstudiov1alpha1 "github.com/redhat-appstudio/application-service/api/v1alpha1"
)

func TestParseDevfileModel(t *testing.T) {
	tests := []struct {
		name          string
		devfileString string
		appModelRepo  string
		gitOpsRepo    string
		wantDevfile   *v2.DevfileV2
	}{
		{
			name: "Simple HASApp CR",
			devfileString: `
metadata:
  attributes:
    appModelRepository.url: https://github.com/testorg/petclinic-app
    gitOpsRepository.url: https://github.com/testorg/petclinic-gitops
  name: petclinic
schemaVersion: 2.2.0`,
			appModelRepo: "https://github.com/testorg/petclinic-app",
			gitOpsRepo:   "https://github.com/testorg/petclinic-gitops",
			wantDevfile: &v2.DevfileV2{
				Devfile: v1alpha2.Devfile{
					DevfileHeader: devfile.DevfileHeader{
						SchemaVersion: string(data.APISchemaVersion220),
						Metadata: devfile.DevfileMetadata{
							Name:       "petclinic",
							Attributes: attributes.Attributes{}.PutString("gitOpsRepository.url", "https://github.com/testorg/petclinic-gitops").PutString("appModelRepository.url", "https://github.com/testorg/petclinic-app"),
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Convert the hasApp resource to a devfile
			devfile, err := ParseDevfileModel(tt.devfileString)
			if err != nil {
				t.Errorf("TestConvertHASApplicationToDevfile() unexpected error: %v", err)
			} else if !reflect.DeepEqual(devfile, tt.wantDevfile) {
				t.Errorf("TestConvertHASApplicationToDevfile() error: expected %v got %v", tt.wantDevfile, devfile)
			}
		})
	}
}

func TestConvertHASApplicationToDevfile(t *testing.T) {
	additionalAttributes := attributes.Attributes{}.PutString("appModelRepository.branch", "testbranch").PutString("gitOpsRepository.branch", "testbranch").PutString("appModelRepository.context", "test/context").PutString("gitOpsRepository.context", "test/context")

	tests := []struct {
		name         string
		hasApp       appstudiov1alpha1.HASApplication
		appModelRepo string
		gitOpsRepo   string
		wantDevfile  *v2.DevfileV2
	}{
		{
			name: "Simple HASApp CR",
			hasApp: appstudiov1alpha1.HASApplication{
				Spec: appstudiov1alpha1.HASApplicationSpec{
					DisplayName: "Petclinic",
				},
			},
			appModelRepo: "https://github.com/testorg/petclinic-app",
			gitOpsRepo:   "https://github.com/testorg/petclinic-gitops",
			wantDevfile: &v2.DevfileV2{
				Devfile: v1alpha2.Devfile{
					DevfileHeader: devfile.DevfileHeader{
						SchemaVersion: string(data.APISchemaVersion220),
						Metadata: devfile.DevfileMetadata{
							Name:       "Petclinic",
							Attributes: attributes.Attributes{}.PutString("gitOpsRepository.url", "https://github.com/testorg/petclinic-gitops").PutString("appModelRepository.url", "https://github.com/testorg/petclinic-app"),
						},
					},
				},
			},
		},
		{
			name: "HASApp CR with branch and context fields set",
			hasApp: appstudiov1alpha1.HASApplication{
				Spec: appstudiov1alpha1.HASApplicationSpec{
					DisplayName: "Petclinic",
					AppModelRepository: appstudiov1alpha1.HASApplicationGitRepository{
						Branch:  "testbranch",
						Context: "test/context",
					},
					GitOpsRepository: appstudiov1alpha1.HASApplicationGitRepository{
						Branch:  "testbranch",
						Context: "test/context",
					},
				},
			},
			appModelRepo: "https://github.com/testorg/petclinic-app",
			gitOpsRepo:   "https://github.com/testorg/petclinic-gitops",
			wantDevfile: &v2.DevfileV2{
				Devfile: v1alpha2.Devfile{
					DevfileHeader: devfile.DevfileHeader{
						SchemaVersion: string(data.APISchemaVersion220),
						Metadata: devfile.DevfileMetadata{
							Name:       "Petclinic",
							Attributes: additionalAttributes.PutString("gitOpsRepository.url", "https://github.com/testorg/petclinic-gitops").PutString("appModelRepository.url", "https://github.com/testorg/petclinic-app"),
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Convert the hasApp resource to a devfile
			convertedDevfile, err := ConvertHASApplicationToDevfile(tt.hasApp, tt.gitOpsRepo, tt.appModelRepo)
			if err != nil {
				t.Errorf("TestConvertHASApplicationToDevfile() unexpected error: %v", err)
			} else if !reflect.DeepEqual(convertedDevfile, tt.wantDevfile) {
				t.Errorf("TestConvertHASApplicationToDevfile() error: expected %v got %v", tt.wantDevfile, convertedDevfile)
			}
		})
	}
}
