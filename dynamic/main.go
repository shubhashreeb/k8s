package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

// GetClient returns a kubernetes client
func GetK8sClient(name string) (*kubernetes.Clientset, error) {
	configpath := flag.String("kubeconfig", filepath.Join("/Users/shubhashree/", ".kube", name), "(optional) absolute path to the kubeconfig file")
	//configpath := "/Users/shubhashree/.kube/config.cluster1"
	config, err := clientcmd.BuildConfigFromFlags("", *configpath)
	if err != nil {
		logrus.Fatalf("Error occured while reading kubeconfig:%v", err)
		return nil, err
	}

	return kubernetes.NewForConfig(config)
}

func main() {
	// kubeConfig := os.Getenv("KUBECONFIG")

	var clusterConfig *rest.Config
	var err error
	configpath := flag.String("kubeconfig", filepath.Join("/Users/shubhashree/", ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	//configpath := "/Users/shubhashree/.kube/config.cluster1"
	clusterConfig, err = clientcmd.BuildConfigFromFlags("", *configpath)
	if err != nil {
		logrus.Fatalf("Error occured while reading kubeconfig:%v", err)
		return
	}

	clusterClient, err := dynamic.NewForConfig(clusterConfig)
	if err != nil {
		log.Fatalln(err)
	}

	//APIResources: []metav1.APIResource{{Name: "pods", SingularName: "pod"}},

	// resource := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
	resource := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}

	factory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(clusterClient, time.Minute, corev1.NamespaceAll, nil)
	informer := factory.ForResource(resource).Informer()

	mux := &sync.RWMutex{}
	synced := false
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			mux.RLock()
			defer mux.RUnlock()
			if !synced {
				return
			}
			fmt.Println("Add event for ", obj)
			// Handler logic
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			mux.RLock()
			defer mux.RUnlock()
			if !synced {
				return
			}
			fmt.Println("Update event for ", newObj)
			// Handler logic
		},
		DeleteFunc: func(obj interface{}) {
			mux.RLock()
			defer mux.RUnlock()
			if !synced {
				return
			}
			fmt.Println("Delete event for ", obj)
			// Handler logic
		},
	})

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	go informer.Run(ctx.Done())

	isSynced := cache.WaitForCacheSync(ctx.Done(), informer.HasSynced)
	mux.Lock()
	synced = isSynced
	mux.Unlock()

	if !isSynced {
		log.Fatal("failed to sync")
	}

	<-ctx.Done()
}
