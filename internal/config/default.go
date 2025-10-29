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

package config

import "time"

// Defaults holds all default configuration values
type Defaults struct {
	OTel         OTelDefaults
	Manager      ManagerDefaults
	Metrics      MetricsDefaults
	Health       HealthDefaults
	HTTP         HTTPDefaults
	Controller   ControllerDefaults
	Server       ServerDefaults
	Alertmanager AlertmanagerDefaults
}

// OTelDefaults holds OpenTelemetry default values
type OTelDefaults struct {
	Enabled  bool
	Exporter string
	Endpoint string
	Service  string
	LogLevel string
	TLS      TLSDefaults
}

// TLSDefaults holds TLS default values
type TLSDefaults struct {
	InsecureSkipVerify bool
	CAFile             string
	CertFile           string
	KeyFile            string
}

// ManagerDefaults holds manager default values
type ManagerDefaults struct {
	LeaderElect bool
}

// MetricsDefaults holds metrics default values
type MetricsDefaults struct {
	BindAddress         string
	Secure              bool
	DurationBucketStart float64
	DurationBucketWidth float64
	DurationBucketCount int
}

// HealthDefaults holds health check default values
type HealthDefaults struct {
	ProbeBindAddress string
}

// HTTPDefaults holds HTTP server default values
type HTTPDefaults struct {
	EnableHTTP2 bool
}

// ControllerDefaults holds controller default values
type ControllerDefaults struct {
	ReconcileTimeout    time.Duration
	RetryDelay          time.Duration
	PrivilegeEscalation bool
	Backoff             time.Duration
}

// ServerDefaults holds server default values
type ServerDefaults struct {
	MetricsBindAddress     string
	HealthProbeBindAddress string
	LeaderElect            bool
}

// AlertmanagerDefaults holds Alertmanager default values
type AlertmanagerDefaults struct {
	Enabled  bool
	Endpoint string
}

// NewDefaults returns the default configuration values
func NewDefaults() *Defaults {
	return &Defaults{
		OTel: OTelDefaults{
			Enabled:  false, // Disabled by default for simpler development
			Exporter: "otlp",
			Endpoint: "otel-collector-opentelemetry-collector.telemetry-system.svc.cluster.local:4317",
			Service:  "firedoor-operator",
			LogLevel: "info",
			TLS: TLSDefaults{
				InsecureSkipVerify: true, // Insecure by default for easier development
				CAFile:             "",
				CertFile:           "",
				KeyFile:            "",
			},
		},
		Manager: ManagerDefaults{
			LeaderElect: false,
		},
		Metrics: MetricsDefaults{
			BindAddress:         ":8080",
			Secure:              false,
			DurationBucketStart: 5.0,
			DurationBucketWidth: 15.0,
			DurationBucketCount: 8,
		},
		Health: HealthDefaults{
			ProbeBindAddress: ":8081",
		},
		HTTP: HTTPDefaults{
			EnableHTTP2: false,
		},
		Controller: ControllerDefaults{
			ReconcileTimeout:    30 * time.Second,
			RetryDelay:          1 * time.Second,
			PrivilegeEscalation: false,
			Backoff:             10 * time.Second,
		},
		Server: ServerDefaults{
			MetricsBindAddress:     ":8080",
			HealthProbeBindAddress: ":8081",
			LeaderElect:            false,
		},
		Alertmanager: AlertmanagerDefaults{
			Enabled:  false,
			Endpoint: "http://alertmanager.telemetry-system.svc.cluster.local:9093",
		},
	}
}
