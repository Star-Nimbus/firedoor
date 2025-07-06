/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package alerting

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"

	accessv1alpha1 "github.com/cloud-nimbus/firedoor/api/v1alpha1"
	"github.com/cloud-nimbus/firedoor/internal/config"
)

func TestAlertmanagerService_Optional(t *testing.T) {
	// Test that Alertmanager service is optional when disabled
	disabledConfig := &config.AlertmanagerConfig{
		Enabled: false,
		URL:     "http://nonexistent:9093",
	}

	recorder := record.NewFakeRecorder(10)
	service := NewAlertmanagerService(disabledConfig, recorder)

	// Verify service is created but client is nil
	if service == nil {
		t.Fatal("Service should be created even when disabled")
	}
	if service.client != nil {
		t.Fatal("Client should be nil when Alertmanager is disabled")
	}

	// Test that sending alerts returns nil (no error) when disabled
	bg := &accessv1alpha1.Breakglass{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-breakglass",
			Namespace: "test-namespace",
		},
		Spec: accessv1alpha1.BreakglassSpec{
			Justification: "Test justification",
		},
	}

	ctx := context.Background()

	// Test active alert
	err := service.SendBreakglassActiveAlert(ctx, bg)
	if err != nil {
		t.Errorf("SendBreakglassActiveAlert should return nil when disabled, got: %v", err)
	}

	// Test expired alert
	err = service.SendBreakglassExpiredAlert(ctx, bg)
	if err != nil {
		t.Errorf("SendBreakglassExpiredAlert should return nil when disabled, got: %v", err)
	}
}

func TestAlertmanagerService_Enabled(t *testing.T) {
	// Test that Alertmanager service is properly configured when enabled
	enabledConfig := &config.AlertmanagerConfig{
		Enabled: true,
		URL:     "http://alertmanager:9093",
		Timeout: 30,
		Alert: config.AlertConfig{
			AlertName:   "TestAlert",
			Severity:    "warning",
			Summary:     "Test summary",
			Description: "Test description",
		},
	}

	recorder := record.NewFakeRecorder(10)
	service := NewAlertmanagerService(enabledConfig, recorder)

	// Verify service is created and client is not nil
	if service == nil {
		t.Fatal("Service should be created when enabled")
	}
	if service.client == nil {
		t.Fatal("Client should not be nil when Alertmanager is enabled")
	}

	// Verify configuration is set
	if service.config.Enabled != true {
		t.Error("Config should be enabled")
	}
	if service.config.URL != "http://alertmanager:9093" {
		t.Error("URL should be set correctly")
	}
}
