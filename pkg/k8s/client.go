/**
 * @Author Herb
 * @Date 2023/8/14 10:03
 **/

package k8s

import (
	"context"
	"errors"
	"fmt"
	"github.com/sirupsen/logrus"
	"github.com/wxnacy/wgo/arrays"
	"istio.io/client-go/pkg/apis/networking/v1alpha3"
	versionedclient "istio.io/client-go/pkg/clientset/versioned"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/printers"
	cliresource "k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/kubectl/pkg/cmd/apply"
	"k8s.io/kubectl/pkg/cmd/delete"
	"k8s.io/kubectl/pkg/cmd/rollout"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/polymorphichelpers"
	"math/rand"
	"net/url"
	"os"
	"strings"
	"time"
)

type ClientInfo struct {
	Host  string
	Token string
}

type K8sInfo struct {
	ApiServer string
	Token     string
	Name      string
}

const (
	CONTAINERD   = "containerd"
	ErrConfig    = "获取Config失败"
	ErrClientSet = "获取Client失败"
)

func GetClusterClientInfo(cluster K8sInfo) ClientInfo {
	host := cluster.ApiServer
	if !strings.HasPrefix(host, "http") {
		host = "https://" + host
	}
	return ClientInfo{
		Host:  host,
		Token: cluster.Token,
	}
}

func (c *ClientInfo) Apply(filesOrPath []string) (flag bool) {
	logrus.Trace("Start Apply ...")
	kubeConfigFlags := genericclioptions.NewConfigFlags(false).WithDeprecatedPasswordFlag()
	kubeConfigFlags.WrapConfigFn = func(*rest.Config) *rest.Config {
		return c.GetRestConfig()
	}
	builder := cliresource.NewBuilder(kubeConfigFlags)
	ioStreams := genericclioptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr}
	o := apply.NewApplyOptions(ioStreams)
	o.Builder = builder
	o.DeleteOptions = &delete.DeleteOptions{
		FilenameOptions: cliresource.FilenameOptions{
			// target k8s yaml files and directories that contain k8s yaml files
			Filenames: filesOrPath,
			Recursive: false,
		},
	}
	o.ToPrinter = func(operation string) (printers.ResourcePrinter, error) {
		o.PrintFlags.NamePrintFlags.Operation = operation
		return o.PrintFlags.ToPrinter()
	}
	flag = true
	err := o.Run()
	if err != nil {
		logrus.Error("Apply file error:", err)
		flag = false
	}
	logrus.Trace("End Apply.")
	return
}

func (c *ClientInfo) GetRestConfig() *rest.Config {
	return &rest.Config{
		Host:        c.Host,
		BearerToken: c.Token,
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: true,
		},
	}
}

func (c *ClientInfo) GetSvc(svcName string, ns string) *corev1.Service {

	obj, err := c.GetUnstructuredData("services", "v1", "", ns, svcName)
	if obj == nil {
		return nil
	}
	svcObj := &corev1.Service{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), svcObj)
	if err != nil {
		logrus.Error("Unstructured对象转Service出错：", err)
	}
	return svcObj
}

func (c *ClientInfo) GetUnstructuredData(res string, version string, group string, ns string, name string) (*unstructured.Unstructured, error) {
	config := c.GetRestConfig()
	if config == nil {
		logrus.Errorf(ErrConfig)
		return nil, errors.New(ErrConfig)
	}
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		logrus.Error(ErrClientSet, err)
		return nil, errors.New(ErrClientSet)
	}
	gvr := schema.GroupVersionResource{Group: group, Version: version, Resource: res}
	obj, err := dynamicClient.Resource(gvr).
		Namespace(ns).
		Get(context.Background(), name, metav1.GetOptions{})
	if err != nil && k8serrors.IsNotFound(err) {
		return nil, nil
	} else if err != nil {
		logrus.Error("获取资源出错", err)
		return nil, errors.New("获取资源出错")
	}
	return obj, nil
}
func (c *ClientInfo) UpdateData(res string, version string, group string, ns string, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	config := c.GetRestConfig()
	if config == nil {
		logrus.Errorf(ErrConfig)
		return nil, errors.New(ErrConfig)
	}
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		logrus.Error(ErrClientSet, err)
		return nil, errors.New(ErrClientSet)
	}
	gvr := schema.GroupVersionResource{Group: group, Version: version, Resource: res}
	obj, err = dynamicClient.Resource(gvr).Namespace(ns).
		Update(context.Background(), obj, metav1.UpdateOptions{})
	if err != nil && k8serrors.IsNotFound(err) {
		return nil, nil
	} else if err != nil {
		logrus.Error("更新资源出错", err)
		return nil, errors.New("更新资源出错")
	}
	return obj, nil
}

