package curatedpackages_test

import (
	"bytes"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"

	packagesv1 "github.com/aws/eks-anywhere-packages/api/v1alpha1"
	"github.com/aws/eks-anywhere/pkg/constants"
	"github.com/aws/eks-anywhere/pkg/curatedpackages"
	"github.com/aws/eks-anywhere/pkg/curatedpackages/mocks"
)

const (
	cronJobName = "cronjob/cron-ecr-renew"
	jobName     = "eksa-auth-refresher"
)

type packageControllerTest struct {
	*WithT
	ctx            context.Context
	kubectl        *mocks.MockKubectlRunner
	chartInstaller *mocks.MockChartInstaller
	command        *curatedpackages.PackageControllerClient
	clusterName    string
	kubeConfig     string
	ociUri         string
	chartName      string
	chartVersion   string
	eksaAccessId   string
	eksaAccessKey  string
	eksaRegion     string
	httpProxy      string
	httpsProxy     string
	noProxy        []string
}

func newPackageControllerTest(t *testing.T) *packageControllerTest {
	ctrl := gomock.NewController(t)
	k := mocks.NewMockKubectlRunner(ctrl)
	ci := mocks.NewMockChartInstaller(ctrl)
	kubeConfig := "kubeconfig.kubeconfig"
	uri := "test/registry_name"
	chartName := "test_controller"
	chartVersion := "v1.0.0"
	eksaAccessId := "test-access-id"
	eksaAccessKey := "test-access-key"
	eksaRegion := "test-region"
	clusterName := "billy"
	return &packageControllerTest{
		WithT:          NewWithT(t),
		ctx:            context.Background(),
		kubectl:        k,
		chartInstaller: ci,
		command: curatedpackages.NewPackageControllerClient(
			ci, k, clusterName, kubeConfig, uri, chartName, chartVersion,
			curatedpackages.WithEksaSecretAccessKey(eksaAccessKey),
			curatedpackages.WithEksaRegion(eksaRegion),
			curatedpackages.WithEksaAccessKeyId(eksaAccessId),
			curatedpackages.WithManagementClusterName("mgmt"),
		),
		clusterName:   clusterName,
		kubeConfig:    kubeConfig,
		ociUri:        uri,
		chartName:     chartName,
		chartVersion:  chartVersion,
		eksaAccessId:  eksaAccessId,
		eksaAccessKey: eksaAccessKey,
		eksaRegion:    eksaRegion,
		httpProxy:     "1.1.1.1",
		httpsProxy:    "1.1.1.1",
		noProxy:       []string{"1.1.1.1/24"},
	}
}

func TestInstallControllerSuccess(t *testing.T) {
	tt := newPackageControllerTest(t)

	registry := curatedpackages.GetRegistry(tt.ociUri)
	sourceRegistry := fmt.Sprintf("sourceRegistry=%s", registry)
	clusterName := fmt.Sprintf("clusterName=%s", "billy")
	values := []string{sourceRegistry, clusterName}
	params := []string{"create", "-f", "-", "--kubeconfig", tt.kubeConfig}
	dat, err := os.ReadFile("testdata/awssecret_test.yaml")
	tt.Expect(err).NotTo(HaveOccurred())
	tt.kubectl.EXPECT().ExecuteFromYaml(tt.ctx, dat, params).Return(bytes.Buffer{}, nil)
	params = []string{"create", "job", jobName, "--from=" + cronJobName, "--kubeconfig", tt.kubeConfig, "--namespace", constants.EksaPackagesName}
	tt.kubectl.EXPECT().ExecuteCommand(tt.ctx, params).Return(bytes.Buffer{}, nil)
	tt.chartInstaller.EXPECT().InstallChart(tt.ctx, tt.chartName, "oci://"+tt.ociUri, tt.chartVersion, tt.kubeConfig, "eksa-packages", values).Return(nil)
	any := gomock.Any()
	tt.kubectl.EXPECT().
		GetObject(any, any, any, any, any, any).
		DoAndReturn(getPBCSuccess(t)).
		AnyTimes()

	err = tt.command.InstallController(tt.ctx)
	if err != nil {
		t.Errorf("Install Controller Should succeed when installation passes")
	}
}

