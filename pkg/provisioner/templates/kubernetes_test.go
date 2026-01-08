package templates

import (
	"bytes"
	"testing"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"

	"github.com/stretchr/testify/assert"
)

func TestNewKubernetes(t *testing.T) {
	tests := []struct {
		name    string
		env     v1alpha1.Environment
		want    *Kubernetes
		wantErr bool
	}{
		{
			name: "default values",
			env: v1alpha1.Environment{
				Spec: v1alpha1.EnvironmentSpec{
					Kubernetes: v1alpha1.Kubernetes{
						KubernetesVersion: "v1.30.0",
					},
					ContainerRuntime: v1alpha1.ContainerRuntime{
						Name: "containerd",
					},
				},
			},
			want: &Kubernetes{
				Version:               "v1.30.0",
				KubeletReleaseVersion: defaultKubeletReleaseVersion,
				Arch:                  defaultArch,
				CniPluginsVersion:     defaultCNIPluginsVersion,
				CalicoVersion:         defaultCalicoVersion,
				CrictlVersion:         defaultCRIVersion,
				UseLegacyInit:         true, // v1.30.0 < v1.32.0
				CriSocket:             "unix:///run/containerd/containerd.sock",
			},
			wantErr: false,
		},
		{
			name: "legacy version",
			env: v1alpha1.Environment{
				Spec: v1alpha1.EnvironmentSpec{
					Kubernetes: v1alpha1.Kubernetes{
						KubernetesVersion: "v1.31.0",
					},
					ContainerRuntime: v1alpha1.ContainerRuntime{
						Name: "containerd",
					},
				},
			},
			want: &Kubernetes{
				Version:               "v1.31.0",
				KubeletReleaseVersion: defaultKubeletReleaseVersion,
				Arch:                  defaultArch,
				CniPluginsVersion:     defaultCNIPluginsVersion,
				CalicoVersion:         defaultCalicoVersion,
				CrictlVersion:         defaultCRIVersion,
				UseLegacyInit:         true, // v1.31.0 < v1.32.0
				CriSocket:             "unix:///run/containerd/containerd.sock",
			},
			wantErr: false,
		},
		{
			name: "custom values",
			env: v1alpha1.Environment{
				Spec: v1alpha1.EnvironmentSpec{
					Kubernetes: v1alpha1.Kubernetes{
						KubernetesVersion:     "v1.30.0",
						KubeletReleaseVersion: "v0.18.0",
						Arch:                  "arm64",
						CniPluginsVersion:     "v1.7.0",
						CalicoVersion:         "v3.30.0",
						CrictlVersion:         "v1.32.0",
						K8sFeatureGates:       []string{"Feature1=true", "Feature2=false"},
					},
					ContainerRuntime: v1alpha1.ContainerRuntime{
						Name: "docker",
					},
				},
			},
			want: &Kubernetes{
				Version:               "v1.30.0",
				KubeletReleaseVersion: "v0.18.0",
				Arch:                  "arm64",
				CniPluginsVersion:     "v1.7.0",
				CalicoVersion:         "v3.30.0",
				CrictlVersion:         "v1.32.0",
				K8sFeatureGates:       "Feature1=true,Feature2=false",
				UseLegacyInit:         true,                           // v1.30.0 < v1.32.0
				CriSocket:             "unix:///run/cri-dockerd.sock", // docker runtime
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewKubernetes(tt.env)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestKubernetes_Execute(t *testing.T) {
	tests := []struct {
		name           string
		env            v1alpha1.Environment
		wantErr        bool
		checkTemplate  bool
		expectedString string
		checkSafeExit  bool
	}{
		{
			name: "kubeadm installer",
			env: v1alpha1.Environment{
				Spec: v1alpha1.EnvironmentSpec{
					Kubernetes: v1alpha1.Kubernetes{
						KubernetesVersion:   "v1.30.0",
						KubernetesInstaller: "kubeadm",
					},
					ContainerRuntime: v1alpha1.ContainerRuntime{
						Name: "containerd",
					},
				},
			},
			wantErr:        false,
			checkTemplate:  true,
			expectedString: "Waiting for Tigera operator",
			checkSafeExit:  true,
		},
		{
			name: "kubeadm installer with endpoint",
			env: v1alpha1.Environment{
				Spec: v1alpha1.EnvironmentSpec{
					Kubernetes: v1alpha1.Kubernetes{
						KubernetesVersion:   "v1.31.0",
						KubernetesInstaller: "kubeadm",
						K8sEndpointHost:     "test-host",
					},
					ContainerRuntime: v1alpha1.ContainerRuntime{
						Name: "containerd",
					},
				},
			},
			wantErr:        false,
			checkTemplate:  true,
			expectedString: `--control-plane-endpoint="${K8S_ENDPOINT_HOST}:6443"`,
			checkSafeExit:  true,
		},
		{
			name: "kind installer",
			env: v1alpha1.Environment{
				Spec: v1alpha1.EnvironmentSpec{
					Kubernetes: v1alpha1.Kubernetes{
						KubernetesVersion:   "v1.32.1",
						KubernetesInstaller: "kind",
					},
					ContainerRuntime: v1alpha1.ContainerRuntime{
						Name: "containerd",
					},
				},
			},
			wantErr:       false,
			checkSafeExit: true,
		},
		{
			name: "microk8s installer",
			env: v1alpha1.Environment{
				Spec: v1alpha1.EnvironmentSpec{
					Kubernetes: v1alpha1.Kubernetes{
						KubernetesVersion:   "v1.32.1",
						KubernetesInstaller: "microk8s",
					},
					ContainerRuntime: v1alpha1.ContainerRuntime{
						Name: "containerd",
					},
				},
			},
			wantErr:       false,
			checkSafeExit: true,
		},
		{
			name: "invalid installer",
			env: v1alpha1.Environment{
				Spec: v1alpha1.EnvironmentSpec{
					Kubernetes: v1alpha1.Kubernetes{
						KubernetesVersion:   "v1.32.1",
						KubernetesInstaller: "invalid",
					},
					ContainerRuntime: v1alpha1.ContainerRuntime{
						Name: "containerd",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "kubeadm installer - check CoreDNS wait",
			env: v1alpha1.Environment{
				Spec: v1alpha1.EnvironmentSpec{
					Kubernetes: v1alpha1.Kubernetes{
						KubernetesVersion:   "v1.31.0",
						KubernetesInstaller: "kubeadm",
					},
					ContainerRuntime: v1alpha1.ContainerRuntime{
						Name: "containerd",
					},
				},
			},
			wantErr:        false,
			checkTemplate:  true,
			expectedString: "Waiting for CoreDNS",
			checkSafeExit:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k, err := NewKubernetes(tt.env)
			if err != nil {
				if !tt.wantErr {
					t.Errorf("NewKubernetes() error = %v, wantErr %v", err, tt.wantErr)
				}
				return
			}

			var buf bytes.Buffer
			err = k.Execute(&buf, tt.env)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)

			out := buf.String()
			if tt.checkTemplate {
				assert.Contains(t, out, tt.expectedString)
			}
		})
	}
}

func TestGetCRISocket(t *testing.T) {
	tests := []struct {
		name    string
		runtime string
		want    string
		wantErr bool
	}{
		{
			name:    "docker runtime",
			runtime: "docker",
			want:    "unix:///run/cri-dockerd.sock",
			wantErr: false,
		},
		{
			name:    "containerd runtime",
			runtime: "containerd",
			want:    "unix:///run/containerd/containerd.sock",
			wantErr: false,
		},
		{
			name:    "crio runtime",
			runtime: "crio",
			want:    "unix:///run/crio/crio.sock",
			wantErr: false,
		},
		{
			name:    "invalid runtime",
			runtime: "invalid",
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetCRISocket(tt.runtime)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
