package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/joho/godotenv"

	"github.com/gorilla/mux"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
)

type Command struct {
	Command string `json:"command"`
}

type OutputMessage struct {
	MessageType string `json:"type"`
	Message     string `json:"message"`
}

func main() {

	kubeconfig := flag.String("kubeconfig", "C:\\Users\\WSO2\\.kube\\config", "location to your cube config files")
	config, err1 := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err1 != nil {
		fmt.Println("Error in creating config")
	}

	clientset, err2 := kubernetes.NewForConfig(config)
	if err2 != nil {
		fmt.Println("Error in creating clientset")
	}
	router := mux.NewRouter().StrictSlash(true)

	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Welcome to the command executor!\n")

	})

	router.HandleFunc("/execute", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		client := &http.Client{}
		ctx := context.Background()
		runner_req, err := http.NewRequestWithContext(ctx, "GET", "https://api.github.com/repos/Sribarathvajasarma/ARCproject03/actions/runners", nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error creating HTTP request: %v\n", err)
			os.Exit(1)
		}

		envErr := godotenv.Load(".env")
		if envErr != nil {
			fmt.Printf("Could not load .env file")
			os.Exit(1)
		}

		token := os.Getenv("GITHUB_PAT")
		runner_req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))
		runner_req.Header.Add("Accept", "application/vnd.github.v3+json")

		resp, err := client.Do(runner_req)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error sending HTTP request: %v\n", err)
			os.Exit(1)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			fmt.Fprintf(os.Stderr, "error response from GitHub API: %s\n", resp.Status)
			os.Exit(1)
		}

		var data map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&data)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error parsing response JSON: %v\n", err)
			os.Exit(1)
		}

		var runnerName string
		if runners, ok := data["runners"].([]interface{}); ok && len(runners) > 0 {
			runner := runners[0].(map[string]interface{})
			runner_to_string := runner["name"].(string)
			runnerName = runner_to_string

		} else {
			fmt.Fprintf(os.Stderr, "error: runners not found in response\n")
			os.Exit(1)
		}

		podName := runnerName
		// json.NewEncoder(w).Encode(runnerName)

		containerName := "runner"

		command := []string{"ls", "runner/_diag/pages"}

		req := clientset.CoreV1().RESTClient().Post().
			Resource("pods").
			Name(podName).
			Namespace("default").
			SubResource("exec").
			VersionedParams(&corev1.PodExecOptions{
				Container: containerName,
				Command:   command,
				Stdin:     false,
				Stdout:    true,
				Stderr:    true,
				TTY:       false,
			}, scheme.ParameterCodec)

		executor, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
		if err != nil {
			panic(err)
		}

		var stdout, stderr bytes.Buffer

		err = executor.Stream(remotecommand.StreamOptions{
			Stdout: &stdout,
			Stderr: &stderr,
			Tty:    false,
		})
		if err != nil {
			panic(err)
		}

		output := stdout.String()
		// json.NewEncoder(w).Encode(output)

		lines := strings.Split(output, "\n")

		if len(lines) > 0 {
			firstLine := lines[0]
			command := []string{"cat", "runner/_diag/pages/" + firstLine}

			req := clientset.CoreV1().RESTClient().Post().
				Resource("pods").
				Name(podName).
				Namespace("default").
				SubResource("exec").
				VersionedParams(&corev1.PodExecOptions{
					Container: containerName,
					Command:   command,
					Stdin:     false,
					Stdout:    true,
					Stderr:    true,
					TTY:       false,
				}, scheme.ParameterCodec)

			executor, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
			if err != nil {
				panic(err)
			}

			var stdout, stderr bytes.Buffer

			err = executor.Stream(remotecommand.StreamOptions{
				Stdout: &stdout,
				Stderr: &stderr,
				Tty:    false,
			})
			if err != nil {
				panic(err)
			}

			output := stdout.String()
			//json.NewEncoder(w).Encode(output)
			w.Write([]byte(output))

		}
	}).Methods("GET")

	log.Fatal(http.ListenAndServe(":8080", router))

}