func getPBCSuccess(t *testing.T) func(context.Context, string, string, string, string, *packagesv1.PackageBundleController) error {
	return func(_ context.Context, _, _, _, _ string, obj *packagesv1.PackageBundleController) error {
		pbc := &packagesv1.PackageBundleController{
			Spec: packagesv1.PackageBundleControllerSpec{
				ActiveBundle: "test-bundle",
			},
		}
		pbc.DeepCopyInto(obj)
		return nil
	}
}

func getPBCFail(t *testing.T) func(context.Context, string, string, string, string, *packagesv1.PackageBundleController) error {
	return func(_ context.Context, _, _, _, _ string, obj *packagesv1.PackageBundleController) error {
		return fmt.Errorf("test error")
	}
}

func TestInstallControllerWithProxy(t *testing.T) {
	tt := newPackageControllerTest(t)
	tt.command = curatedpackages.NewPackageControllerClient(
		tt.chartInstaller, tt.kubectl, "billy", tt.kubeConfig, tt.ociUri, tt.chartName, tt.chartVersion,
		curatedpackages.WithEksaSecretAccessKey(tt.eksaAccessKey),
		curatedpackages.WithEksaRegion(tt.eksaRegion),
		curatedpackages.WithEksaAccessKeyId(tt.eksaAccessId),
		curatedpackages.WithHTTPProxy(tt.httpProxy),
		curatedpackages.WithHTTPSProxy(tt.httpsProxy),
		curatedpackages.WithNoProxy(tt.noProxy),
	)

	registry := curatedpackages.GetRegistry(tt.ociUri)
	sourceRegistry := fmt.Sprintf("sourceRegistry=%s", registry)
	clusterName := fmt.Sprintf("clusterName=%s", "billy")
	httpProxy := fmt.Sprintf("proxy.HTTP_PROXY=%s", tt.httpProxy)
	httpsProxy := fmt.Sprintf("proxy.HTTPS_PROXY=%s", tt.httpsProxy)
	noProxy := fmt.Sprintf("proxy.NO_PROXY=%s", strings.Join(tt.noProxy, "\\,"))

	values := []string{sourceRegistry, clusterName, httpProxy, httpsProxy, noProxy}
	params := []string{"create", "-f", "-", "--kubeconfig", tt.kubeConfig}
	dat, err := os.ReadFile("testdata/awssecret_test.yaml")
	tt.Expect(err).NotTo(HaveOccurred())
	tt.kubectl.EXPECT().ExecuteFromYaml(tt.ctx, dat, params).Return(bytes.Buffer{}, nil)
	params = []string{"create", "job", jobName, "--from=" + cronJobName, "--kubeconfig", tt.kubeConfig, "--namespace", constants.EksaPackagesName}
	tt.kubectl.EXPECT().ExecuteCommand(tt.ctx, params).Return(bytes.Buffer{}, nil)
	tt.chartInstaller.EXPECT().InstallChart(tt.ctx, tt.chartName, "oci://"+tt.ociUri, tt.chartVersion, tt.kubeConfig, "eksa-packages", values).Return(nil)
	any := gomock.Any()
	tt.kubectl.EXPECT().
		GetObject(any, any, any, any, any, any).
		DoAndReturn(getPBCSuccess(t)).
		AnyTimes()

	err = tt.command.InstallController(tt.ctx)
	if err != nil {
		t.Errorf("Install Controller Should succeed when installation passes")
	}
}