func (c *ClientInfo) GetClient() (*kubernetes.Clientset, error) {
	config := c.GetRestConfig()
	if config == nil {
		logrus.Errorf(ErrConfig)
		return nil, errors.New(ErrConfig)
	}
	return kubernetes.NewForConfig(config)
}
func (c *ClientInfo) GetIstioClient() (*versionedclient.Clientset, error) {
	config := c.GetRestConfig()
	if config == nil {
		logrus.Errorf(ErrConfig)
		return nil, errors.New(ErrConfig)
	}
	return versionedclient.NewForConfig(config)
}

func (c *ClientInfo) GetDynamic() (dynamic.Interface, error) {
	config := c.GetRestConfig()
	if config == nil {
		logrus.Errorf(ErrConfig)
		return nil, errors.New(ErrConfig)
	}
	return dynamic.NewForConfig(config)
}

func (c *ClientInfo) ListNs() []corev1.Namespace {
	clients, err := c.GetClient()
	if err != nil {
		logrus.Error(ErrClientSet)
		return nil
	}
	nsList, err := clients.CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		logrus.Error("查询NS列表失败", err)
		return nil
	}
	return nsList.Items
}

func (c *ClientInfo) ListSvc(ns string) []corev1.Service {
	clients, err := c.GetClient()
	if err != nil {
		logrus.Errorf(ErrClientSet)
		return nil
	}
	svcList, err := clients.CoreV1().Services(ns).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		logrus.Error("查询Service列表失败", err)
		return nil
	}
	return svcList.Items
}
func getMapKeys(labelMap map[string]string) []string {
	keys := make([]string, 0, len(labelMap))
	for k := range labelMap {
		keys = append(keys, k)
	}
	return keys
}
func IsSvcContainsLabels(service corev1.Service, labelMap map[string]string) (flag bool) {
	selectorKeys := getMapKeys(labelMap)
	serviceLabelKeys := getMapKeys(service.Spec.Selector)
	flag = true
	for _, key := range selectorKeys {
		if i := arrays.ContainsString(serviceLabelKeys, key); i < 0 || labelMap[key] != service.Spec.Selector[key] {
			flag = false
			break
		}
	}
	return
}
func (c *ClientInfo) ListSvcFromLabelSelector(ns string, labelMap map[string]string) []corev1.Service {
	var filterService []corev1.Service
	serviceList := c.ListSvc(ns)
	for _, service := range serviceList {
		if flag := IsSvcContainsLabels(service, labelMap); flag {
			filterService = append(filterService, service)
		}
	}
	return filterService
}

func (c *ClientInfo) GetSvcPorts(ns string, svc string) []corev1.ServicePort {
	clients, err := c.GetClient()
	if err != nil {
		logrus.Errorf(ErrClientSet)
		return nil
	}
	svcObj, err := clients.CoreV1().Services(ns).Get(context.Background(), svc, metav1.GetOptions{})
	if err != nil {
		logrus.Error("查询Service失败", err)
		return nil
	}
	return svcObj.Spec.Ports
}
func (c *ClientInfo) GetDeployFromSvc(ns string, svc string) []v1.Deployment {
	clients, err := c.GetClient()
	if err != nil {
		logrus.Errorf(ErrClientSet)
		return nil
	}
	svcObj, err := clients.CoreV1().Services(ns).Get(context.Background(), svc, metav1.GetOptions{})
	if err != nil {
		logrus.Error("查询Service失败", err)
		return nil
	}
	deploymentList, err := clients.AppsV1().Deployments(ns).List(context.Background(), metav1.ListOptions{LabelSelector: labels.SelectorFromSet(svcObj.Spec.Selector).String()})
	return deploymentList.Items
}

