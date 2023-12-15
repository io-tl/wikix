package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"path/filepath"
	"text/template"

	"github.com/gorilla/mux"
	"golang.org/x/net/webdav"
)

var (
	//go:embed templates
	tpls embed.FS

	//go:embed static
	statics embed.FS
	html, _ = fs.Sub(statics, "static")
	hs      = http.FileServer(http.FS(html))
)

func logDavDebug(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Debug : %s %s", r.Method, r.RequestURI)
		next.ServeHTTP(w, r)
	})
}

func addCORSDav(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Allow-Methods", "ACL, CANCELUPLOAD, CHECKIN, CHECKOUT, COPY, DELETE, GET, HEAD, LOCK, MKCALENDAR, MKCOL, MOVE, OPTIONS, POST, PROPFIND, PROPPATCH, PUT, REPORT, SEARCH, UNCHECKOUT, UNLOCK, UPDATE, VERSION-CONTROL")
		w.Header().Set("Access-Control-Allow-Headers", "Overwrite, Destination, Content-Type, Depth, User-Agent, Translate, Range, Content-Range, Timeout, X-File-Size, X-Requested-With, If-Modified-Since, X-File-Name, Cache-Control, Location, Lock-Token, If")
		w.Header().Set("Access-Control-Expose-Headers", "DAV, Content-length, Allow")
		next.ServeHTTP(w, r)
	})
}

func WebdavHandler() http.Handler {

	davRouter := mux.NewRouter()
	//davRouter.Use(logDavDebug)
	davRouter.Use(addCORSDav)
	pagesFS := &webdav.Handler{
		Prefix:     "/dav/pages",
		FileSystem: webdav.Dir(filepath.Join(config["pages"])),
		LockSystem: webdav.NewMemLS(),
		Logger: func(r *http.Request, err error) {
			if err != nil {
				log.Printf("[%s] DAV 404 pages [%s]: %s, ERROR: %s\n", r.RemoteAddr, r.Method, r.URL, err)
			} else {
				log.Printf("[%s] DAV 200 pages [%s]: %s \n", r.RemoteAddr, r.Method, r.URL)
			}
		},
	}

	attachmentsFS := &webdav.Handler{
		Prefix:     "/dav/files",
		FileSystem: webdav.Dir(filepath.Join(config["files"])),
		LockSystem: webdav.NewMemLS(),
		Logger: func(r *http.Request, err error) {
			if err != nil {
				log.Printf("[%s] DAV 404 files [%s]: %s, ERROR: %s\n", r.RemoteAddr, r.Method, r.URL, err)
			} else {
				log.Printf("[%s] DAV 200 files [%s]: %s \n", r.RemoteAddr, r.Method, r.URL)
			}
		},
	}
	davRouter.PathPrefix("/dav/pages").Handler(pagesFS)
	davRouter.PathPrefix("/dav/pages/").Handler(pagesFS)
	davRouter.PathPrefix("/dav/files").Handler(attachmentsFS)
	davRouter.PathPrefix("/dav/files/").Handler(attachmentsFS)

	davRouter.PathPrefix("/dav").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" || r.Method == "HEAD" {

			t, err := template.ParseFS(tpls, "templates/dav.html")
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			if err := t.ExecuteTemplate(w, "base", nil); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		}
	})

	return davRouter
}
