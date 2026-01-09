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

func TestNewKubeadmConfig(t *testing.T) {
	tests := []struct {
		name    string
		env     v1alpha1.Environment
		wantErr bool
		check   func(*testing.T, *KubeadmConfig)
	}{
		{
			name: "default config with containerd",
			env: v1alpha1.Environment{
				Spec: v1alpha1.EnvironmentSpec{
					Kubernetes: v1alpha1.Kubernetes{
						KubernetesVersion: "v1.30.0",
					},
					ContainerRuntime: v1alpha1.ContainerRuntime{
						Name: v1alpha1.ContainerRuntimeContainerd,
					},
				},
			},
			wantErr: false,
			check: func(t *testing.T, c *KubeadmConfig) {
				assert.Equal(t, "v1.30.0", c.KubernetesVersion)
				assert.Equal(t, "unix:///run/containerd/containerd.sock", c.CriSocket)
				assert.Equal(t, "holodeck-cluster", c.ClusterName)
				assert.Equal(t, "192.168.0.0/16", c.PodSubnet)
				assert.True(t, c.IsUbuntu)
			},
		},
		{
			name: "config with endpoint host",
			env: v1alpha1.Environment{
				Spec: v1alpha1.EnvironmentSpec{
					Kubernetes: v1alpha1.Kubernetes{
						KubernetesVersion: "v1.31.0",
						K8sEndpointHost:   "my-endpoint.example.com",
					},
					ContainerRuntime: v1alpha1.ContainerRuntime{
						Name: v1alpha1.ContainerRuntimeContainerd,
					},
				},
			},
			wantErr: false,
			check: func(t *testing.T, c *KubeadmConfig) {
				assert.Equal(t, "my-endpoint.example.com", c.ControlPlaneEndpoint)
			},
		},
		{
			name: "config with feature gates",
			env: v1alpha1.Environment{
				Spec: v1alpha1.EnvironmentSpec{
					Kubernetes: v1alpha1.Kubernetes{
						KubernetesVersion: "v1.30.0",
						K8sFeatureGates:   []string{"Feature1=true", "Feature2=false"},
					},
					ContainerRuntime: v1alpha1.ContainerRuntime{
						Name: v1alpha1.ContainerRuntimeContainerd,
					},
				},
			},
			wantErr: false,
			check: func(t *testing.T, c *KubeadmConfig) {
				assert.Equal(t, "Feature1=true,Feature2=false", c.FeatureGates)
			},
		},
		{
			name: "config with docker runtime",
			env: v1alpha1.Environment{
				Spec: v1alpha1.EnvironmentSpec{
					Kubernetes: v1alpha1.Kubernetes{
						KubernetesVersion: "v1.30.0",
					},
					ContainerRuntime: v1alpha1.ContainerRuntime{
						Name: v1alpha1.ContainerRuntimeDocker,
					},
				},
			},
			wantErr: false,
			check: func(t *testing.T, c *KubeadmConfig) {
				assert.Equal(t, "unix:///run/cri-dockerd.sock", c.CriSocket)
			},
		},
		{
			name: "config with crio runtime",
			env: v1alpha1.Environment{
				Spec: v1alpha1.EnvironmentSpec{
					Kubernetes: v1alpha1.Kubernetes{
						KubernetesVersion: "v1.30.0",
					},
					ContainerRuntime: v1alpha1.ContainerRuntime{
						Name: v1alpha1.ContainerRuntimeCrio,
					},
				},
			},
			wantErr: false,
			check: func(t *testing.T, c *KubeadmConfig) {
				assert.Equal(t, "unix:///run/crio/crio.sock", c.CriSocket)
			},
		},
		{
			name: "config with empty version defaults",
			env: v1alpha1.Environment{
				Spec: v1alpha1.EnvironmentSpec{
					Kubernetes: v1alpha1.Kubernetes{
						KubernetesVersion: "",
					},
					ContainerRuntime: v1alpha1.ContainerRuntime{
						Name: v1alpha1.ContainerRuntimeContainerd,
					},
				},
			},
			wantErr: false,
			check: func(t *testing.T, c *KubeadmConfig) {
				assert.Equal(t, defaultKubernetesVersion, c.KubernetesVersion)
			},
		},
		{
			name: "config with invalid runtime",
			env: v1alpha1.Environment{
				Spec: v1alpha1.EnvironmentSpec{
					Kubernetes: v1alpha1.Kubernetes{
						KubernetesVersion: "v1.30.0",
					},
					ContainerRuntime: v1alpha1.ContainerRuntime{
						Name: "invalid-runtime",
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewKubeadmConfig(tt.env)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			if tt.check != nil {
				tt.check(t, got)
			}
		})
	}
}

func TestKubeadmConfig_ParseFeatureGates(t *testing.T) {
	tests := []struct {
		name         string
		featureGates string
		want         map[string]string
	}{
		{
			name:         "empty feature gates",
			featureGates: "",
			want:         map[string]string{},
		},
		{
			name:         "single feature gate",
			featureGates: "Feature1=true",
			want:         map[string]string{"Feature1": "true"},
		},
		{
			name:         "multiple feature gates",
			featureGates: "Feature1=true,Feature2=false,Feature3=true",
			want: map[string]string{
				"Feature1": "true",
				"Feature2": "false",
				"Feature3": "true",
			},
		},
		{
			name:         "malformed feature gate",
			featureGates: "NoEqualsHere",
			want:         map[string]string{},
		},
		{
			name:         "mixed valid and invalid",
			featureGates: "Valid=true,Invalid,Another=false",
			want: map[string]string{
				"Valid":   "true",
				"Another": "false",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &KubeadmConfig{FeatureGates: tt.featureGates}
			got := c.ParseFeatureGates()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestKubeadmConfig_GenerateKubeadmConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  *KubeadmConfig
		wantErr bool
		check   func(*testing.T, string)
	}{
		{
			name: "generate basic config",
			config: &KubeadmConfig{
				CriSocket:         "unix:///run/containerd/containerd.sock",
				ClusterName:       "test-cluster",
				KubernetesVersion: "v1.30.0",
				PodSubnet:         "192.168.0.0/16",
				FeatureGates:      "",
				IsUbuntu:          true,
				Template:          "apiVersion: test\nclusterName: {{.ClusterName}}\nversion: {{.KubernetesVersion}}",
			},
			wantErr: false,
			check: func(t *testing.T, output string) {
				assert.Contains(t, output, "clusterName: test-cluster")
				assert.Contains(t, output, "version: v1.30.0")
			},
		},
		{
			name: "generate config with feature gates",
			config: &KubeadmConfig{
				CriSocket:         "unix:///run/containerd/containerd.sock",
				ClusterName:       "test-cluster",
				KubernetesVersion: "v1.30.0",
				PodSubnet:         "192.168.0.0/16",
				FeatureGates:      "Feature1=true,Feature2=false",
				IsUbuntu:          true,
				Template:          "{{range $k, $v := .ParsedFeatureGates}}{{$k}}={{$v}}\n{{end}}",
			},
			wantErr: false,
			check: func(t *testing.T, output string) {
				assert.Contains(t, output, "Feature1=true")
				assert.Contains(t, output, "Feature2=false")
			},
		},
		{
			name: "invalid template",
			config: &KubeadmConfig{
				Template: "{{.Invalid",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := tt.config.GenerateKubeadmConfig()
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			if tt.check != nil {
				tt.check(t, output)
			}
		})
	}
}

func TestIsLegacyKubernetesVersion(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    bool
	}{
		{
			name:    "v1.30.0 is legacy",
			version: "v1.30.0",
			want:    true,
		},
		{
			name:    "v1.31.0 is legacy",
			version: "v1.31.0",
			want:    true,
		},
		{
			name:    "v1.32.0 is not legacy",
			version: "v1.32.0",
			want:    false,
		},
		{
			name:    "v1.33.0 is not legacy",
			version: "v1.33.0",
			want:    false,
		},
		{
			name:    "1.30.0 without v prefix is legacy",
			version: "1.30.0",
			want:    true,
		},
		{
			name:    "v0.99.0 is legacy",
			version: "v0.99.0",
			want:    true,
		},
		{
			name:    "v2.0.0 is not legacy",
			version: "v2.0.0",
			want:    false,
		},
		{
			name:    "invalid version returns false",
			version: "invalid",
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