func TestInstallControllerWithEmptyProxy(t *testing.T) {
	tt := newPackageControllerTest(t)
	tt.command = curatedpackages.NewPackageControllerClient(
		tt.chartInstaller, tt.kubectl, "billy", tt.kubeConfig, tt.ociUri, tt.chartName, tt.chartVersion,
		curatedpackages.WithEksaSecretAccessKey(tt.eksaAccessKey),
		curatedpackages.WithEksaRegion(tt.eksaRegion),
		curatedpackages.WithEksaAccessKeyId(tt.eksaAccessId),
		curatedpackages.WithHTTPProxy(""),
		curatedpackages.WithHTTPSProxy(""),
		curatedpackages.WithNoProxy(nil),
	)

	registry := curatedpackages.GetRegistry(tt.ociUri)
	sourceRegistry := fmt.Sprintf("sourceRegistry=%s", registry)
	clusterName := fmt.Sprintf("clusterName=%s", "billy")

	values := []string{sourceRegistry, clusterName}
	params := []string{"create", "-f", "-", "--kubeconfig", tt.kubeConfig}
	dat, err := os.ReadFile("testdata/awssecret_test.yaml")
	tt.Expect(err).NotTo(HaveOccurred())
	tt.kubectl.EXPECT().ExecuteFromYaml(tt.ctx, dat, params).Return(bytes.Buffer{}, nil)
	params = []string{"create", "job", jobName, "--from=" + cronJobName, "--kubeconfig", tt.kubeConfig, "--namespace", constants.EksaPackagesName}
	tt.kubectl.EXPECT().ExecuteCommand(tt.ctx, params).Return(bytes.Buffer{}, nil)
	tt.chartInstaller.EXPECT().InstallChart(tt.ctx, tt.chartName, "oci://"+tt.ociUri, tt.chartVersion, tt.kubeConfig, "eksa-packages", values).Return(nil)
	any := gomock.Any()
	tt.kubectl.EXPECT().
		GetObject(any, any, any, any, any, any).
		DoAndReturn(getPBCSuccess(t)).
		AnyTimes()

	err = tt.command.InstallController(tt.ctx)
	if err != nil {
		t.Errorf("Install Controller Should succeed when installation passes")
	}
}

func TestInstallControllerFail(t *testing.T) {
	tt := newPackageControllerTest(t)
	registry := curatedpackages.GetRegistry(tt.ociUri)
	sourceRegistry := fmt.Sprintf("sourceRegistry=%s", registry)
	clusterName := fmt.Sprintf("clusterName=%s", "billy")
	values := []string{sourceRegistry, clusterName}

	tt.chartInstaller.EXPECT().InstallChart(tt.ctx, tt.chartName, "oci://"+tt.ociUri, tt.chartVersion, tt.kubeConfig, "eksa-packages", values).Return(errors.New("login failed"))
	any := gomock.Any()
	tt.kubectl.EXPECT().
		GetObject(any, any, any, any, any, any).
		DoAndReturn(getPBCSuccess(t)).
		AnyTimes()

	err := tt.command.InstallController(tt.ctx)
	if err == nil {
		t.Errorf("Install Controller Should fail when installation fails")
	}
}

func TestInstallControllerFailNoActiveBundle(t *testing.T) {
	tt := newPackageControllerTest(t)

	registry := curatedpackages.GetRegistry(tt.ociUri)
	sourceRegistry := fmt.Sprintf("sourceRegistry=%s", registry)
	clusterName := fmt.Sprintf("clusterName=%s", "billy")
	values := []string{sourceRegistry, clusterName}
	params := []string{"create", "-f", "-", "--kubeconfig", tt.kubeConfig}
	dat, err := os.ReadFile("testdata/awssecret_test.yaml")
	tt.Expect(err).NotTo(HaveOccurred())
	tt.kubectl.EXPECT().ExecuteFromYaml(tt.ctx, dat, params).Return(bytes.Buffer{}, nil)
	params = []string{"create", "job", jobName, "--from=" + cronJobName, "--kubeconfig", tt.kubeConfig, "--namespace", constants.EksaPackagesName}
	tt.kubectl.EXPECT().ExecuteCommand(tt.ctx, params).Return(bytes.Buffer{}, nil)
	tt.chartInstaller.EXPECT().InstallChart(tt.ctx, tt.chartName, "oci://"+tt.ociUri, tt.chartVersion, tt.kubeConfig, "eksa-packages", values).Return(nil)
	any := gomock.Any()
	tt.kubectl.EXPECT().
		GetObject(any, any, any, any, any, any).
		DoAndReturn(getPBCFail(t)).
		AnyTimes()

	err = tt.command.InstallController(tt.ctx)
	if err == nil {
		t.Errorf("expected error, got nil")
	}
}

