package main

import (
	helmclient "github.com/mittwald/go-helm-client"
	"helm.sh/helm/v3/pkg/repo"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"log"
	"os"
	"time"
)

var kubeClient *kubernetes.Clientset
var helmClient helmclient.Client

const secretLabel = "argocd.argoproj.io/secret-type=repository"

func main() {
	initK8sClient()
	initHelmClient()
	go watch()
	log.Println("Ready")
	select {}
}

func watch() {
	labelOptions := informers.WithTweakListOptions(func(opts *metav1.ListOptions) {
		opts.LabelSelector = secretLabel
	})

	factory := informers.NewSharedInformerFactoryWithOptions(kubeClient, 1*time.Second, informers.WithNamespace(os.Getenv("MY_POD_NAMESPACE")), labelOptions)
	informer := factory.Core().V1().Secrets().Informer()
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    secretAdded,
		UpdateFunc: secretUpdated,
		DeleteFunc: secretDeleted,
	})
	factory.Start(wait.NeverStop)
	factory.WaitForCacheSync(wait.NeverStop)
}

func secretAdded(obj interface{}) {
	secretObj, ok := obj.(*v1.Secret)
	if ok && secretObj != nil && string(secretObj.Data["type"]) == "helm" {
		log.Printf("ArgoCD helm repository secret %s detected", secretObj.Name)
		if err := addOrUpdateChartRepo(secretObj); err != nil {
			log.Printf("Add helm repo failure.")
			log.Printf(err.Error())
		}
	}
}

func secretUpdated(oldObj, newObj interface{}) {
	if oldObj == newObj {
		return
	}

	secretObj, ok := newObj.(*v1.Secret)
	if ok && secretObj != nil && string(secretObj.Data["type"]) == "helm" {
		log.Printf("ArgoCD repository secret %s change detected", secretObj.Name)
		if err := addOrUpdateChartRepo(secretObj); err != nil {
			log.Printf("Update helm repo failure.")
			log.Printf(err.Error())
		}
	}
}

func secretDeleted(obj interface{}) {}

func initK8sClient() {
	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err)
	}
	kubeClient, err = kubernetes.NewForConfig(config)
	if err != nil {
		panic(err)
	}
}

func initHelmClient() {
	opt := &helmclient.Options{
		RepositoryCache:  os.Getenv("HELM_CACHE_HOME"),
		RepositoryConfig: os.Getenv("HELM_CONFIG_HOME") + "/repositories.yaml",
		Debug:            true,
		Linting:          true,
		DebugLog:         func(format string, v ...interface{}) {},
	}
	var err error
	helmClient, err = helmclient.New(opt)
	if err != nil {
		panic(err)
	}
}

func addOrUpdateChartRepo(secretObj *v1.Secret) error {

	chartRepo := repo.Entry{
		Name:     string(secretObj.Data["name"]),
		URL:      string(secretObj.Data["url"]),
		Username: string(secretObj.Data["username"]),
		Password: string(secretObj.Data["password"]),
		// Since helm 3.6.1 it is necessary to pass 'PassCredentialsAll = true'.
		PassCredentialsAll:    true,
		InsecureSkipTLSverify: true,
	}
	log.Printf("Adding/Updating helm repo %s", string(secretObj.Data["name"]))
	return helmClient.AddOrUpdateChartRepo(chartRepo)
}
