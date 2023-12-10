package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net"

	"net/http"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/gorilla/mux"
)

var config = make(map[string]string)

type Search struct {
	Page    string
	Preview string
	Pattern string
}

func downloadHandler(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path[len("/dl/"):]
	log.Printf("[%s] DOWNLOAD [%s]: %s \n", r.RemoteAddr, r.Method, r.URL)
	http.ServeFile(w, r, config["files"]+path)
}

func listAttachments() ([]string, error) {
	var fileList []string

	files, err := os.ReadDir(config["files"])
	if err != nil {
		return nil, err
	}
	for _, file := range files {
		if !file.IsDir() {
			fileList = append(fileList, file.Name())
		}

	}
	return fileList, nil
}

func listPages() ([]string, error) {
	var fileList []string

	files, err := os.ReadDir(config["pages"])
	if err != nil {
		return nil, err
	}
	for _, file := range files {
		if !file.IsDir() {
			ext := filepath.Ext(file.Name())
			fileNameWithoutExt := file.Name()[0 : len(file.Name())-len(ext)]
			fileList = append(fileList, fileNameWithoutExt)
		}
	}
	return fileList, nil
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("[%s] UPLOAD [%s]: %s \n", r.RemoteAddr, r.Method, r.URL)

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	content, err := io.ReadAll(r.Body)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	name := r.URL.Path[len("/dl/"):]
	p := &Attachment{Filename: name, Content: content}
	err = p.save()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	resp := "upload " + p.Filename + " ok\n"
	w.Write([]byte(resp))
}

func getIp() *string {

	interfaces, err := net.Interfaces()
	if err != nil {
		log.Println("Error nic ", err)
	} else {
		for _, iface := range interfaces {
			if (iface.Flags&net.FlagUp) != 0 && (iface.Flags&net.FlagLoopback) == 0 {
				addrs, err := iface.Addrs()
				if err != nil {
					log.Println("Error addr:", err)
					continue
				}
				for _, addr := range addrs {
					if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
						if ipnet.IP.To4() != nil {
							res := ipnet.IP.String()
							return &res
						}
					}
				}
			}
		}
	}
	res := "127.0.0.1"
	return &res
}

func searchHandler(w http.ResponseWriter, r *http.Request) {
	query := strings.ToLower(mux.Vars(r)["query"])

	files, err := os.ReadDir(config["pages"])
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var results []Search
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		content, err := os.ReadFile(config["pages"] + file.Name())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		lowerContent := strings.ToLower(string(content))
		index := strings.Index(lowerContent, query)
		if index != -1 {
			start := max(0, index-50)
			end := min(len(lowerContent), index+len(query)+50)
			preview := lowerContent[start:end]
			ext := filepath.Ext(file.Name())
			fileNameWithoutExt := file.Name()[0 : len(file.Name())-len(ext)]
			results = append(results, Search{Page: fileNameWithoutExt, Preview: preview, Pattern: query})
		}
	}

	p, _ := listPages()
	a, _ := listAttachments()
	tr := TemplateRender{Pages: p, Attachments: a, Data: results, Content: query}

	t, err := template.ParseFS(tpls, "templates/base.html", "templates/search.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := t.ExecuteTemplate(w, "base", tr); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func makeAddr(ip *string, port string) string {
	if strings.HasPrefix(port, ":") {
		port = "127.0.0.1" + port
	} else {
		return port
	}

	_, portStr, err := net.SplitHostPort(port)
	if err != nil {
		return "127.0.0.1:0"
	}
	retAddr := fmt.Sprintf("%s:%s", *ip, portStr)
	return retAddr
}

func docHandler(w http.ResponseWriter, r *http.Request) {

	p, _ := listPages()
	a, _ := listAttachments()

	t, err := template.ParseFS(tpls, "templates/base.html", "templates/doc.html")

	addr := makeAddr(getIp(), config["port"])

	tr := TemplateRender{Pages: p, Attachments: a, Data: addr}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := t.ExecuteTemplate(w, "base", tr); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func deleteHandler(w http.ResponseWriter, r *http.Request) {
	title := r.URL.Path[len("/del/"):]
	filename := "./pages/" + filepath.Clean(title) + ".md"
	err := os.Remove(filename)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/", http.StatusFound)
}

func checkDir(dir string) {
	_, err := os.Stat(dir)
	if os.IsNotExist(err) {
		err := os.Mkdir(dir, os.ModePerm)
		if err != nil {
			log.Fatalf("unable to create %s", dir)
			os.Exit(-1)
		}
	} else if err != nil {
		log.Fatalf("error on %s : %s", dir, err)
	}
}

func infoHandler(w http.ResponseWriter, r *http.Request) {
	headers := r.Header
	ip := r.RemoteAddr
	data := ""
	for name, values := range headers {
		data += name + ": " + strings.Join(values, ", ") + "\n"
	}
	data += "\nIP: " + ip + "\n"
	w.Write([]byte(data))
}

func BasicAuthHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		elems := strings.SplitN(config["auth"], ":", 2)
		usr, pwd, ok := r.BasicAuth()

		if !ok || usr != elems[0] || pwd != elems[1] {
			w.Header().Set("WWW-Authenticate", `Basic realm="auth required"`)
			http.Error(w, "Unauthorized.", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func main() {

	listen := flag.String("listen", ":8800", "listen addr ")
	auth := flag.String("auth", "", "user:pass")
	flag.Parse()
	if *auth != "" {
		config["auth"] = *auth
	}
	config["gorm"] = "gorm.db"
	SetupGorm()

	config["pages"] = "./pages/"
	config["files"] = "./files/"

	checkDir(config["pages"])
	checkDir(config["files"])

	config["port"] = *listen

	router := mux.NewRouter()

	if _, ok := config["auth"]; ok {
		router.Use(BasicAuthHandler)
	}

	router.HandleFunc("/", indexHandler)
	router.HandleFunc("/doc", docHandler)
	router.HandleFunc("/info", infoHandler)

	router.HandleFunc("/view/{page}", viewHandler)
	router.HandleFunc("/edit/{page}", editHandler)
	router.HandleFunc("/save/{page}", saveHandler)
	router.HandleFunc("/del/{page}", deleteHandler)

	router.HandleFunc("/search/{query}", searchHandler)

	router.HandleFunc("/dl/{file}", downloadHandler)
	router.HandleFunc("/up/{file}", uploadHandler)
	router.HandleFunc("/backup", BackupHandler)

	router.PathPrefix("/fonts/").Handler(http.StripPrefix("/fonts", hs))
	router.PathPrefix("/js/").Handler(http.StripPrefix("/js", hs))
	router.PathPrefix("/css/").Handler(http.StripPrefix("/css", hs))

	router.HandleFunc("/nmap", NmapHandler)
	router.PathPrefix("/nmap/").Handler(http.StripPrefix("/nmap", NmapRouter()))
	router.PathPrefix("/dav").Handler(WebdavHandler())

	log.Printf("started on %s", config["port"])

	err := http.ListenAndServe(config["port"], router)
	if err != nil {
		log.Printf("ERROR  %v", err)
	}
}
