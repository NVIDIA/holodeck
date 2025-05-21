package provider

import (
	"errors"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MockProvider is a mock implementation of the Provider interface
type MockProvider struct {
	name            string
	createError     error
	deleteError     error
	dryRunError     error
	statusError     error
	updateTagsError error
	conditions      []metav1.Condition
}

func (m *MockProvider) Name() string {
	return m.name
}

func (m *MockProvider) Create() error {
	return m.createError
}

func (m *MockProvider) Delete() error {
	return m.deleteError
}

func (m *MockProvider) DryRun() error {
	return m.dryRunError
}

func (m *MockProvider) Status() ([]metav1.Condition, error) {
	return m.conditions, m.statusError
}

func (m *MockProvider) UpdateResourcesTags(tags map[string]string, resources ...string) error {
	return m.updateTagsError
}

func TestProviderInterface(t *testing.T) {
	tests := []struct {
		name           string
		provider       *MockProvider
		expectedName   string
		expectedError  error
		expectedStatus []metav1.Condition
	}{
		{
			name: "successful provider",
			provider: &MockProvider{
				name: "test-provider",
				conditions: []metav1.Condition{
					{
						Type:   "Ready",
						Status: "True",
					},
				},
			},
			expectedName:  "test-provider",
			expectedError: nil,
			expectedStatus: []metav1.Condition{
				{
					Type:   "Ready",
					Status: "True",
				},
			},
		},
		{
			name: "provider with create error",
			provider: &MockProvider{
				name:        "error-provider",
				createError: errors.New("create error"),
			},
			expectedName:  "error-provider",
			expectedError: errors.New("create error"),
		},
		{
			name: "provider with delete error",
			provider: &MockProvider{
				name:        "error-provider",
				deleteError: errors.New("delete error"),
			},
			expectedName:  "error-provider",
			expectedError: errors.New("delete error"),
		},
		{
			name: "provider with dry run error",
			provider: &MockProvider{
				name:        "error-provider",
				dryRunError: errors.New("dry run error"),
			},
			expectedName:  "error-provider",
			expectedError: errors.New("dry run error"),
		},
		{
			name: "provider with status error",
			provider: &MockProvider{
				name:        "error-provider",
				statusError: errors.New("status error"),
			},
			expectedName:  "error-provider",
			expectedError: errors.New("status error"),
		},
		{
			name: "provider with update tags error",
			provider: &MockProvider{
				name:            "error-provider",
				updateTagsError: errors.New("update tags error"),
			},
			expectedName:  "error-provider",
			expectedError: errors.New("update tags error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test Name()
			if got := tt.provider.Name(); got != tt.expectedName {
				t.Errorf("Name() = %v, want %v", got, tt.expectedName)
			}

			// Test Create()
			if err := tt.provider.Create(); err != nil && err.Error() != tt.provider.createError.Error() {
				t.Errorf("Create() error = %v, want %v", err, tt.provider.createError)
			}

			// Test Delete()
			if err := tt.provider.Delete(); err != nil && err.Error() != tt.provider.deleteError.Error() {
				t.Errorf("Delete() error = %v, want %v", err, tt.provider.deleteError)
			}

			// Test DryRun()
			if err := tt.provider.DryRun(); err != nil && err.Error() != tt.provider.dryRunError.Error() {
				t.Errorf("DryRun() error = %v, want %v", err, tt.provider.dryRunError)
			}

			// Test Status()
			conditions, err := tt.provider.Status()
			if err != nil && err.Error() != tt.provider.statusError.Error() {
				t.Errorf("Status() error = %v, want %v", err, tt.provider.statusError)
			}
			if err == nil && len(conditions) != len(tt.expectedStatus) {
				t.Errorf("Status() conditions length = %v, want %v", len(conditions), len(tt.expectedStatus))
			}

			// Test UpdateResourcesTags()
			if err := tt.provider.UpdateResourcesTags(map[string]string{"key": "value"}); err != nil && err.Error() != tt.provider.updateTagsError.Error() {
				t.Errorf("UpdateResourcesTags() error = %v, want %v", err, tt.provider.updateTagsError)
			}
		})
	}
}
