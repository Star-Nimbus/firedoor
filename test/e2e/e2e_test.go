/*
   End-to-end flow tests for the Firedoor operator.  The suite is **black-box**:
   we only interact with the cluster API surface – never internal packages.

   Flow:
     1. Spin-up the operator with `skaffold run --profile=dev` **once** per suite.
     2. Exercise a couple of happy-path and validation scenarios.
     3. Tear the deployment down with `skaffold delete`.

   Run locally with:
     make test-e2e         # invokes `go test ./test/e2e -v -ginkgo.v`
*/

package e2e

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	accessv1alpha1 "github.com/cloud-nimbus/firedoor/api/v1alpha1"
	"github.com/cloud-nimbus/firedoor/internal/conditions"
	"github.com/cloud-nimbus/firedoor/test/utils"
)

const (
	operatorNamespace = "firedoor-system"
	pollInterval      = 2 * time.Second
	deployTimeout     = 3 * time.Minute  // give image pull & webhooks some slack
	shortTimeout      = 90 * time.Second // validation / activation path
	longTimeout       = 4 * time.Minute  // expiry path
)

// randomName generates a simple DNS-1123 compliant name.
func randomName(prefix string) string {
	return fmt.Sprintf("%s-%06x", prefix, rand.Int31())
}

var _ = Describe("Firedoor operator", Ordered, Serial, func() {
	var (
		ctx             context.Context
		clientset       *kubernetes.Clientset
		k8sClient       ctrlclient.Client
		discoveryClient discovery.DiscoveryInterface
	)

	// -------------------------------------------------------------------------
	// Cluster + client bootstrap (once per suite)
	// -------------------------------------------------------------------------
	BeforeAll(func() {
		ctx = context.Background()

		restCfg, err := config.GetConfig()
		Expect(err).NotTo(HaveOccurred())

		clientset, err = kubernetes.NewForConfig(restCfg)
		Expect(err).NotTo(HaveOccurred())

		discoveryClient, err = discovery.NewDiscoveryClientForConfig(restCfg)
		Expect(err).NotTo(HaveOccurred())

		scheme := runtime.NewScheme()
		Expect(corev1.AddToScheme(scheme)).To(Succeed())
		Expect(accessv1alpha1.AddToScheme(scheme)).To(Succeed())

		k8sClient, err = ctrlclient.New(restCfg, ctrlclient.Options{Scheme: scheme})
		Expect(err).NotTo(HaveOccurred())

		ensureNamespace(ctx, clientset, operatorNamespace)
	})

	// -------------------------------------------------------------------------
	// Deploy operator once – all specs run against the same instance
	// -------------------------------------------------------------------------
	BeforeAll(func() {
		By("checking if operator is already deployed")
		pods, err := clientset.CoreV1().Pods(operatorNamespace).List(ctx,
			metav1.ListOptions{LabelSelector: "control-plane=controller-manager"})
		if err != nil || len(pods.Items) == 0 {
			By("deploying operator with skaffold …")
			Expect(utils.SkaffoldRun("dev")).To(Succeed())
			DeferCleanup(func() { utils.CleanupSkaffoldDeployment("dev") })
		} else {
			By("operator already deployed, skipping deployment")
		}

		Eventually(controllerPodReady(ctx, clientset), deployTimeout, pollInterval).Should(Succeed())

		// Wait for the Breakglass CRD to be established.
		gvr := schema.GroupVersionResource{
			Group:    "access.cloudnimbus.io",
			Version:  "v1alpha1",
			Resource: "breakglasses",
		}
		Eventually(func() error {
			return utils.WaitForCRDWithDiscovery(ctx, discoveryClient, gvr, 30*time.Second)
		}, deployTimeout, pollInterval).Should(Succeed())
	})

	// -------------------------------------------------------------------------
	// Scenario: happy-path activation then automatic expiry
	// -------------------------------------------------------------------------
	It("grants and then expires a Breakglass session", func() {
		name := randomName("bg-happy")
		bg := newBreakglass(name, withUser("e2e-user"), withDuration(time.Minute))
		Expect(k8sClient.Create(ctx, bg)).To(Succeed())
		DeferCleanup(k8sClient.Delete, ctx, bg)

		By("waiting for Breakglass to become Active")
		Eventually(bgPhase(ctx, k8sClient, name), shortTimeout, pollInterval).
			Should(Equal(accessv1alpha1.PhaseActive))

		By("waiting for Breakglass to expire")
		Eventually(bgPhase(ctx, k8sClient, name), longTimeout, pollInterval).
			Should(Equal(accessv1alpha1.PhaseExpired))

		fetched := mustFetch(ctx, k8sClient, name)
		Expect(hasCondition(fetched, conditions.Expired, metav1.ConditionTrue)).To(BeTrue())
	})

	// -------------------------------------------------------------------------
	// Scenario: validation failure – no subjects provided
	// -------------------------------------------------------------------------
	It("denies a Breakglass without subjects", func() {
		name := randomName("bg-invalid")
		bg := newBreakglass(name) // no user / group
		Expect(k8sClient.Create(ctx, bg)).To(Succeed())
		DeferCleanup(k8sClient.Delete, ctx, bg)

		Eventually(bgPhase(ctx, k8sClient, name), shortTimeout, pollInterval).
			Should(Equal(accessv1alpha1.PhaseDenied))
		fetched := mustFetch(ctx, k8sClient, name)
		Expect(hasCondition(fetched, conditions.Denied, metav1.ConditionTrue)).To(BeTrue())
	})

	// -------------------------------------------------------------------------
	// Scenario: operator logs are accessible (debug aid)
	// -------------------------------------------------------------------------
	It("should have operator logs available", func() {
		By("fetching operator pod logs (last 20 lines)")
		pods, err := clientset.CoreV1().Pods(operatorNamespace).
			List(ctx, metav1.ListOptions{LabelSelector: "control-plane=controller-manager"})
		Expect(err).NotTo(HaveOccurred())
		Expect(len(pods.Items)).To(BeNumerically(">", 0))

		tail := int64(20)
		logs, err := clientset.CoreV1().Pods(operatorNamespace).
			GetLogs(pods.Items[0].Name, &corev1.PodLogOptions{TailLines: &tail}).Do(ctx).Raw()
		if err == nil {
			fmt.Fprintf(GinkgoWriter, "Operator logs (last 20 lines):\n%s\n", string(logs))
		}
	})

	// -------------------------------------------------------------------------
	// Scenario: RBAC artefacts wired correctly
	// -------------------------------------------------------------------------
	It("should have proper RBAC permissions", func() {
		By("checking RBAC objects")
		_, err := clientset.CoreV1().ServiceAccounts(operatorNamespace).
			Get(ctx, "controller-manager", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())

		_, err = clientset.RbacV1().ClusterRoleBindings().
			Get(ctx, "manager-rolebinding", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())

		_, err = clientset.RbacV1().ClusterRoles().
			Get(ctx, "manager-role", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
	})
})

/* -------------------------------------------------------------------------- */
/* Helpers                                                                    */
/* -------------------------------------------------------------------------- */

func ensureNamespace(ctx context.Context, cs *kubernetes.Clientset, ns string) {
	if _, err := cs.CoreV1().Namespaces().Get(ctx, ns, metav1.GetOptions{}); err == nil {
		return
	}
	_, _ = cs.CoreV1().Namespaces().
		Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}}, metav1.CreateOptions{})
}

