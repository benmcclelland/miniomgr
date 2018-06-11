package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/benmcclelland/miniomgr/backend"
)

type Redirector struct {
	be *backend.MinioMgr
}

func logRequest(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s %s\n", r.RemoteAddr, r.Method, r.URL)
		handler.ServeHTTP(w, r)
	})
}

func (rd *Redirector) handle(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 2 {
		http.Error(w,
			"could not parse url path", http.StatusInternalServerError)
		return
	}

	user := parts[1]
	port, err := rd.be.GetUserURL(user)
	if err != nil {
		http.Error(w,
			fmt.Sprintf("GetUserURL %q %q: %v", r.URL.Path, user, err),
			http.StatusInternalServerError)
		return
	}

	hoststr := r.Host
	if hoststr == "" && r.URL != nil {
		hoststr = r.URL.Host
	}

	parts = strings.Split(hoststr, ":")
	if len(parts) < 1 {
		http.Error(w,
			"could not parse host string", http.StatusInternalServerError)
		return
	}

	host := parts[0]

	log.Println("redirecting to", host, port)

	w.Header().Set("Cache-Control", "no-store")

	http.Redirect(w, r, fmt.Sprintf("http://%v:%v/", host, port), 302)
}

func New(fspath string) (*Redirector, error) {
	b, err := backend.New(fspath)
	if err != nil {
		return nil, err
	}
	return &Redirector{be: b}, nil
}

func (r *Redirector) Done() {
	r.be.Done()
}

func singleJoiningSlash(a, b string) string {
	aslash := strings.HasSuffix(a, "/")
	bslash := strings.HasPrefix(b, "/")
	switch {
	case aslash && bslash:
		return a + b[1:]
	case !aslash && !bslash:
		return a + "/" + b
	}
	return a + b
}
