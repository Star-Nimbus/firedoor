package e2e

import (
	"context"
	"fmt"
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
	timeout           = 2 * time.Minute
)

// randomName generates a DNS-1123 compliant name with timestamp for uniqueness
func randomName(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

var _ = Describe("Firedoor operator", Ordered, Serial, func() {
	var (
		ctx             context.Context
		clientset       *kubernetes.Clientset
		k8sClient       ctrlclient.Client
		discoveryClient discovery.DiscoveryInterface
	)

	BeforeAll(func() {
		ctx = context.Background()

		// Setup clients
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

		// Ensure namespace exists
		ensureNamespace(ctx, clientset, operatorNamespace)

		// Deploy operator if not already running
		By("checking if operator is deployed")
		pods, err := clientset.CoreV1().Pods(operatorNamespace).List(ctx,
			metav1.ListOptions{LabelSelector: "control-plane=controller-manager"})
		if err != nil || len(pods.Items) == 0 {
			By("deploying operator with skaffold")
			Expect(utils.SkaffoldRun("dev")).To(Succeed())
			DeferCleanup(func() { utils.CleanupSkaffoldDeployment("dev") })
		}

		// Wait for operator to be ready
		Eventually(func() error {
			pods, err := clientset.CoreV1().Pods(operatorNamespace).List(ctx,
				metav1.ListOptions{LabelSelector: "control-plane=controller-manager"})
			if err != nil || len(pods.Items) != 1 {
				return fmt.Errorf("expected one controller pod")
			}
			if pods.Items[0].Status.Phase != corev1.PodRunning {
				return fmt.Errorf("controller pod not running")
			}
			return nil
		}, timeout, pollInterval).Should(Succeed())

		// Wait for CRD to be established
		gvr := schema.GroupVersionResource{
			Group:    "access.cloudnimbus.io",
			Version:  "v1alpha1",
			Resource: "breakglasses",
		}
		Eventually(func() error {
			return utils.WaitForCRDWithDiscovery(ctx, discoveryClient, gvr, 30*time.Second)
		}, timeout, pollInterval).Should(Succeed())
	})

	It("should grant and expire a Breakglass session", func() {
		name := randomName("breakglass")
		bg := createBreakglass(name, "test-user", time.Minute)

		Expect(k8sClient.Create(ctx, bg)).To(Succeed())
		DeferCleanup(k8sClient.Delete, ctx, bg)

		By("waiting for Breakglass to become Active")
		Eventually(func() accessv1alpha1.BreakglassPhase {
			bg, _ := fetchBreakglass(ctx, k8sClient, name)
			return bg.Status.Phase
		}, timeout, pollInterval).Should(Equal(accessv1alpha1.PhaseActive))

		By("waiting for Breakglass to expire")
		Eventually(func() accessv1alpha1.BreakglassPhase {
			bg, _ := fetchBreakglass(ctx, k8sClient, name)
			return bg.Status.Phase
		}, timeout, pollInterval).Should(Equal(accessv1alpha1.PhaseExpired))

		// Verify expired condition
		bg, err := fetchBreakglass(ctx, k8sClient, name)
		Expect(err).NotTo(HaveOccurred())
		Expect(hasCondition(bg, conditions.Expired, metav1.ConditionTrue)).To(BeTrue())
	})

	It("should deny a Breakglass without subjects", func() {
		name := randomName("breakglass-invalid")
		bg := createBreakglass(name, "", time.Minute) // No subjects

		Expect(k8sClient.Create(ctx, bg)).To(Succeed())
		DeferCleanup(k8sClient.Delete, ctx, bg)

		Eventually(func() accessv1alpha1.BreakglassPhase {
			bg, _ := fetchBreakglass(ctx, k8sClient, name)
			return bg.Status.Phase
		}, timeout, pollInterval).Should(Equal(accessv1alpha1.PhaseDenied))

		bg, _ = fetchBreakglass(ctx, k8sClient, name)
		Expect(hasCondition(bg, conditions.Denied, metav1.ConditionTrue)).To(BeTrue())
	})
})

func ensureNamespace(ctx context.Context, cs *kubernetes.Clientset, ns string) {
	if _, err := cs.CoreV1().Namespaces().Get(ctx, ns, metav1.GetOptions{}); err == nil {
		return
	}
	_, _ = cs.CoreV1().Namespaces().
		Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}}, metav1.CreateOptions{})
}

func fetchBreakglass(ctx context.Context, c ctrlclient.Client, name string) (*accessv1alpha1.Breakglass, error) {
	var bg accessv1alpha1.Breakglass
	err := c.Get(ctx, ctrlclient.ObjectKey{Namespace: operatorNamespace, Name: name}, &bg)
	return &bg, err
}

func hasCondition(bg *accessv1alpha1.Breakglass, t conditions.Condition, status metav1.ConditionStatus) bool {
	for _, cond := range bg.Status.Conditions {
		if cond.Type == t.String() && cond.Status == status {
			return true
		}
	}
	return false
}

func createBreakglass(name, user string, duration time.Duration) *accessv1alpha1.Breakglass {
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
			Duration:         &metav1.Duration{Duration: duration},
			Justification:    "e2e test",
			AccessPolicy: &accessv1alpha1.AccessPolicy{
				Rules: []accessv1alpha1.AccessRule{
					{
						Actions:   []accessv1alpha1.Action{"get", "list"},
						Resources: []string{"pods"},
						APIGroups: []string{""},
					},
				},
			},
		},
	}

	if user != "" {
		bg.Spec.Subjects = []accessv1alpha1.SubjectRef{
			{Kind: "User", Name: user},
		}
	}

	return bg
}
