//go:build e2e
// +build e2e

package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/conduit-ops/mkurator/test/utils"
)

const metricsCurlImage = "mkurator-e2e-curl:dev"

var (
	// managerImage is the manager image to be built and loaded for testing.
	managerImage = "mkurator-controller-manager:dev"
	// shouldCleanupCertManager tracks whether CertManager was installed by this suite.
	shouldCleanupCertManager = false
)

// TestE2E runs the e2e test suite to validate the solution in an isolated environment.
// The default setup requires Kind and CertManager.
//
// To enable kubectl kuberc (use custom kubectl configurations), set: KUBECTL_KUBERC=true
// By default, kuberc is disabled to ensure consistent test behavior across different environments.
// To skip CertManager installation, set: CERT_MANAGER_INSTALL_SKIP=true
func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	_, _ = fmt.Fprintf(GinkgoWriter, "Starting mkurator e2e test suite\n")
	RunSpecs(t, "e2e suite")
}

// SynchronizedBeforeSuite: process 1 builds/loads images, cert-manager, and deploys the operator once.
var _ = SynchronizedBeforeSuite(func() []byte {
	e2eStage("PLATFORM PREP — build/load images, cert-manager, deploy (process 1)")
	By("building the manager image")
	cmd := exec.Command("task", "docker:build")
	cmd.Env = append(os.Environ(), fmt.Sprintf("DOCKER_IMAGE=%s", managerImage))
	_, err := utils.Run(cmd)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to build the manager image")

	By("loading the manager image on Kind")
	err = utils.LoadImageToKindClusterWithName(managerImage)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to load the manager image into Kind")

	By("building the curl image for metrics tests")
	projectDir, err := utils.GetProjectDir()
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	curlBuildCtx := filepath.Join(projectDir, "test", "e2e", "fixtures", "metrics-curl")
	buildCurl := exec.Command("docker", "build", "--provenance=false", "--sbom=false",
		"-t", metricsCurlImage, curlBuildCtx)
	_, err = utils.Run(buildCurl)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to build curl image for metrics e2e")
	err = utils.LoadImageToKindClusterWithName(metricsCurlImage)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to load curl image into Kind")

	setupCertManager()
	ensureManagerNamespaceAndDeploy()
	applyChannelAuthPrereqFixtureOnce()
	return nil
}, func(_ []byte) {
	configureKubectlKubeRC()
	// Every parallel Ginkgo process must wait for CRD discovery before applying CRs.
	waitForMKuratorCRDsEstablished()
})

var _ = AfterSuite(func() {
	teardownCertManager()
	cleanupE2EResources()
})

// Disable kubectl kuberc by default for test isolation.
func configureKubectlKubeRC() {
	if os.Getenv("KUBECTL_KUBERC") != "true" {
		By("disabling kubectl kuberc for test isolation")
		err := os.Setenv("KUBECTL_KUBERC", "false")
		ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to disable kubectl kuberc")
		_, _ = fmt.Fprintf(GinkgoWriter,
			"kubectl kuberc disabled for consistent test behavior (override with KUBECTL_KUBERC=true)\n")
	} else {
		_, _ = fmt.Fprintf(GinkgoWriter, "kubectl kuberc enabled (KUBECTL_KUBERC=true)\n")
	}
}

func setupCertManager() {
	if os.Getenv("CERT_MANAGER_INSTALL_SKIP") == "true" {
		_, _ = fmt.Fprintf(GinkgoWriter, "Skipping CertManager installation (CERT_MANAGER_INSTALL_SKIP=true)\n")
		return
	}

	By("checking if CertManager is already installed")
	if utils.IsCertManagerCRDsInstalled() {
		_, _ = fmt.Fprintf(GinkgoWriter, "CertManager is already installed. Skipping installation.\n")
		return
	}

	shouldCleanupCertManager = true

	By("installing CertManager")
	Expect(utils.InstallCertManager()).To(Succeed(), "Failed to install CertManager")
}

func teardownCertManager() {
	if !shouldCleanupCertManager {
		_, _ = fmt.Fprintf(GinkgoWriter, "Skipping CertManager cleanup (not installed by this suite)\n")
		return
	}

	By("uninstalling CertManager")
	utils.UninstallCertManager()
}