func TestInstallControllerSuccessWhenApplySecretFails(t *testing.T) {
	tt := newPackageControllerTest(t)

	registry := curatedpackages.GetRegistry(tt.ociUri)
	sourceRegistry := fmt.Sprintf("sourceRegistry=%s", registry)
	clusterName := fmt.Sprintf("clusterName=%s", "billy")
	values := []string{sourceRegistry, clusterName}
	params := []string{"create", "-f", "-", "--kubeconfig", tt.kubeConfig}
	dat, err := os.ReadFile("testdata/awssecret_test.yaml")
	tt.Expect(err).To(BeNil())
	tt.kubectl.EXPECT().ExecuteFromYaml(tt.ctx, dat, params).Return(bytes.Buffer{}, errors.New("error applying secrets"))
	params = []string{"create", "job", jobName, "--from=" + cronJobName, "--kubeconfig", tt.kubeConfig, "--namespace", constants.EksaPackagesName}
	tt.kubectl.EXPECT().ExecuteCommand(tt.ctx, params).Return(bytes.Buffer{}, nil)
	tt.chartInstaller.EXPECT().InstallChart(tt.ctx, tt.chartName, "oci://"+tt.ociUri, tt.chartVersion, tt.kubeConfig, "eksa-packages", values).Return(nil)
	any := gomock.Any()
	tt.kubectl.EXPECT().
		GetObject(any, any, any, any, any, any).
		DoAndReturn(getPBCSuccess(t)).
		AnyTimes()

	err = tt.command.InstallController(tt.ctx)
	if err != nil {
		t.Errorf("Install Controller Should succeed when secret creation fails")
	}
}

func TestInstallControllerSuccessWhenCronJobFails(t *testing.T) {
	tt := newPackageControllerTest(t)

	registry := curatedpackages.GetRegistry(tt.ociUri)
	sourceRegistry := fmt.Sprintf("sourceRegistry=%s", registry)
	clusterName := fmt.Sprintf("clusterName=%s", "billy")
	values := []string{sourceRegistry, clusterName}
	params := []string{"create", "-f", "-", "--kubeconfig", tt.kubeConfig}
	dat, err := os.ReadFile("testdata/awssecret_test.yaml")
	tt.Expect(err).To(BeNil())
	tt.kubectl.EXPECT().ExecuteFromYaml(tt.ctx, dat, params).Return(bytes.Buffer{}, nil)
	params = []string{"create", "job", jobName, "--from=" + cronJobName, "--kubeconfig", tt.kubeConfig, "--namespace", constants.EksaPackagesName}
	tt.kubectl.EXPECT().ExecuteCommand(tt.ctx, params).Return(bytes.Buffer{}, errors.New("error creating cron job"))
	tt.chartInstaller.EXPECT().InstallChart(tt.ctx, tt.chartName, "oci://"+tt.ociUri, tt.chartVersion, tt.kubeConfig, "eksa-packages", values).Return(nil)
	any := gomock.Any()
	tt.kubectl.EXPECT().
		GetObject(any, any, any, any, any, any).
		DoAndReturn(getPBCSuccess(t)).
		AnyTimes()

	err = tt.command.InstallController(tt.ctx)
	if err != nil {
		t.Errorf("Install Controller Should succeed when cron job fails")
	}
}

func TestIsInstalledTrue(t *testing.T) {
	tt := newPackageControllerTest(t)

	tt.kubectl.EXPECT().HasResource(tt.ctx, "packageBundleController", tt.clusterName, tt.kubeConfig, constants.EksaPackagesName).Return(false, nil)

	found := tt.command.IsInstalled(tt.ctx)
	if found {
		t.Errorf("expected true, got %t", found)
	}
}

func TestIsInstalledFalse(t *testing.T) {
	tt := newPackageControllerTest(t)

	tt.kubectl.EXPECT().HasResource(tt.ctx, "packageBundleController", tt.clusterName, tt.kubeConfig, constants.EksaPackagesName).
		Return(false, errors.New("controller doesn't exist"))

	found := tt.command.IsInstalled(tt.ctx)
	if found {
		t.Errorf("expected false, got %t", found)
	}
}

