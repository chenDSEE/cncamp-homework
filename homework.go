package main

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"sync/atomic"

	log "github.com/sirupsen/logrus"
)

/**
 * use case:
 * 1. get system VERSION
 *    curl http://192.168.8.130:80/version
 *
 * 2. get server healthz
 *    curl http://192.168.8.130:80/healthz
 *
 * 3. log level change
 *    curl http://192.168.8.130:80/log?level=panic
 *    curl http://192.168.8.130:80/log?level=fatal
 *    curl http://192.168.8.130:80/log?level=error
 *    curl http://192.168.8.130:80/log?level=warning
 *    curl http://192.168.8.130:80/log?level=info
 *    curl http://192.168.8.130:80/log?level=debug
 *    curl http://192.168.8.130:80/log?level=trace
 */

func init() {
	log.SetOutput(os.Stdout)
	log.SetLevel(log.DebugLevel)
}

const ipAddr string = ":80"

var reqCounter int32 = 0

func main() {
	http.HandleFunc("/version", versionHandler)
	http.HandleFunc("/log", logHandler)
	http.HandleFunc("/healthz", healthzHandler)

	log.Warning("========= http server start ========")
	http.ListenAndServe(ipAddr, nil)
	log.Warning("========= http server stop ========")

}

func versionHandler(resp http.ResponseWriter, req *http.Request) {
	log.Warningf("client[%s] %s %s", req.RemoteAddr, req.Method, req.URL.Path)
	atomic.AddInt32(&reqCounter, 1)

	for h, v := range req.Header {
		for _, subValue := range v {
			log.Debugf("Header[%s]:[%s]", h, subValue)
			resp.Header().Add(h, subValue)
		}
	}

	resp.WriteHeader(http.StatusOK)
	buf := bytes.Buffer{}
	buf.Write([]byte("VERSION="))
	buf.Write([]byte(runtime.Version()))
	buf.Write([]byte("\nAIMERNAME="))
	buf.Write([]byte(os.Getenv("AIMERNAME")))
	resp.Write(buf.Bytes())
}

func logHandler(resp http.ResponseWriter, req *http.Request) {
	atomic.AddInt32(&reqCounter, 1)

	input := req.URL.Query().Get("level")
	log.Warningf("log level change from [%s] to [%s]", log.GetLevel().String(), input)
	level, err := log.ParseLevel(input)
	if err != nil {
		log.Warning(err)
	}

	log.SetLevel(level)
}

func healthzHandler(resp http.ResponseWriter, req *http.Request) {
	buf := fmt.Sprintf("total request count:[%d]\n", atomic.LoadInt32(&reqCounter))
	resp.Write([]byte(buf))
}