func controllerPodReady(ctx context.Context, cs *kubernetes.Clientset) func() error {
	return func() error {
		pods, err := cs.CoreV1().
			Pods(operatorNamespace).
			List(ctx, metav1.ListOptions{LabelSelector: "control-plane=controller-manager"})
		if err != nil {
			return err
		}
		if len(pods.Items) != 1 {
			return fmt.Errorf("expected one controller pod, got %d", len(pods.Items))
		}
		if phase := pods.Items[0].Status.Phase; phase != corev1.PodRunning {
			return fmt.Errorf("controller pod not running (phase %s)", phase)
		}
		return nil
	}
}

/* ----------------------- Breakglass-specific helpers ---------------------- */

func mustFetch(ctx context.Context, c ctrlclient.Client, name string) *accessv1alpha1.Breakglass {
	bg, err := fetchBG(ctx, c, name)
	Expect(err).NotTo(HaveOccurred())
	return bg
}

func fetchBG(ctx context.Context, c ctrlclient.Client, name string) (*accessv1alpha1.Breakglass, error) {
	var bg accessv1alpha1.Breakglass
	if err := c.Get(ctx,
		ctrlclient.ObjectKey{Namespace: operatorNamespace, Name: name}, &bg); err != nil {
		return nil, err
	}
	return &bg, nil
}

func bgPhase(ctx context.Context, c ctrlclient.Client, name string) func() (accessv1alpha1.BreakglassPhase, error) {
	return func() (accessv1alpha1.BreakglassPhase, error) {
		bg, err := fetchBG(ctx, c, name)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(GinkgoWriter, "Breakglass %s: phase=%s  conditions=%+v\n",
			name, bg.Status.Phase, bg.Status.Conditions)
		return bg.Status.Phase, nil
	}
}

func hasCondition(bg *accessv1alpha1.Breakglass, t conditions.Condition, status metav1.ConditionStatus) bool {
	want := t.String()
	for _, cond := range bg.Status.Conditions {
		if cond.Type == want && cond.Status == status {
			return true
		}
	}
	return false
}

/* ----------------------- Breakglass object builder ----------------------- */

type bgOpt func(*accessv1alpha1.Breakglass)

func newBreakglass(name string, opts ...bgOpt) *accessv1alpha1.Breakglass {
	bg := &accessv1alpha1.Breakglass{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "access.cloudnimbus.io/v1alpha1",
			Kind:       "Breakglass",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: operatorNamespace,
		},
		Spec: accessv1alpha1.BreakglassSpec{
			ApprovalRequired: false,
			AccessPolicy:     &accessv1alpha1.AccessPolicy{Rules: []accessv1alpha1.AccessRule{defaultRule()}},
			Duration:         &metav1.Duration{Duration: time.Minute},
			Justification:    "e2e test",
		},
	}

	for _, o := range opts {
		o(bg)
	}
	return bg
}

func defaultRule() accessv1alpha1.AccessRule {
	return accessv1alpha1.AccessRule{
		Actions:   []accessv1alpha1.Action{"get", "list"},
		Resources: []string{"pods"},
	}
}

func withUser(name string) bgOpt {
	return func(bg *accessv1alpha1.Breakglass) {
		bg.Spec.Subjects = []accessv1alpha1.SubjectRef{{Kind: "User", Name: name}}
	}
}

func withDuration(d time.Duration) bgOpt {
	return func(bg *accessv1alpha1.Breakglass) {
		bg.Spec.Duration = &metav1.Duration{Duration: d}
	}
}
