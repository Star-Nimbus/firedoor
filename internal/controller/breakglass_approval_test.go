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

package controller

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"

	accessv1alpha1 "github.com/cloud-nimbus/firedoor/api/v1alpha1"
	"github.com/cloud-nimbus/firedoor/internal/alerting"
	"github.com/cloud-nimbus/firedoor/internal/config"
	rbacv1 "k8s.io/api/rbac/v1"
)

var _ = Describe("Breakglass Approval Logic", func() {
	var (
		ctx        context.Context
		reconciler *BreakglassReconciler
		mockClient *MockClient
		recorder   *record.FakeRecorder
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockClient = &MockClient{}
		recorder = record.NewFakeRecorder(10)
		reconciler = &BreakglassReconciler{
			Client:   mockClient,
			Recorder: recorder,
		}
		// Create a disabled Alertmanager service for testing
		alertService := alerting.NewAlertmanagerService(&config.AlertmanagerConfig{Enabled: false}, recorder)
		reconciler.operator = NewBreakglassOperator(mockClient, recorder, alertService)
	})

	Describe("Approval Required Logic", func() {
		It("should require approval when ApprovalRequired is true", func() {
			bg := &accessv1alpha1.Breakglass{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-approval-required",
					Namespace: "default",
				},
				Spec: accessv1alpha1.BreakglassSpec{
					Subjects: []accessv1alpha1.SubjectRef{{
						Kind: rbacv1.UserKind,
						Name: "test-user",
					}},
					ClusterRoles:     []string{"admin"},
					ApprovalRequired: true,
					Duration:         &metav1.Duration{Duration: time.Hour},
					Justification:    "Test approval required",
				},
				Status: accessv1alpha1.BreakglassStatus{
					Phase: accessv1alpha1.PhasePending,
				},
			}

			// Should stay in pending phase when approval is required but not approved
			result, err := reconciler.handlePendingBreakglass(ctx, bg)
			Expect(err).NotTo(HaveOccurred())
			// Account for jitter: base 30s + 0-30s jitter = 30-60s range
			Expect(result.RequeueAfter).To(BeNumerically(">=", 30*time.Second))
			Expect(result.RequeueAfter).To(BeNumerically("<=", 60*time.Second))
			Expect(bg.Status.Phase).To(Equal(accessv1alpha1.PhasePending))
		})

		It("should auto-approve when ApprovalRequired is false", func() {
			bg := &accessv1alpha1.Breakglass{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-auto-approve",
					Namespace: "default",
				},
				Spec: accessv1alpha1.BreakglassSpec{
					Subjects: []accessv1alpha1.SubjectRef{{
						Kind: rbacv1.UserKind,
						Name: "test-user",
					}},
					ClusterRoles:     []string{"admin"},
					ApprovalRequired: false,
					Duration:         &metav1.Duration{Duration: time.Hour},
					Justification:    "Test auto approve",
				},
				Status: accessv1alpha1.BreakglassStatus{
					Phase: accessv1alpha1.PhasePending,
				},
			}

			// Should proceed to grant access when approval not required
			result, err := reconciler.handlePendingBreakglass(ctx, bg)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(BeNumerically(">", 0))
		})

		It("should proceed when approval is required and already approved", func() {
			bg := &accessv1alpha1.Breakglass{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-already-approved",
					Namespace: "default",
				},
				Spec: accessv1alpha1.BreakglassSpec{
					Subjects: []accessv1alpha1.SubjectRef{{
						Kind: rbacv1.UserKind,
						Name: "test-user",
					}},
					ClusterRoles:     []string{"admin"},
					ApprovalRequired: true,
					Duration:         &metav1.Duration{Duration: time.Hour},
					Justification:    "Test already approved",
				},
				Status: accessv1alpha1.BreakglassStatus{
					Phase:      accessv1alpha1.PhasePending,
					ApprovedBy: "admin-user",
					ApprovedAt: &metav1.Time{Time: time.Now()},
				},
			}

			// Should proceed to grant access when already approved
			result, err := reconciler.handlePendingBreakglass(ctx, bg)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(BeNumerically(">", 0))
		})
	})

	Describe("Manual Approval", func() {
		It("should approve a pending breakglass", func() {
			bg := &accessv1alpha1.Breakglass{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-manual-approve",
					Namespace: "default",
				},
				Spec: accessv1alpha1.BreakglassSpec{
					Subjects: []accessv1alpha1.SubjectRef{{
						Kind: rbacv1.UserKind,
						Name: "test-user",
					}},
					ClusterRoles:     []string{"admin"},
					ApprovalRequired: true,
					Duration:         &metav1.Duration{Duration: time.Hour},
					Justification:    "Test manual approval",
				},
				Status: accessv1alpha1.BreakglassStatus{
					Phase: accessv1alpha1.PhasePending,
				},
			}

			err := reconciler.ApproveBreakglass(ctx, bg, "admin-user")
			Expect(err).NotTo(HaveOccurred())
			Expect(bg.Status.ApprovedBy).To(Equal("admin-user"))
			Expect(bg.Status.ApprovedAt).NotTo(BeNil())

			// Check approval condition
			var approvalCondition *metav1.Condition
			for i := range bg.Status.Conditions {
				if bg.Status.Conditions[i].Type == "Approved" {
					approvalCondition = &bg.Status.Conditions[i]
					break
				}
			}
			Expect(approvalCondition).NotTo(BeNil())
			Expect(approvalCondition.Status).To(Equal(metav1.ConditionTrue))
			Expect(approvalCondition.Reason).To(Equal("AccessGranted"))
		})

		It("should not approve an already approved breakglass", func() {
			bg := &accessv1alpha1.Breakglass{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-already-approved-error",
					Namespace: "default",
				},
				Spec: accessv1alpha1.BreakglassSpec{
					Subjects: []accessv1alpha1.SubjectRef{{
						Kind: rbacv1.UserKind,
						Name: "test-user",
					}},
					ClusterRoles:     []string{"admin"},
					ApprovalRequired: true,
					Duration:         &metav1.Duration{Duration: time.Hour},
					Justification:    "Test already approved error",
				},
				Status: accessv1alpha1.BreakglassStatus{
					Phase:      accessv1alpha1.PhasePending,
					ApprovedBy: "first-admin",
					ApprovedAt: &metav1.Time{Time: time.Now()},
				},
			}

			err := reconciler.ApproveBreakglass(ctx, bg, "second-admin")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("already approved"))
		})

		It("should not approve a non-pending breakglass", func() {
			bg := &accessv1alpha1.Breakglass{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-wrong-phase-error",
					Namespace: "default",
				},
				Spec: accessv1alpha1.BreakglassSpec{
					Subjects: []accessv1alpha1.SubjectRef{{
						Kind: rbacv1.UserKind,
						Name: "test-user",
					}},
					ClusterRoles:     []string{"admin"},
					ApprovalRequired: true,
					Duration:         &metav1.Duration{Duration: time.Hour},
					Justification:    "Test wrong phase error",
				},
				Status: accessv1alpha1.BreakglassStatus{
					Phase: accessv1alpha1.PhaseActive,
				},
			}

			err := reconciler.ApproveBreakglass(ctx, bg, "admin-user")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("cannot approve breakglass in phase"))
		})
	})

	Describe("Manual Denial", func() {
		It("should deny a pending breakglass", func() {
			bg := &accessv1alpha1.Breakglass{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-manual-deny",
					Namespace: "default",
				},
				Spec: accessv1alpha1.BreakglassSpec{
					Subjects: []accessv1alpha1.SubjectRef{{
						Kind: rbacv1.UserKind,
						Name: "test-user",
					}},
					ClusterRoles:     []string{"admin"},
					ApprovalRequired: true,
					Duration:         &metav1.Duration{Duration: time.Hour},
					Justification:    "Test manual denial",
				},
				Status: accessv1alpha1.BreakglassStatus{
					Phase: accessv1alpha1.PhasePending,
				},
			}

			err := reconciler.DenyBreakglass(ctx, bg, "admin-user", "Insufficient justification")
			Expect(err).NotTo(HaveOccurred())
			Expect(bg.Status.Phase).To(Equal(accessv1alpha1.PhaseDenied))

			// Check denial condition
			var denialCondition *metav1.Condition
			for i := range bg.Status.Conditions {
				if bg.Status.Conditions[i].Type == "Denied" {
					denialCondition = &bg.Status.Conditions[i]
					break
				}
			}
			Expect(denialCondition).NotTo(BeNil())
			Expect(denialCondition.Status).To(Equal(metav1.ConditionTrue))
			Expect(denialCondition.Reason).To(Equal("AccessDenied"))
			Expect(denialCondition.Message).To(ContainSubstring("Insufficient justification"))
		})

		It("should not deny an already approved breakglass", func() {
			bg := &accessv1alpha1.Breakglass{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deny-approved-error",
					Namespace: "default",
				},
				Spec: accessv1alpha1.BreakglassSpec{
					Subjects: []accessv1alpha1.SubjectRef{{
						Kind: rbacv1.UserKind,
						Name: "test-user",
					}},
					ClusterRoles:     []string{"admin"},
					ApprovalRequired: true,
					Duration:         &metav1.Duration{Duration: time.Hour},
					Justification:    "Test deny approved error",
				},
				Status: accessv1alpha1.BreakglassStatus{
					Phase:      accessv1alpha1.PhasePending,
					ApprovedBy: "admin-user",
					ApprovedAt: &metav1.Time{Time: time.Now()},
				},
			}

			err := reconciler.DenyBreakglass(ctx, bg, "admin-user", "Too late")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("already approved"))
		})
	})

	Describe("Manual Revocation", func() {
		It("should revoke an active breakglass", func() {
			bg := &accessv1alpha1.Breakglass{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-manual-revoke",
					Namespace: "default",
				},
				Spec: accessv1alpha1.BreakglassSpec{
					Subjects: []accessv1alpha1.SubjectRef{{
						Kind: rbacv1.UserKind,
						Name: "test-user",
					}},
					ClusterRoles:     []string{"admin"},
					ApprovalRequired: false,
					Duration:         &metav1.Duration{Duration: time.Hour},
					Justification:    "Test manual revocation",
				},
				Status: accessv1alpha1.BreakglassStatus{
					Phase: accessv1alpha1.PhaseActive,
				},
			}

			err := reconciler.RevokeBreakglass(ctx, bg, "admin-user", "Security incident")
			Expect(err).NotTo(HaveOccurred())
			Expect(bg.Status.Phase).To(Equal(accessv1alpha1.PhaseRevoked))

			// Check revocation condition
			var revocationCondition *metav1.Condition
			for i := range bg.Status.Conditions {
				if bg.Status.Conditions[i].Type == "Revoked" {
					revocationCondition = &bg.Status.Conditions[i]
					break
				}
			}
			Expect(revocationCondition).NotTo(BeNil())
			Expect(revocationCondition.Status).To(Equal(metav1.ConditionTrue))
			Expect(revocationCondition.Reason).To(Equal("AccessRevoked"))
			Expect(revocationCondition.Message).To(ContainSubstring("Security incident"))
		})

		It("should not revoke a non-active breakglass", func() {
			bg := &accessv1alpha1.Breakglass{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-revoke-pending-error",
					Namespace: "default",
				},
				Spec: accessv1alpha1.BreakglassSpec{
					Subjects: []accessv1alpha1.SubjectRef{{
						Kind: rbacv1.UserKind,
						Name: "test-user",
					}},
					ClusterRoles:     []string{"admin"},
					ApprovalRequired: true,
					Duration:         &metav1.Duration{Duration: time.Hour},
					Justification:    "Test revoke pending error",
				},
				Status: accessv1alpha1.BreakglassStatus{
					Phase: accessv1alpha1.PhasePending,
				},
			}

			err := reconciler.RevokeBreakglass(ctx, bg, "admin-user", "Too early")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("cannot revoke breakglass in phase"))
		})
	})

	Describe("New Breakglass Handling", func() {
		It("should set pending phase for new breakglass with approval required", func() {
			bg := &accessv1alpha1.Breakglass{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-new-pending",
					Namespace: "default",
				},
				Spec: accessv1alpha1.BreakglassSpec{
					Subjects: []accessv1alpha1.SubjectRef{{
						Kind: rbacv1.UserKind,
						Name: "test-user",
					}},
					ClusterRoles:     []string{"admin"},
					ApprovalRequired: true,
					Duration:         &metav1.Duration{Duration: time.Hour},
					Justification:    "Test new pending",
				},
			}

			result, err := reconciler.handleNewBreakglass(ctx, bg)
			Expect(err).NotTo(HaveOccurred())
			Expect(bg.Status.Phase).To(Equal(accessv1alpha1.PhasePending))
			Expect(result.RequeueAfter).To(Equal(time.Duration(0)))
		})

		It("should auto-approve new breakglass when approval not required", func() {
			bg := &accessv1alpha1.Breakglass{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-new-auto-approve",
					Namespace: "default",
				},
				Spec: accessv1alpha1.BreakglassSpec{
					Subjects: []accessv1alpha1.SubjectRef{{
						Kind: rbacv1.UserKind,
						Name: "test-user",
					}},
					ClusterRoles:     []string{"admin"},
					ApprovalRequired: false,
					Duration:         &metav1.Duration{Duration: time.Hour},
					Justification:    "Test new auto approve",
				},
			}

			result, err := reconciler.handleNewBreakglass(ctx, bg)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(BeNumerically(">", 0))
		})
	})
})
