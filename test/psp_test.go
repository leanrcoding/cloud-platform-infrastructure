package integration_tests

import (
	"context"
	"fmt"
	"html/template"
	"strings"

	rbac "k8s.io/api/rbac/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gruntwork-io/terratest/modules/k8s"
	"github.com/gruntwork-io/terratest/modules/random"
	"github.com/ministryofjustice/cloud-platform-infrastructure/test/helpers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("pod security policies", func() {
	FIt("should have the expected policies defined", func() {
		// Get a list of policies in the cluster
		policies, err := c.Client.Clientset.PolicyV1beta1().PodSecurityPolicies().List(context.Background(), v1.ListOptions{})
		Expect(err).NotTo(HaveOccurred())

		// Check that the expected policies are defined
		for _, policy := range policies.Items {
			switch policy.Name {
			case "privileged":
				Expect(policy.Spec.Privileged).To(BeTrue())
			case "restricted":
				Expect(policy.Spec.Privileged).To(BeFalse())
			}
		}
	})

	Context("when a container requires privileges", func() {
		var (
			namespace string
			options   *k8s.KubectlOptions
		)
		BeforeEach(func() {
			namespace = fmt.Sprintf("%s-psp-%s", c.Prefix, strings.ToLower(random.UniqueId()))
			options = k8s.NewKubectlOptions("", "", namespace)

			tpl, err := helpers.TemplateFile("./fixtures/namespace.yaml.tmpl", "namespace.yaml.tmpl", template.FuncMap{
				"namespace": namespace,
			})
			Expect(err).NotTo(HaveOccurred())

			err = k8s.KubectlApplyFromStringE(GinkgoT(), options, tpl)
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			err := k8s.DeleteNamespaceE(GinkgoT(), options, namespace)
			Expect(err).NotTo(HaveOccurred())
		})

		FIt("should work in a privileged namespace", func() {
			err := makeNamespacePrivileged(options, namespace)
			Expect(err).NotTo(HaveOccurred())

			// Create a pod in the privileged namespace
			tpl, err := helpers.TemplateFile("./fixtures/privileged-deployment.yaml.tmpl", "privileged-deployment.yaml.tmpl", template.FuncMap{
				"namespace": namespace,
			})
			Expect(err).NotTo(HaveOccurred())

			err = k8s.KubectlApplyFromStringE(GinkgoT(), options, tpl)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() error {
				_, err := c.Client.Clientset.CoreV1().Pods(namespace).Get(context.Background(), "privileged-integration-test", v1.GetOptions{})
				if err != nil {
					return err
				}
				return nil
			}, "2m", "10s").ShouldNot(BeNil())
		})

		FIt("shouldn't work in a unprivileged namespace", func() {
			// Create a pod in the unprivileged namespace
			tpl, err := helpers.TemplateFile("./fixtures/privileged-deployment.yaml.tmpl", "privileged-deployment.yaml.tmpl", template.FuncMap{
				"namespace": namespace,
			})
			Expect(err).NotTo(HaveOccurred())

			err = k8s.KubectlApplyFromStringE(GinkgoT(), options, tpl)
			Expect(err).NotTo(HaveOccurred())

			Consistently(func() error {
				_, err := c.Client.Clientset.CoreV1().Pods(namespace).Get(context.Background(), "privileged-integration-test", v1.GetOptions{})
				if err != nil {
					return err
				}
				return nil
			}, "10s", "5s").ShouldNot(BeNil())
		})
	})

	Context("when a container doesn't require privileges", func() {
		var (
			namespace string
			options   *k8s.KubectlOptions
		)

		BeforeEach(func() {
			namespace = fmt.Sprintf("%s-psp-%s", c.Prefix, strings.ToLower(random.UniqueId()))
			options = k8s.NewKubectlOptions("", "", namespace)

			tpl, err := helpers.TemplateFile("./fixtures/namespace.yaml.tmpl", "namespace.yaml.tmpl", template.FuncMap{
				"namespace": namespace,
			})
			Expect(err).NotTo(HaveOccurred())

			err = k8s.KubectlApplyFromStringE(GinkgoT(), options, tpl)
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			err := k8s.DeleteNamespaceE(GinkgoT(), options, namespace)
			Expect(err).NotTo(HaveOccurred())
		})

		FIt("should work in a privileged namespace", func() {
			err := makeNamespacePrivileged(options, namespace)
			Expect(err).NotTo(HaveOccurred())

			// Create a pod in the privileged namespace
			tpl, err := helpers.TemplateFile("./fixtures/unprivileged-deployment.yaml.tmpl", "unprivileged-deployment.yaml.tmpl", template.FuncMap{
				"namespace": namespace,
			})
			Expect(err).NotTo(HaveOccurred())

			err = k8s.KubectlApplyFromStringE(GinkgoT(),options, namespace), tpl)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() error {
				_, err := c.Client.Clientset.CoreV1().Pods("").Get(context.Background(), "privileged-integration-test", v1.GetOptions{})
				if err != nil {
					return err
				}
				return nil
			}, "2m", "10s").ShouldNot(BeNil())
		})

		It("should work in a unprivileged namespace", func() {
		})
	})
})

func makeNamespacePrivileged(options *k8s.KubectlOptions, namespace string) error {
	tpl, err := helpers.TemplateFile("./fixtures/namespace.yaml.tmpl", "namespace.yaml.tmpl", template.FuncMap{
		"namespace": namespace,
	})
	if err != nil {
		return err
	}

	err = k8s.KubectlApplyFromStringE(GinkgoT(), options, tpl)
	if err != nil {
		return err
	}

	// Create cluster role binding
	_, err = c.Client.Clientset.RbacV1().ClusterRoleBindings().Create(context.Background(), &rbac.ClusterRoleBinding{
		ObjectMeta: v1.ObjectMeta{
			Name:      namespace,
			Namespace: namespace,
		},
		RoleRef: rbac.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "psp:privileged",
		},
		Subjects: []rbac.Subject{
			{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "Group",
				Name:     "system:serviceaccounts:" + namespace,
			},
		},
	}, v1.CreateOptions{})
	if err != nil {
		return err
	}

	return nil
}