func TestDefaultEksaRegionSetWhenNoRegionSpecified(t *testing.T) {
	tt := newPackageControllerTest(t)

	registry := curatedpackages.GetRegistry(tt.ociUri)
	sourceRegistry := fmt.Sprintf("sourceRegistry=%s", registry)
	clusterName := fmt.Sprintf("clusterName=%s", "billy")
	values := []string{sourceRegistry, clusterName}
	params := []string{"create", "-f", "-", "--kubeconfig", tt.kubeConfig}
	dat, err := os.ReadFile("testdata/awssecret_defaultregion.yaml")
	tt.Expect(err).To(BeNil())
	tt.kubectl.EXPECT().ExecuteFromYaml(tt.ctx, dat, params).Return(bytes.Buffer{}, nil)
	params = []string{"create", "job", jobName, "--from=" + cronJobName, "--kubeconfig", tt.kubeConfig, "--namespace", constants.EksaPackagesName}
	tt.kubectl.EXPECT().ExecuteCommand(tt.ctx, params).Return(bytes.Buffer{}, errors.New("error creating cron job"))
	tt.chartInstaller.EXPECT().InstallChart(tt.ctx, tt.chartName, "oci://"+tt.ociUri, tt.chartVersion, tt.kubeConfig, "eksa-packages", values).Return(nil)
	any := gomock.Any()
	tt.kubectl.EXPECT().
		GetObject(any, any, any, any, any, any).
		DoAndReturn(getPBCSuccess(t)).
		AnyTimes()

	tt.command = curatedpackages.NewPackageControllerClient(
		tt.chartInstaller, tt.kubectl, "billy", tt.kubeConfig, tt.ociUri, tt.chartName, tt.chartVersion,
		curatedpackages.WithEksaRegion(""),
		curatedpackages.WithEksaAccessKeyId(tt.eksaAccessId),
		curatedpackages.WithEksaSecretAccessKey(tt.eksaAccessKey),
	)
	err = tt.command.InstallController(tt.ctx)
	if err != nil {
		t.Errorf("Install Controller Should succeed when cron job fails")
	}
}

func TestInstallControllerActiveBundleCustomTimeout(t *testing.T) {
	tt := newPackageControllerTest(t)
	tt.command = curatedpackages.NewPackageControllerClient(
		tt.chartInstaller, tt.kubectl, "billy", tt.kubeConfig, tt.ociUri, tt.chartName, tt.chartVersion,
		curatedpackages.WithEksaSecretAccessKey(tt.eksaAccessKey),
		curatedpackages.WithEksaRegion(tt.eksaRegion),
		curatedpackages.WithEksaAccessKeyId(tt.eksaAccessId),
		curatedpackages.WithActiveBundleTimeout(time.Second),
	)

	registry := curatedpackages.GetRegistry(tt.ociUri)
	sourceRegistry := fmt.Sprintf("sourceRegistry=%s", registry)
	clusterName := fmt.Sprintf("clusterName=%s", "billy")
	values := []string{sourceRegistry, clusterName}
	params := []string{"create", "-f", "-", "--kubeconfig", tt.kubeConfig}
	dat, err := os.ReadFile("testdata/awssecret_test.yaml")
	tt.Expect(err).NotTo(HaveOccurred())
	tt.kubectl.EXPECT().ExecuteFromYaml(tt.ctx, dat, params).Return(bytes.Buffer{}, nil)
	params = []string{"create", "job", jobName, "--from=" + cronJobName, "--kubeconfig", tt.kubeConfig, "--namespace", constants.EksaPackagesName}
	tt.kubectl.EXPECT().ExecuteCommand(tt.ctx, params).Return(bytes.Buffer{}, nil)
	tt.chartInstaller.EXPECT().InstallChart(tt.ctx, tt.chartName, "oci://"+tt.ociUri, tt.chartVersion, tt.kubeConfig, "eksa-packages", values).Return(nil)
	any := gomock.Any()
	tt.kubectl.EXPECT().
		GetObject(any, any, any, any, any, any).
		DoAndReturn(getPBCSuccess(t)).
		AnyTimes()

	err = tt.command.InstallController(tt.ctx)
	if err != nil {
		t.Errorf("Install Controller Should succeed when installation passes")
	}
}

