package app

import (
	"fmt"
	"net/http"

	"k8s.io/klog/v2"
)

const (
	Success = "OK"
)

func StartHealthzServer(port int) {
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(Success))
	})

	klog.Infof("Starting healthz server, listening on port: %d", port)
	klog.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
}
