// Copyright 2017 Google Inc. All Rights Reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.sc-corp.net/scaddlive/hello-universe/pkg/kargo"
)

var (
	hostname string
	region   string
	httpAddr string
)

func main() {

	// pase args
	flag.StringVar(&httpAddr, "http", "127.0.0.1:8080", "HTTP service address")
	flag.Parse()

	var err error
	// get hostname
	hostname, err = os.Hostname()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	region = os.Getenv("REGION")

	fmt.Println("Starting hello-universe...")
	errChan := make(chan error, 10)

	var dm *kargo.DeploymentManager
	if kargo.EnableKubernetes {
		// this means we have a kubernetes deployment

		// first upload to build and upload to gcs
		link, err := kargo.Upload(kargo.UploadConfig{
			ProjectID:  "staging-glass-pen-358",
			BucketName: "test-binaries",
			ObjectName: "hello-universe",
		})

		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		// create a new deployment
		dm = kargo.New()
		err = dm.Create(kargo.DeploymentConfig{
			Args:      []string{"-http=0.0.0.0:8080"},
			Name:      "hello-universe",
			BinaryURL: link,
		})
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		// stream logs
		err = dm.Logs(os.Stdout)
		if err != nil {
			fmt.Println("Local logging has been disabled.")
		}
	} else {
		// if we're running locally then we just want to run
		http.HandleFunc("/", httpHandler)

		go func() {
			errChan <- http.ListenAndServe(httpAddr, nil)
		}()
	}

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	for {
		select {
		case err := <-errChan:
			if err != nil {
				fmt.Printf("%s - %s\n", hostname, err)
				os.Exit(1)
			}
		case <-signalChan:
			fmt.Printf("%s - Shutdown signal received, exiting...\n", hostname)
			if kargo.EnableKubernetes {
				err := dm.Delete()
				if err != nil {
					fmt.Printf("%s - %s\n", hostname, err)
					os.Exit(1)
				}
			}
			os.Exit(0)
		}
	}
}
