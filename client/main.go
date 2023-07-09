package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"path/filepath"
	"time"

	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	retry "github.com/avast/retry-go"
)

var namespace string = "default"

// GetClient returns a kubernetes client
func GetK8sClient(configpath string) (*kubernetes.Clientset, error) {
	if configpath == "" {
		logrus.Info("Using Incluster configuration")
		config, err := rest.InClusterConfig()
		if err != nil {
			logrus.Fatalf("Error occured while reading incluster kubeconfig:%v", err)
			return nil, err
		}
		return kubernetes.NewForConfig(config)
	}

	logrus.Infof("Using configuration file:%s", configpath)
	config, err := clientcmd.BuildConfigFromFlags("", configpath)
	if err != nil {
		logrus.Fatalf("Error occured while reading kubeconfig:%v", err)
		return nil, err
	}
	return kubernetes.NewForConfig(config)
}

func createDeployment(clientset *kubernetes.Clientset) error {
	deploymentClient := clientset.AppsV1().Deployments(namespace)
	var podCount int32 = 2

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "demo-deployment",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &podCount,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "demo",
				},
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "demo",
					},
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:  "web",
							Image: "ealen/echo-server",
							Ports: []v1.ContainerPort{
								{
									Name:          "http",
									Protocol:      v1.ProtocolTCP,
									ContainerPort: 80,
								},
							},
						},
					},
				},
			},
		},
	}
	_, e := deploymentClient.Create(context.TODO(), deployment, metav1.CreateOptions{})
	if e != nil {
		log.Fatalln("Failed to create deployment.", e)
	}
	log.Println("Created K8s job successfully")
	return e
}

func deleteDeployment(clientset *kubernetes.Clientset, dName string) error {
	deploymentClient := clientset.AppsV1().Deployments(namespace)
	e := deploymentClient.Delete(context.TODO(), dName, *&metav1.DeleteOptions{})
	if e != nil {
		log.Fatal("Error in deleting the deployment", e)
	}
	return e
}

func createService(clientset *kubernetes.Clientset) error {
	serviceClient := clientset.CoreV1().Services(namespace)
	service := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo-service",
			Namespace: "default",
			Labels: map[string]string{
				"app": "demo",
			},
		},
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{
				v1.ServicePort{
					Name:       "http",
					Port:       80,
					TargetPort: intstr.IntOrString{IntVal: 80},
					NodePort:   31080,
				},
			},
			Selector: map[string]string{
				"app": "demo",
			},
			Type: v1.ServiceTypeNodePort,
		},
	}

	_, e := serviceClient.Create(context.TODO(), service, metav1.CreateOptions{})
	if e != nil {
		log.Fatalln("Failed to create deployment.", e)
	}
	log.Println("Created K8s job successfully")
	return e
}

func deleteService(clientset *kubernetes.Clientset, svcName string) error {
	serviceClient := clientset.CoreV1().Services(namespace)
	e := serviceClient.Delete(context.TODO(), svcName, *&metav1.DeleteOptions{})
	if e != nil {
		log.Fatal("Error in deleting service")
	}
	return e
}

func sendHttpTraffic(url string) error {
	c := http.Client{Timeout: time.Duration(1) * time.Second}
	resp, err := c.Get(url)
	if err != nil {
		fmt.Printf("Error %s", err)
		return err
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("Failed to get 200 OK")
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	fmt.Printf("Body : %s", body)
	return nil
}

func main() {
	fmt.Println("Starting client")
	kubeconfig := flag.String("kubeconfig", filepath.Join("/Users/shubhashree/", ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	k8sClient, err := GetK8sClient(*kubeconfig)
	if err != nil {
		log.Fatal("Failed to create k8s client")
	}
	createDeployment(k8sClient)
	createService(k8sClient)

	url := "http://localhost:31080"
	retryCount := 0
	trafficErr := retry.Do(
		func() error {
			retryCount++
			err := sendHttpTraffic(url)
			if err != nil {
				fmt.Println("Error in sending traffic %v and retry attempt %d", err, retryCount)
				return err
			}
			return nil
		},
	)

	if trafficErr != nil {
		fmt.Println("Failed to get any traffic after retries ", trafficErr)
	}

	// Clean up the deployment and service
	deleteDeployment(k8sClient, "demo-deployment")
	deleteService(k8sClient, "demo-service")
}
