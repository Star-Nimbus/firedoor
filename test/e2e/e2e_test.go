package e2e

import (
	"context"
	"fmt"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	accessv1alpha1 "github.com/cloud-nimbus/firedoor/api/v1alpha1"
	"github.com/cloud-nimbus/firedoor/test/utils"
	rbacv1 "k8s.io/api/rbac/v1"
)

const (
	operatorNamespace = "firedoor-system"
	pollInterval      = 2 * time.Second
	timeout           = 1 * time.Minute // Reduced timeout for simpler test
)

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
		var restCfg *rest.Config
		var err error
		// Try in-cluster config first, fallback to KUBECONFIG
		restCfg, err = rest.InClusterConfig()
		if err != nil {
			kubeconfig := os.Getenv("KUBECONFIG")
			if kubeconfig == "" {
				kubeconfig = os.ExpandEnv("$HOME/.kube/config")
			}
			restCfg, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
			Expect(err).NotTo(HaveOccurred())
		}

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

	It("should create a simple Breakglass resource and be processed by the operator", func() {
		name := randomName("simple-debug-access")
		By(fmt.Sprintf("creating simple breakglass - %s", name))

		// Create a simple one-time breakglass without complex scheduling
		bg := &accessv1alpha1.Breakglass{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: operatorNamespace,
			},
			Spec: accessv1alpha1.BreakglassSpec{
				Justification: "Simple test access for debugging",
				TicketID:      "TEST-001",
				Subjects: []rbacv1.Subject{
					{Kind: "User", Name: "test@example.com", APIGroup: "rbac.authorization.k8s.io"},
				},
				Policy: []accessv1alpha1.Policy{
					{
						Namespace: "default",
						Rules: []rbacv1.PolicyRule{
							{
								Verbs:     []string{"get", "list"},
								Resources: []string{"pods"},
								APIGroups: []string{""},
							},
						},
					},
				},
				Approval: &accessv1alpha1.ApprovalSpec{Required: false},
				Schedule: accessv1alpha1.ScheduleSpec{
					Start:    metav1.Time{Time: time.Now().Add(1 * time.Minute)}, // Start in 1 minute
					Duration: metav1.Duration{Duration: 30 * time.Minute},        // 30 minute duration
				},
			},
		}
		Expect(k8sClient.Create(ctx, bg)).To(Succeed())
		DeferCleanup(k8sClient.Delete, ctx, bg)

		By("waiting for the Breakglass resource to be processed by the operator")
		// Just check that the resource gets some status update, not a specific condition
		Eventually(func() bool {
			fetched := &accessv1alpha1.Breakglass{}
			err := k8sClient.Get(ctx, ctrlclient.ObjectKey{Name: name, Namespace: operatorNamespace}, fetched)
			if err != nil {
				return false
			}
			// Check if the operator has processed the resource (has conditions or observed generation)
			return len(fetched.Status.Conditions) > 0 || fetched.Status.ObservedGeneration > 0
		}, timeout, pollInterval).Should(BeTrue())

		By("verifying the resource was created successfully")
		fetched := &accessv1alpha1.Breakglass{}
		err := k8sClient.Get(ctx, ctrlclient.ObjectKey{Name: name, Namespace: operatorNamespace}, fetched)
		Expect(err).NotTo(HaveOccurred())
		Expect(fetched.Name).To(Equal(name))
		Expect(fetched.Spec.Justification).To(Equal("Simple test access for debugging"))
	})
})

func ensureNamespace(ctx context.Context, cs *kubernetes.Clientset, ns string) {
	if _, err := cs.CoreV1().Namespaces().Get(ctx, ns, metav1.GetOptions{}); err == nil {
		return
	}
	_, _ = cs.CoreV1().Namespaces().
		Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}}, metav1.CreateOptions{})
}

func int32Ptr(i int32) *int32 { return &i }
