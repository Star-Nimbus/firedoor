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
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"k8s.io/client-go/tools/record"

	accessv1alpha1 "github.com/cloud-nimbus/firedoor/api/v1alpha1"
	"github.com/cloud-nimbus/firedoor/internal/config"
)

// AlertmanagerService handles sending alerts to Alertmanager
type AlertmanagerService struct {
	config   *config.AlertmanagerConfig
	client   *http.Client
	recorder record.EventRecorder
}

// Alert represents an Alertmanager alert
type Alert struct {
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	StartsAt    time.Time         `json:"startsAt"`
	EndsAt      time.Time         `json:"endsAt"`
}

// AlertGroup represents a group of alerts sent to Alertmanager
type AlertGroup struct {
	GroupLabels map[string]string `json:"groupLabels"`
	Alerts      []Alert           `json:"alerts"`
}

// NewAlertmanagerService creates a new Alertmanager service
func NewAlertmanagerService(cfg *config.AlertmanagerConfig, recorder record.EventRecorder) *AlertmanagerService {
	if !cfg.Enabled {
		return &AlertmanagerService{
			config:   cfg,
			client:   nil,
			recorder: recorder,
		}
	}

	// Create HTTP client with TLS configuration
	tlsConfig := &tls.Config{
		InsecureSkipVerify: cfg.TLS.InsecureSkipVerify,
	}

	client := &http.Client{
		Timeout: cfg.Timeout,
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
	}

	return &AlertmanagerService{
		config:   cfg,
		client:   client,
		recorder: recorder,
	}
}

// SendBreakglassActiveAlert sends an alert when breakglass access becomes active
func (a *AlertmanagerService) SendBreakglassActiveAlert(ctx context.Context, bg *accessv1alpha1.Breakglass) error {
	if !a.config.Enabled || a.client == nil {
		return nil
	}

	alert, err := a.createBreakglassAlert(bg, true)
	if err != nil {
		return fmt.Errorf("failed to create breakglass alert: %w", err)
	}

	return a.sendAlert(ctx, alert)
}

// SendBreakglassExpiredAlert sends an alert when breakglass access expires
func (a *AlertmanagerService) SendBreakglassExpiredAlert(ctx context.Context, bg *accessv1alpha1.Breakglass) error {
	if !a.config.Enabled || a.client == nil {
		return nil
	}

	alert, err := a.createBreakglassAlert(bg, false)
	if err != nil {
		return fmt.Errorf("failed to create breakglass alert: %w", err)
	}

	return a.sendAlert(ctx, alert)
}

// createBreakglassAlert creates an alert for a breakglass resource
func (a *AlertmanagerService) createBreakglassAlert(bg *accessv1alpha1.Breakglass, isActive bool) (*Alert, error) {
	now := time.Now()

	// Create labels
	labels := make(map[string]string)
	for k, v := range a.config.Alert.Labels {
		labels[k] = v
	}

	// Add breakglass-specific labels
	labels["alertname"] = a.config.Alert.AlertName
	labels["severity"] = a.config.Alert.Severity
	labels["breakglass_name"] = bg.Name
	labels["breakglass_namespace"] = bg.Namespace

	if isActive {
		labels["status"] = "active"
	} else {
		labels["status"] = "expired"
	}

	// Create annotations
	annotations := make(map[string]string)
	for k, v := range a.config.Alert.Annotations {
		annotations[k] = v
	}

	// Add breakglass-specific annotations
	annotations["summary"] = a.config.Alert.Summary
	annotations["description"] = a.config.Alert.Description
	annotations["justification"] = bg.Spec.Justification
	annotations["approved_by"] = bg.Status.ApprovedBy
	annotations["subjects"] = a.formatSubjects(bg.Spec.Subjects)

	if bg.Spec.TicketID != "" {
		annotations["ticket_id"] = bg.Spec.TicketID
	}

	if isActive {
		annotations["status"] = "Active"
		annotations["expires_at"] = bg.Status.ExpiresAt.Format(time.RFC3339)
	} else {
		annotations["status"] = "Expired"
		annotations["granted_at"] = bg.Status.GrantedAt.Format(time.RFC3339)
	}

	// Set alert timing
	var startsAt, endsAt time.Time
	if isActive {
		startsAt = now
		if bg.Status.ExpiresAt != nil {
			endsAt = bg.Status.ExpiresAt.Time
		} else {
			endsAt = now.Add(24 * time.Hour) // Default to 24 hours if no expiry
		}
	} else {
		startsAt = now.Add(-5 * time.Minute) // Alert was active 5 minutes ago
		endsAt = now
	}

	return &Alert{
		Labels:      labels,
		Annotations: annotations,
		StartsAt:    startsAt,
		EndsAt:      endsAt,
	}, nil
}

// sendAlert sends an alert to Alertmanager
func (a *AlertmanagerService) sendAlert(ctx context.Context, alert *Alert) error {
	alertGroup := AlertGroup{
		GroupLabels: map[string]string{
			"alertname": a.config.Alert.AlertName,
		},
		Alerts: []Alert{*alert},
	}

	payload, err := json.Marshal(alertGroup)
	if err != nil {
		return fmt.Errorf("failed to marshal alert: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", a.config.URL+"/api/v1/alerts", bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Add basic auth if configured
	if a.config.BasicAuth.Username != "" {
		req.SetBasicAuth(a.config.BasicAuth.Username, a.config.BasicAuth.Password)
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send alert: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("alertmanager returned status %d", resp.StatusCode)
	}

	return nil
}

// formatSubjects formats the subjects for annotation
func (a *AlertmanagerService) formatSubjects(subjects []accessv1alpha1.SubjectRef) string {
	if len(subjects) == 0 {
		return "none"
	}

	var result string
	for i, subject := range subjects {
		if i > 0 {
			result += ", "
		}
		result += fmt.Sprintf("%s:%s", subject.Kind, subject.Name)
	}
	return result
}