func TestInstallControllerActiveBundleWaitLoops(t *testing.T) {
	tt := newPackageControllerTest(t)

	registry := curatedpackages.GetRegistry(tt.ociUri)
	sourceRegistry := fmt.Sprintf("sourceRegistry=%s", registry)
	clusterName := fmt.Sprintf("clusterName=%s", "billy")
	values := []string{sourceRegistry, clusterName}
	params := []string{"create", "-f", "-", "--kubeconfig", tt.kubeConfig}
	dat, err := os.ReadFile("testdata/awssecret_test.yaml")
	tt.Expect(err).NotTo(HaveOccurred())
	tt.kubectl.EXPECT().ExecuteFromYaml(tt.ctx, dat, params).Return(bytes.Buffer{}, nil)
	params = []string{"create", "job", jobName, "--from=" + cronJobName, "--kubeconfig", tt.kubeConfig, "--namespace", constants.EksaPackagesName}
	tt.kubectl.EXPECT().ExecuteCommand(tt.ctx, params).Return(bytes.Buffer{}, nil)
	tt.chartInstaller.EXPECT().InstallChart(tt.ctx, tt.chartName, "oci://"+tt.ociUri, tt.chartVersion, tt.kubeConfig, "eksa-packages", values).Return(nil)
	any := gomock.Any()
	tt.kubectl.EXPECT().
		GetObject(any, any, any, any, any, any).
		DoAndReturn(getPBCLoops(t, 3)).
		AnyTimes()

	err = tt.command.InstallController(tt.ctx)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func getPBCLoops(t *testing.T, loops int) func(context.Context, string, string, string, string, *packagesv1.PackageBundleController) error {
	return func(_ context.Context, _, _, _, _ string, obj *packagesv1.PackageBundleController) error {
		loops = loops - 1
		if loops > 0 {
			return nil
		}
		pbc := &packagesv1.PackageBundleController{
			Spec: packagesv1.PackageBundleControllerSpec{
				ActiveBundle: "test-bundle",
			},
		}
		pbc.DeepCopyInto(obj)
		return nil
	}
}

func TestInstallControllerActiveBundleTimesOut(t *testing.T) {
	tt := newPackageControllerTest(t)
	tt.command = curatedpackages.NewPackageControllerClient(
		tt.chartInstaller, tt.kubectl, "billy", tt.kubeConfig, tt.ociUri, tt.chartName, tt.chartVersion,
		curatedpackages.WithEksaSecretAccessKey(tt.eksaAccessKey),
		curatedpackages.WithEksaRegion(tt.eksaRegion),
		curatedpackages.WithEksaAccessKeyId(tt.eksaAccessId),
		curatedpackages.WithActiveBundleTimeout(time.Millisecond),
	)

	registry := curatedpackages.GetRegistry(tt.ociUri)
	sourceRegistry := fmt.Sprintf("sourceRegistry=%s", registry)
	clusterName := fmt.Sprintf("clusterName=%s", "billy")
	values := []string{sourceRegistry, clusterName}
	params := []string{"create", "-f", "-", "--kubeconfig", tt.kubeConfig}
	dat, err := os.ReadFile("testdata/awssecret_test.yaml")
	tt.Expect(err).NotTo(HaveOccurred())
	tt.kubectl.EXPECT().ExecuteFromYaml(tt.ctx, dat, params).Return(bytes.Buffer{}, nil)
	params = []string{"create", "job", jobName, "--from=" + cronJobName, "--kubeconfig", tt.kubeConfig, "--namespace", constants.EksaPackagesName}
	tt.kubectl.EXPECT().ExecuteCommand(tt.ctx, params).Return(bytes.Buffer{}, nil)
	tt.chartInstaller.EXPECT().InstallChart(tt.ctx, tt.chartName, "oci://"+tt.ociUri, tt.chartVersion, tt.kubeConfig, "eksa-packages", values).Return(nil)
	any := gomock.Any()
	tt.kubectl.EXPECT().
		GetObject(any, any, any, any, any, any).
		DoAndReturn(getPBCDelay(t, time.Second)).
		AnyTimes()

	err = tt.command.InstallController(tt.ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected %v, got %v", context.DeadlineExceeded, err)
	}
}

func getPBCDelay(t *testing.T, delay time.Duration) func(context.Context, string, string, string, string, *packagesv1.PackageBundleController) error {
	return func(_ context.Context, _, _, _, _ string, obj *packagesv1.PackageBundleController) error {
		time.Sleep(delay)
		return fmt.Errorf("test error")
	}
}