func (c *ClientInfo) ApplyYaml(filesOrPath []string) error {
	logrus.Trace("Starting Apply Yaml : ", filesOrPath)
	kubeConfigFlags := genericclioptions.NewConfigFlags(false).WithDeprecatedPasswordFlag()
	kubeConfigFlags.WrapConfigFn = func(*rest.Config) *rest.Config {
		return c.GetRestConfig()
	}
	builder := cliresource.NewBuilder(kubeConfigFlags)
	ioStreams := genericclioptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr}
	o := apply.NewApplyOptions(ioStreams)
	o.Builder = builder
	o.DeleteOptions = &delete.DeleteOptions{
		FilenameOptions: cliresource.FilenameOptions{
			// target k8s yaml files and directories that contain k8s yaml files
			Filenames: filesOrPath,
			Recursive: false,
		},
	}
	o.ToPrinter = func(operation string) (printers.ResourcePrinter, error) {
		o.PrintFlags.NamePrintFlags.Operation = operation
		return o.PrintFlags.ToPrinter()
	}
	return o.Run()
}
func (c *ClientInfo) GetConfigFlag() *genericclioptions.ConfigFlags {
	insecure := true
	return &genericclioptions.ConfigFlags{
		APIServer:   &c.Host,
		BearerToken: &c.Token,
		Insecure:    &insecure,
	}
}
func (c *ClientInfo) DeleteYaml(filesOrPath []string) error {
	logrus.Trace("Starting Delete Yaml : ", filesOrPath)
	ioStreams := genericclioptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr}
	configFlag := c.GetConfigFlag()
	f := cmdutil.NewFactory(configFlag)
	deleteCmd := delete.NewCmdDelete(f, ioStreams)

	//deleteCmd.Run(deleteCmd, filesOrPath)
	deleteOptions := &delete.DeleteOptions{
		FilenameOptions: cliresource.FilenameOptions{
			// target k8s yaml files and directories that contain k8s yaml files
			Filenames: filesOrPath,
			Recursive: false,
		},
		CascadingStrategy: metav1.DeletePropagationBackground,
		Quiet:             true,
	}

	deleteOptions.Complete(f, []string{}, deleteCmd)
	deleteOptions.Validate()
	err := deleteOptions.RunDelete(f)
	if err != nil {
		return err
	}
	return nil
}
func (c *ClientInfo) GetDeployment(ns string, name string) (*v1.Deployment, error) {
	clients, err := c.GetClient()
	if err != nil {
		logrus.Errorf(ErrClientSet)
		return nil, errors.New(ErrConfig)
	}
	return clients.AppsV1().Deployments(ns).Get(context.Background(), name, metav1.GetOptions{})
}
func (c *ClientInfo) GetService(ns string, name string) (*corev1.Service, error) {
	clients, err := c.GetClient()
	if err != nil {
		logrus.Errorf(ErrClientSet)
		return nil, errors.New(ErrConfig)
	}
	return clients.CoreV1().Services(ns).Get(context.Background(), name, metav1.GetOptions{})
}
func (c *ClientInfo) GetCrd(name string) (*unstructured.Unstructured, error) {
	dynamicClient, err := c.GetDynamic()
	if err != nil {
		logrus.Error(ErrClientSet)
		return nil, errors.New(ErrClientSet)
	}
	gvr := schema.GroupVersionResource{Group: "apiextensions.k8s.io", Version: "v1", Resource: "customresourcedefinitions"}
	return dynamicClient.Resource(gvr).Get(context.Background(), name, metav1.GetOptions{})
}
func (c *ClientInfo) GetCrd2(name string) (*unstructured.Unstructured, error) {
	clients, err := c.GetClient()
	if err != nil {
		logrus.Errorf(ErrClientSet)
		return nil, errors.New(ErrConfig)
	}
	data := &unstructured.Unstructured{}
	err = clients.RESTClient().Get().Resource(name).Do(context.Background()).Into(data)
	return data, err
}
func (c *ClientInfo) IsCrdExist(name string) (bool, error) {
	_, err := c.GetCrd(name)
	if err != nil && k8serrors.IsNotFound(err) {
		logrus.Debug("该CRD资源不存在")
		return false, nil
	} else if err != nil {
		logrus.Error("查找资源出错: ", err)
		return false, errors.New(fmt.Sprintf("查找资源出错: %s", err.Error()))
	}
	return true, nil
}
func (c *ClientInfo) GetDaemonSet(ns string, name string) (*v1.DaemonSet, error) {
	clients, err := c.GetClient()
	if err != nil {
		logrus.Errorf(ErrClientSet)
		return nil, errors.New(ErrConfig)
	}
	return clients.AppsV1().DaemonSets(ns).Get(context.Background(), name, metav1.GetOptions{})
}

