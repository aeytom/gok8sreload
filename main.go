package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	batchv1 "k8s.io/api/batch/v1"

	apiv1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	//
	// Uncomment to load all auth plugins
	// _ "k8s.io/client-go/plugin/pkg/client/auth"
	//
	// Or uncomment to load specific auth plugins
	// _ "k8s.io/client-go/plugin/pkg/client/auth/azure"
	// _ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	// _ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
	// _ "k8s.io/client-go/plugin/pkg/client/auth/openstack"
)

var (
	kubeconfig *string
	namespace  *string
)

func init() {
	if home := homeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	namespace = flag.String("namespace", "default", "Namespace containing varnish stack")
	flag.Parse()
}

func main() {
	var (
		config *rest.Config
		err    error
	)
	if *kubeconfig != "" {
		// use the current context in kubeconfig
		config, err = clientcmd.BuildConfigFromFlags("", *kubeconfig)
		if err != nil {
			panic(err.Error())
		}
	} else {
		// creates the in-cluster config
		config, err = rest.InClusterConfig()
		if err != nil {
			panic(err.Error())
		}
	}

	// create the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	if svcwatch, err := clientset.CoreV1().Services("").Watch(metav1.ListOptions{}); err != nil {
		panic(err.Error())
	} else {
		go func() {
			for event := range svcwatch.ResultChan() {
				p, ok := event.Object.(*v1.Service)
				if !ok {
					log.Fatal("unexpected type")
				}
				fmt.Printf("service %10s %-20s %s\n", event.Type, p.Namespace, p.Name)
				fmt.Println(p.Spec.Ports)

				spanJob(clientset)
			}
		}()
	}

	for {
		pods, err := clientset.CoreV1().Pods(*namespace).List(metav1.ListOptions{})
		if err != nil {
			panic(err.Error())
		}
		fmt.Printf("There are %d pods in the cluster\n", len(pods.Items))
		// for idx, pod := range pods.Items {
		// 	fmt.Printf("… %4d %-20s %s\n", idx, pod.Namespace, pod.Name)
		// }

		// Examples for error handling:
		// - Use helper functions like e.g. errors.IsNotFound()
		// - And/or cast to StatusError and use its properties like e.g. ErrStatus.Message
		pod := "example-xxxxx"
		_, err = clientset.CoreV1().Pods(*namespace).Get(pod, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			fmt.Printf("Pod %s in namespace %s not found\n", pod, *namespace)
		} else if statusError, isStatus := err.(*errors.StatusError); isStatus {
			fmt.Printf("Error getting pod %s in namespace %s: %v\n",
				pod, *namespace, statusError.ErrStatus.Message)
		} else if err != nil {
			panic(err.Error())
		} else {
			fmt.Printf("Found pod %s in namespace %s\n", pod, *namespace)
		}

		time.Sleep(10 * time.Second)
	}
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}

func spanJob(clientset *kubernetes.Clientset) {
	jobsClient := clientset.BatchV1().Jobs(v1.NamespaceDefault)
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "prometheus-webhook-",
			Namespace:    v1.NamespaceDefault,
		},
		Spec: batchv1.JobSpec{
			TTLSecondsAfterFinished: int32Ptr(60),
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "prometheus-webhook-",
				},
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						{
							Name:  "ansible-job",
							Image: "yourimage",
						},
					},
					RestartPolicy: apiv1.RestartPolicyNever,
				},
			},
		},
	}
	result, err := jobsClient.Create(job)
	if err != nil {
		panic(err)
	}
	fmt.Println(result)
}

func int32Ptr(i int32) *int32 { return &i }
