package server

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os/exec"
	"time"

	"github.com/adrielparedes/kogito-local-server/pkg/config"
	"github.com/adrielparedes/kogito-local-server/pkg/utils"
	"github.com/gorilla/mux"
)

type Proxy struct {
	srv *http.Server
	cmd *exec.Cmd
}

func (p *Proxy) Start() {

	var config config.Config
	conf := config.GetConfig()

	target, err := url.Parse("http://" + conf.Runner.IP + ":" + conf.Runner.Port)
	p.cmd = exec.Command("java", "-Dquarkus.http.port="+conf.Runner.Port, "-jar", utils.GetBaseDir()+"/"+conf.Runner.Location)
	stdout, _ := p.cmd.StdoutPipe()

	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			msg := scanner.Text()
			fmt.Printf("msg: %s\n", msg)
		}
	}()

	go startRunner(p.cmd)

	if err != nil {
		log.Fatal(err)
	}

	proxy := httputil.NewSingleHostReverseProxy(target)

	r := mux.NewRouter()
	r.HandleFunc("/ping", pingHandler)
	r.PathPrefix("/").HandlerFunc(proxyHandler(proxy, p.cmd))

	p.srv = &http.Server{
		Handler:      r,
		Addr:         conf.Proxy.IP + ":" + conf.Proxy.Port,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	go p.srv.ListenAndServe()
}

func (p *Proxy) Stop() {
	log.Println("Shutting down")

	stopRunner(p.cmd)

	ctx, cancel := context.WithTimeout(context.TODO(), time.Second*15)
	defer cancel()

	if err := p.srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server Shutdown Failed:%+v", err)
	}
	log.Println("Shutdown complete")
}

func proxyHandler(proxy *httputil.ReverseProxy, cmd *exec.Cmd) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Host = r.URL.Host
		proxy.ServeHTTP(w, r)
	}
}

func pingHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	result := map[string]string{"status": "ok"}
	json, _ := json.Marshal(result)
	w.Write(json)
}

func startRunner(cmd *exec.Cmd) {
	if err := cmd.Start(); err != nil {
		log.Fatal(err)
	}
}

func stopRunner(cmd *exec.Cmd) {
	cmd.Process.Kill()
}