func (c *ClientInfo) GetConfigMap(ns string, name string) (*corev1.ConfigMap, error) {
	clients, err := c.GetClient()
	if err != nil {
		logrus.Errorf(ErrClientSet)
		return nil, errors.New(ErrConfig)
	}
	return clients.CoreV1().ConfigMaps(ns).Get(context.Background(), name, metav1.GetOptions{})
}
func (c *ClientInfo) ApplyJsonData(jsonData string) error {
	logrus.Trace("Starting Apply Json data ")
	randNum := fmt.Sprintf("%06v", rand.New(rand.NewSource(time.Now().UnixNano())).Int31n(1000000))
	fileName := fmt.Sprintf("applyjson-%s", randNum)
	file, err := os.Create(fileName)
	if err != nil {
		logrus.Error(err.Error())
		return err
	}
	defer os.Remove(fileName)
	defer file.Close()
	file.WriteString(jsonData)
	return c.ApplyYaml([]string{fileName})
}
func (c *ClientInfo) DeleteJsonData(jsonData string) error {
	logrus.Trace("Starting Delete Json data ")
	randNum := fmt.Sprintf("%06v", rand.New(rand.NewSource(time.Now().UnixNano())).Int31n(1000000))
	fileName := fmt.Sprintf("deletejson-%s", randNum)
	file, err := os.Create(fileName)
	if err != nil {
		logrus.Error(err.Error())
		return err
	}
	defer os.Remove(fileName)
	defer file.Close()
	file.WriteString(jsonData)
	return c.DeleteYaml([]string{fileName})
}
func (c *ClientInfo) CheckNodeCRI() error {
	client, err := c.GetClient()
	if err != nil {
		logrus.Errorf(ErrClientSet)
		return errors.New("连接集群失败")
	}
	nodeList, err := client.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		logrus.Errorf("List node err %v", err)
		return errors.New("获取集群节点信息失败")
	}
	for _, node := range nodeList.Items {
		containerRuntime := node.Status.NodeInfo.ContainerRuntimeVersion
		url, nodeErr := url.Parse(containerRuntime)
		if nodeErr != nil {
			logrus.Errorf("Get container cri fail err %v", nodeErr)
			return errors.New("解析节点信息失败")
		}
		if url.Scheme != CONTAINERD {
			logrus.Warnf("There are non-Containerd nodes")
			return errors.New("集群存在非Containerd节点，不符合安装条件！")
		}
	}
	return nil
}

