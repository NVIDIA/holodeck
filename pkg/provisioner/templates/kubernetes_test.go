package templates

import (
	"bytes"
	"testing"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"

	"github.com/stretchr/testify/assert"
)

func TestIsLegacyKubernetesVersion(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    bool
	}{
		{
			name:    "legacy version v1.31.0",
			version: "v1.31.0",
			want:    true,
		},
		{
			name:    "legacy version v1.30.0",
			version: "v1.30.0",
			want:    true,
		},
		{
			name:    "supported version v1.32.0",
			version: "v1.32.0",
			want:    false,
		},
		{
			name:    "supported version v1.32.1",
			version: "v1.32.1",
			want:    false,
		},
		{
			name:    "invalid version",
			version: "invalid",
			want:    false,
		},
		{
			name:    "empty version",
			version: "",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isLegacyKubernetesVersion(tt.version)
			assert.Equal(t, tt.want, got)
		})
	}
}

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
				UseLegacyInit:         true,
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
				UseLegacyInit:         true,
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
				UseLegacyInit:         true,
				CriSocket:             "unix:///run/cri-dockerd.sock",
			},
			wantErr: false,
		},
		{
			name: "invalid container runtime",
			env: v1alpha1.Environment{
				Spec: v1alpha1.EnvironmentSpec{
					Kubernetes: v1alpha1.Kubernetes{
						KubernetesVersion: "v1.30.0",
					},
					ContainerRuntime: v1alpha1.ContainerRuntime{
						Name: "invalid",
					},
				},
			},
			want:    nil,
			wantErr: true,
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
			wantErr:       false,
			checkSafeExit: true,
		},
		{
			name: "legacy kubeadm installer",
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
			expectedString: "kubeadm init \\\n  --kubernetes-version=${K8S_VERSION} \\\n  --pod-network-cidr=192.168.0.0/16 \\\n  --control-plane-endpoint=test-host:6443 \\\n  --ignore-preflight-errors=all",
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

			if tt.checkSafeExit {
				assert.Contains(t, out, "exit 0", "template output missing safe exit")
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
