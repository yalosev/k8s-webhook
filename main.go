package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {

	ns := os.Getenv("NAMESPACE")
	if ns == "" {
		ns = "default"
	}

	// create kubernetes in-cluster client
	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}

	//var kubeconfig *string
	//if home := homedir.HomeDir(); home != "" {
	//	kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	//} else {
	//	kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	//}
	//flag.Parse()
	//
	//config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	//if err != nil {
	//	panic(err)
	//}

	// creates the clientset
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "ok")
	})
	mux.HandleFunc("/webhook/{name}", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		name := r.PathValue("name")
		// read the request body

		var bodyMap map[string]interface{}
		err = json.NewDecoder(r.Body).Decode(&bodyMap)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		fmt.Println("Request body", bodyMap)

		web := &unstructured.Unstructured{Object: map[string]interface{}{
			"apiVersion": "test.deckhouse.io/v1alpha1",
			"kind":       "WebhookRequest",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": ns,
			},
			"body": bodyMap,
		}}

		fmt.Printf("Created webhook request: %s/%s\n", ns, name)

		_, err = dynamicClient.Resource(schema.GroupVersionResource{
			Group:    "test.deckhouse.io",
			Version:  "v1alpha1",
			Resource: "webhookrequests",
		}).Namespace(ns).Create(context.TODO(), web, v1.CreateOptions{})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})

	server := &http.Server{
		Addr:         ":8080",
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func() {
		if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("HTTP server error: %v", err)
		}
		log.Println("Stopped serving new connections.")
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	shutdownCtx, shutdownRelease := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownRelease()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("HTTP shutdown error: %v", err)
	}
	log.Println("Graceful shutdown complete.")
}