func (c *ClientInfo) CreateEnvoyfilter(envoyfilter *v1alpha3.EnvoyFilter) (*v1alpha3.EnvoyFilter, error) {
	clients, err := c.GetIstioClient()
	if err != nil {
		logrus.Errorf(ErrClientSet)
		return nil, errors.New(ErrConfig)
	}
	return clients.NetworkingV1alpha3().EnvoyFilters(envoyfilter.Namespace).Create(context.Background(), envoyfilter, metav1.CreateOptions{})
}
func (c *ClientInfo) UpdateEnvoyfilter(envoyfilter *v1alpha3.EnvoyFilter) (*v1alpha3.EnvoyFilter, error) {
	clients, err := c.GetIstioClient()
	if err != nil {
		logrus.Errorf(ErrClientSet)
		return nil, errors.New(ErrConfig)
	}
	return clients.NetworkingV1alpha3().EnvoyFilters(envoyfilter.Namespace).Update(context.Background(), envoyfilter, metav1.UpdateOptions{})
}
func (c *ClientInfo) ListEnvoyfilter(ns string) (*v1alpha3.EnvoyFilterList, error) {
	clients, err := c.GetIstioClient()
	if err != nil {
		logrus.Errorf(ErrClientSet)
		return nil, errors.New(ErrConfig)
	}
	return clients.NetworkingV1alpha3().EnvoyFilters(ns).List(context.Background(), metav1.ListOptions{})
}

func (c *ClientInfo) GetEnvoyfilter(ns string, name string) (*v1alpha3.EnvoyFilter, error) {
	clients, err := c.GetIstioClient()
	if err != nil {
		logrus.Errorf(ErrClientSet)
		return nil, errors.New(ErrConfig)
	}
	return clients.NetworkingV1alpha3().EnvoyFilters(ns).Get(context.Background(), name, metav1.GetOptions{})
}

func (c *ClientInfo) DeleteEnvoyfilter(ns string, name string) error {
	clients, err := c.GetIstioClient()
	if err != nil {
		logrus.Errorf(ErrClientSet)
		return errors.New(ErrConfig)
	}
	return clients.NetworkingV1alpha3().EnvoyFilters(ns).Delete(context.Background(), name, metav1.DeleteOptions{})
}

func (c *ClientInfo) UpdateService(ns string, svcObj *corev1.Service) error {
	clients, err := c.GetClient()
	if err != nil {
		logrus.Errorf(ErrClientSet)
		return errors.New(ErrConfig)
	}
	_, err = clients.CoreV1().Services(ns).Update(context.Background(), svcObj, metav1.UpdateOptions{})
	return err
}
func (c *ClientInfo) UpdateDeployment(ns string, deployObj *v1.Deployment) error {
	clients, err := c.GetClient()
	if err != nil {
		logrus.Errorf(ErrClientSet)
		return errors.New(ErrConfig)
	}
	_, err = clients.AppsV1().Deployments(ns).Update(context.Background(), deployObj, metav1.UpdateOptions{})
	return err
}
func (c *ClientInfo) RestartDeployment(ns string, deployName string) error {
	ioStreams := genericclioptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr}
	o := rollout.NewRolloutRestartOptions(ioStreams)
	matchVersionKubeConfigFlags := cmdutil.NewMatchVersionFlags(c.GetConfigFlag())
	f := cmdutil.NewFactory(matchVersionKubeConfigFlags)
	o.ToPrinter = func(operation string) (printers.ResourcePrinter, error) {
		o.PrintFlags.NamePrintFlags.Operation = operation
		return o.PrintFlags.ToPrinter()
	}

	o.Builder = f.NewBuilder
	o.Resources = []string{"deployment", deployName}
	o.Namespace = ns

	o.Restarter = polymorphichelpers.ObjectRestarterFn
	return o.RunRestart()
}
func (c *ClientInfo) GetSvcSelector(svc string, ns string) map[string]string {
	svcObj := c.GetSvc(svc, ns)
	if svcObj != nil {
		return svcObj.Spec.Selector
	}
	return nil
}
