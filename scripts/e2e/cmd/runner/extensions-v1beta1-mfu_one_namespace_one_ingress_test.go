// -------------------------------------------------------------------------------------------
// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License. See License.txt in the project root for license information.
// --------------------------------------------------------------------------------------------

//go:build e2e
// +build e2e

package runner

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	versioned "github.com/Azure/application-gateway-kubernetes-ingress/pkg/crd_client/agic_crd_client/clientset/versioned"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

var _ = Describe("extensions-v1beta1-MFU", func() {
	var (
		clientset *kubernetes.Clientset
		crdClient *versioned.Clientset
		err       error
	)

	Context("One Namespace One Ingress", func() {
		BeforeEach(func() {
			clientset, crdClient, err = getClients()
			Expect(err).To(BeNil())

			UseExtensionsV1Beta1Ingress = supportsExtensionsV1Beta1IngressPackage(clientset)
			skipIfExtensionsV1Beta1NotSupport()

			cleanUp(clientset)
		})

		It("[ssl-e2e-redirect] ssl termination and ssl redirect to https backend should work", func() {
			namespaceName := "e2e-ssl-e2e-redirect"
			ns := &v1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: namespaceName,
				},
			}
			klog.Info("Creating namespace: ", namespaceName)
			_, err = clientset.CoreV1().Namespaces().Create(context.TODO(), ns, metav1.CreateOptions{})
			Expect(err).To(BeNil())

			SSLE2ERedirectYamlPath := "testdata/extensions-v1beta1/one-namespace-one-ingress/ssl-e2e-redirect/app.yaml"
			klog.Info("Applying yaml: ", SSLE2ERedirectYamlPath)
			err = applyYaml(clientset, crdClient, namespaceName, SSLE2ERedirectYamlPath)
			Expect(err).To(BeNil())
			time.Sleep(30 * time.Second)

			// get ip address for 1 ingress
			klog.Info("Getting public IP from Ingress...")
			publicIP, _ := getPublicIP(clientset, namespaceName)
			Expect(publicIP).ToNot(Equal(""))

			urlHttp := fmt.Sprintf("http://%s/index.html", publicIP)
			urlHttps := fmt.Sprintf("https://%s/index.html", publicIP)
			// http get to return 301
			resp, err := makeGetRequest(urlHttp, "", 301, true)
			Expect(err).To(BeNil())
			redirectLocation := resp.Header.Get("Location")
			klog.Infof("redirect location: %s", redirectLocation)
			Expect(redirectLocation).To(Equal(urlHttps))
			// https get to return 200 ok
			_, err = makeGetRequest(urlHttps, "", 200, true)
			Expect(err).To(BeNil())

			//start to configure with bad hostname, 502 is expected
			healthConfigProbeBadHostnameYamlPath := "testdata/extensions-v1beta1/one-namespace-one-ingress/ssl-e2e-redirect/probe-hostname-bad.yaml"
			klog.Info("Updating ingress with bad hostname annotation")
			err = updateYaml(clientset, crdClient, namespaceName, healthConfigProbeBadHostnameYamlPath)
			Expect(err).To(BeNil())
			time.Sleep(30 * time.Second)
			_, err = makeGetRequest(urlHttps, "", 502, true)
			Expect(err).To(BeNil())

			// start to configure with good hostname, 200 is expected
			healthConfigProbeGoodHostnameYamlPath := "testdata/extensions-v1beta1/one-namespace-one-ingress/ssl-e2e-redirect/probe-hostname-good.yaml"
			klog.Info("Updating ingress with good hostname annotation")
			err = updateYaml(clientset, crdClient, namespaceName, healthConfigProbeGoodHostnameYamlPath)
			Expect(err).To(BeNil())
			time.Sleep(30 * time.Second)
			_, err = makeGetRequest(urlHttps, "", 200, true)
			Expect(err).To(BeNil())
		})

		It("[three-namespaces] containers with the same probe and labels in 3 different namespaces should have unique and working health probes", func() {
			// http get to return 200 ok
			for _, nm := range []string{"e2e-ns-x", "e2e-ns-y", "e2e-ns-z"} {
				ns := &v1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: nm,
					},
				}
				klog.Info("Creating namespace: ", nm)
				_, err = clientset.CoreV1().Namespaces().Create(context.TODO(), ns, metav1.CreateOptions{})
				Expect(err).To(BeNil())
			}
			threeNamespacesYamlPath := "testdata/extensions-v1beta1/one-namespace-one-ingress/three-namespaces/app.yaml"
			klog.Info("Applying yaml: ", threeNamespacesYamlPath)
			err = applyYaml(clientset, crdClient, "", threeNamespacesYamlPath)
			Expect(err).To(BeNil())
			time.Sleep(30 * time.Second)

			// get ip address for 1 ingress
			klog.Info("Getting public IP from Ingress...")
			publicIP, _ := getPublicIP(clientset, "e2e-ns-x")
			Expect(publicIP).ToNot(Equal(""))

			hosts := []string{"ws-e2e-ns-x.mis.li", "ws-e2e-ns-y.mis.li", "ws-e2e-ns-z.mis.li"}
			url := fmt.Sprintf("http://%s/status/200", publicIP)
			for _, host := range hosts {
				_, err = makeGetRequest(url, host, 200, true)
				Expect(err).To(BeNil())
			}
		})

		It("[health-probe-config] health probe configuration with annotation should be applied first", func() {
			namespaceName := "e2e-health-probe-config"
			ns := &v1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: namespaceName,
				},
			}
			klog.Info("Creating namespace: ", namespaceName)
			_, err = clientset.CoreV1().Namespaces().Create(context.TODO(), ns, metav1.CreateOptions{})
			Expect(err).To(BeNil())

			healthConfigProbeYamlPath := "testdata/extensions-v1beta1/one-namespace-one-ingress/health-probe-configurations/app.yaml"
			klog.Info("Applying yaml: ", healthConfigProbeYamlPath)
			err = applyYaml(clientset, crdClient, namespaceName, healthConfigProbeYamlPath)
			Expect(err).To(BeNil())
			time.Sleep(30 * time.Second)

			// get ip address for 1 ingress
			klog.Info("Getting public IP from Ingress...")
			publicIP, _ := getPublicIP(clientset, namespaceName)
			Expect(publicIP).ToNot(Equal(""))

			// initial deployment should be ok for the request
			url := fmt.Sprintf("http://%s/status/200", publicIP)
			_, err = makeGetRequest(url, "", 200, true)
			Expect(err).To(BeNil())

			// start to configure with bad path, 502 is expected
			healthConfigProbeBadPathYamlPath := "testdata/extensions-v1beta1/one-namespace-one-ingress/health-probe-configurations/probe-path-bad.yaml"
			klog.Info("Updating ingress with bad path annotation")
			err = updateYaml(clientset, crdClient, namespaceName, healthConfigProbeBadPathYamlPath)
			Expect(err).To(BeNil())
			time.Sleep(15 * time.Second)
			_, err = makeGetRequest(url, "", 502, true)
			Expect(err).To(BeNil())

			// start to configure with good path, 200 is expected
			healthConfigProbeGoodPathYamlPath := "testdata/extensions-v1beta1/one-namespace-one-ingress/health-probe-configurations/probe-path-good.yaml"
			klog.Info("Updating ingress with good path annotation")
			err = updateYaml(clientset, crdClient, namespaceName, healthConfigProbeGoodPathYamlPath)
			Expect(err).To(BeNil())
			time.Sleep(15 * time.Second)
			_, err = makeGetRequest(url, "", 200, true)
			Expect(err).To(BeNil())

			// start to configure with bad port, 502 is expected
			healthConfigProbeBadPortYamlPath := "testdata/extensions-v1beta1/one-namespace-one-ingress/health-probe-configurations/probe-port-bad.yaml"
			klog.Info("Updating ingress with bad port annotation")
			err = updateYaml(clientset, crdClient, namespaceName, healthConfigProbeBadPortYamlPath)
			Expect(err).To(BeNil())
			time.Sleep(15 * time.Second)
			_, err = makeGetRequest(url, "", 502, true)
			Expect(err).To(BeNil())

			// start to configure with good port, 200 is expected
			healthConfigProbeGoodPortYamlPath := "testdata/extensions-v1beta1/one-namespace-one-ingress/health-probe-configurations/probe-port-good.yaml"
			klog.Info("Updating ingress with good port annotation")
			err = updateYaml(clientset, crdClient, namespaceName, healthConfigProbeGoodPortYamlPath)
			Expect(err).To(BeNil())
			time.Sleep(15 * time.Second)
			_, err = makeGetRequest(url, "", 200, true)
			Expect(err).To(BeNil())

			// start to configure with bad status, 502 is expected
			healthConfigProbeBadStatusYamlPath := "testdata/extensions-v1beta1/one-namespace-one-ingress/health-probe-configurations/probe-status-bad.yaml"
			klog.Info("Updating ingress with bad status annotation")
			err = updateYaml(clientset, crdClient, namespaceName, healthConfigProbeBadStatusYamlPath)
			Expect(err).To(BeNil())
			time.Sleep(15 * time.Second)
			_, err = makeGetRequest(url, "", 502, true)
			Expect(err).To(BeNil())

			// start to configure with good status, 200 is expected
			healthConfigProbeGoodStatusYamlPath := "testdata/extensions-v1beta1/one-namespace-one-ingress/health-probe-configurations/probe-status-good.yaml"
			klog.Info("Updating ingress with good status annotation")
			err = updateYaml(clientset, crdClient, namespaceName, healthConfigProbeGoodStatusYamlPath)
			Expect(err).To(BeNil())
			time.Sleep(15 * time.Second)
			_, err = makeGetRequest(url, "", 200, true)
			Expect(err).To(BeNil())
		})

		It("[container-readiness-probe] backend should be removed when health probe is not healthy", func() {
			// http get to return 200 ok
			for _, nm := range []string{"e2e-probe1", "e2e-probe2"} {
				ns := &v1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: nm,
					},
				}
				klog.Info("Creating namespace: ", nm)
				_, err = clientset.CoreV1().Namespaces().Create(context.TODO(), ns, metav1.CreateOptions{})
				Expect(err).To(BeNil())
			}
			containerReadinessProbeYamlPath := "testdata/extensions-v1beta1/one-namespace-one-ingress/container-readiness-probe/app.yaml"
			klog.Info("Applying yaml: ", containerReadinessProbeYamlPath)
			err = applyYaml(clientset, crdClient, "", containerReadinessProbeYamlPath)
			Expect(err).To(BeNil())
			time.Sleep(30 * time.Second)

			// get ip address for 1 ingress
			klog.Info("Getting public IP from Ingress...")
			publicIP, _ := getPublicIP(clientset, "e2e-probe1")
			Expect(publicIP).ToNot(Equal(""))
			urlGood := fmt.Sprintf("http://%s/good", publicIP)
			urlBad := fmt.Sprintf("http://%s/bad", publicIP)
			_, err = makeGetRequest(urlGood, "ws.mis.li.probe", 200, true)
			Expect(err).To(BeNil())
			_, err = makeGetRequest(urlBad, "ws.mis.li.probe", 502, true)
			Expect(err).To(BeNil())
		})

		It("[retry access check] should be able to wait for the access to be granted", func() {
			klog.Info("Initializing role client")
			roleClient, err := getRoleAssignmentsClient()
			Expect(err).To(BeNil())

			// remove role assignment
			// output=$(az role assignment list --assignee $identityPrincipalId --subscription $subscription --all -o json | jq -r ".[].id") | xargs
			klog.Info("Removing all role assignments")
			err = removeRoleAssignments(roleClient)
			Expect(err).To(BeNil())

			// wait for 120 seconds
			klog.Info("Wait for 120 seconds")
			time.Sleep(120 * time.Second)

			klog.Info("Deleting AAD Pod identity pod")
			err = deleteAADPodIdentityPods(clientset)
			Expect(err).To(BeNil())

			// delete the AGIC pod. This will create the pod
			klog.Info("Deleting AGIC pod")
			err = deleteAGICPod(clientset)
			Expect(err).To(BeNil())

			// wait for 120 seconds
			klog.Info("Wait for 120 seconds")
			time.Sleep(120 * time.Second)

			// add the contributor assignment
			groupID := GetEnv().GetResourceGroupID()
			err = addRoleAssignment(roleClient, Contributor, groupID)
			Expect(err).To(BeNil())

			// install an app
			namespaceName := "e2e-retry-access-check"
			ns := &v1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: namespaceName,
				},
			}
			klog.Info("Creating namespace: ", namespaceName)
			_, err = clientset.CoreV1().Namespaces().Create(context.TODO(), ns, metav1.CreateOptions{})
			Expect(err).To(BeNil())

			SSLE2ERedirectYamlPath := "testdata/extensions-v1beta1/one-namespace-one-ingress/ssl-e2e-redirect/app.yaml"
			klog.Info("Applying yaml: ", SSLE2ERedirectYamlPath)
			err = applyYaml(clientset, crdClient, namespaceName, SSLE2ERedirectYamlPath)
			Expect(err).To(BeNil())

			// get ip address for 1 ingress
			klog.Info("Getting public IP from Ingress...")
			publicIP, _ := getPublicIP(clientset, namespaceName)
			Expect(publicIP).ToNot(Equal(""))

			// check 200 status
			url := fmt.Sprintf("https://%s/index.html", publicIP)
			_, err = makeGetRequest(url, "", 200, true)
			Expect(err).To(BeNil())
		})

		It("[override-frontend-port] should be able to use frontend port other than 80/443", func() {
			namespaceName := "e2e-override-frontend-port"
			ns := &v1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: namespaceName,
				},
			}
			klog.Info("Creating namespace: ", namespaceName)
			_, err = clientset.CoreV1().Namespaces().Create(context.TODO(), ns, metav1.CreateOptions{})
			Expect(err).To(BeNil())

			OverrideFrontendPortYamlPath := "testdata/extensions-v1beta1/one-namespace-one-ingress/override-frontend-port/app.yaml"
			klog.Info("Applying yaml: ", OverrideFrontendPortYamlPath)
			err = applyYaml(clientset, crdClient, namespaceName, OverrideFrontendPortYamlPath)
			Expect(err).To(BeNil())
			time.Sleep(30 * time.Second)

			// get ip address for 1 ingress
			klog.Info("Getting public IP from Ingress...")
			publicIP, _ := getPublicIP(clientset, namespaceName)
			Expect(publicIP).ToNot(Equal(""))

			urlHttp := fmt.Sprintf("http://%s:%d/good", publicIP, 8080)
			urlHttps := fmt.Sprintf("https://%s:%d/good", publicIP, 8443)
			// http get to return 200 ok
			_, err = makeGetRequest(urlHttp, "app.http", 200, true)
			Expect(err).To(BeNil())
			// https get to return 200 ok
			_, err = makeGetRequest(urlHttps, "app.https", 200, true)
			Expect(err).To(BeNil())
		})

		It("[configuration-reliability] should be able to work with an invalid configuration containing duplicate paths and multiple backend port", func() {
			namespaceName := "e2e-configuration-reliability"
			ns := &v1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: namespaceName,
				},
			}
			klog.Info("Creating namespace: ", namespaceName)
			_, err = clientset.CoreV1().Namespaces().Create(context.TODO(), ns, metav1.CreateOptions{})
			Expect(err).To(BeNil())

			InvalidConfigYamlPath := "testdata/extensions-v1beta1/one-namespace-one-ingress/invalid-configuration/app.yaml"
			klog.Info("Applying yaml: ", InvalidConfigYamlPath)
			err = applyYaml(clientset, crdClient, namespaceName, InvalidConfigYamlPath)
			Expect(err).To(BeNil())
			time.Sleep(30 * time.Second)

			// get ip address for 1 ingress
			klog.Info("Getting public IP from Ingress...")
			publicIP, _ := getPublicIP(clientset, namespaceName)
			Expect(publicIP).ToNot(Equal(""))

			url := fmt.Sprintf("http://%s/", publicIP)
			resp, err := makeGetRequest(url, "app.http", 200, true)
			Expect(err).To(BeNil())
			Expect(readBody(resp)).To(Equal("app"))

			url = fmt.Sprintf("http://%s/app", publicIP)
			resp, err = makeGetRequest(url, "app.http", 200, true)
			Expect(err).To(BeNil())
			Expect(readBody(resp)).To(Equal("app"))

			url = fmt.Sprintf("http://%s/app1", publicIP)
			resp, err = makeGetRequest(url, "app.http", 200, true)
			Expect(err).To(BeNil())
			Expect(readBody(resp)).To(Equal("app"))
		})

		It("[empty-secret] should be able to update application gateway if empty secret is populated", func() {
			namespaceName := "e2e-empty-secret"
			ns := &v1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: namespaceName,
				},
			}
			klog.Info("Creating namespace: ", namespaceName)
			_, err = clientset.CoreV1().Namespaces().Create(context.TODO(), ns, metav1.CreateOptions{})
			Expect(err).To(BeNil())

			EmptySecretYamlPath := "testdata/extensions-v1beta1/one-namespace-one-ingress/empty-secret/empty-secret.yaml"
			klog.Info("Applying empty secret yaml: ", EmptySecretYamlPath)
			err = applyYaml(clientset, crdClient, namespaceName, EmptySecretYamlPath)
			Expect(err).To(BeNil())
			time.Sleep(30 * time.Second)

			AppYamlPath := "testdata/extensions-v1beta1/one-namespace-one-ingress/empty-secret/app.yaml"
			klog.Info("Applying App yaml: ", AppYamlPath)
			err = applyYaml(clientset, crdClient, namespaceName, AppYamlPath)
			Expect(err).To(BeNil())
			time.Sleep(30 * time.Second)

			SecretYamlPath := "testdata/extensions-v1beta1/one-namespace-one-ingress/empty-secret/populated-secret.yaml"
			klog.Info("Update secret yaml: ", SecretYamlPath)
			err = updateYaml(clientset, crdClient, namespaceName, SecretYamlPath)
			Expect(err).To(BeNil())
			time.Sleep(30 * time.Second)

			// get ip address for 1 ingress
			klog.Info("Getting public IP from Ingress...")
			publicIP, _ := getPublicIP(clientset, namespaceName)
			Expect(publicIP).ToNot(Equal(""))

			urlHttps := fmt.Sprintf("https://%s", publicIP)
			// http get to return 200 ok
			_, err = makeGetRequest(urlHttps, "example.com", 200, true)
			Expect(err).To(BeNil())
		})

		AfterEach(func() {
			// clear all namespaces
			cleanUp(clientset)
		})
	})
})
