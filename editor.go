package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/microcosm-cc/bluemonday"
	"github.com/russross/blackfriday/v2"
)

type Page struct {
	Title string
	Body  []byte
}

func (p *Page) save() error {
	filename := config["pages"] + strings.Replace(filepath.Clean(p.Title+".md"), "/", "_", -1)
	return os.WriteFile(filename, p.Body, 0600)
}

type Attachment struct {
	Filename string
	Content  []byte
}

func (p *Attachment) save() error {
	filename := config["files"] + strings.Replace(filepath.Clean(p.Filename), "/", "_", -1)
	return os.WriteFile(filename, p.Content, 0600)
}

type TemplateRender struct {
	Pages       []string
	Attachments []string
	Page        *Page
	Content     string
	Data        any
}

func loadPage(title string) (*Page, error) {
	filename := config["pages"] + title + ".md"
	body, err := os.ReadFile(filepath.Clean(filename))
	if err != nil {
		return nil, err
	}
	return &Page{Title: title, Body: body}, nil
}

func renderMarkdown(rawMarkdown []byte) string {
	toparse := string(rawMarkdown)
	unsafe := blackfriday.Run([]byte(strings.Replace(toparse, "\r\n", "\n", -1)))
	html := string(bluemonday.UGCPolicy().SanitizeBytes(unsafe))
	return html
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	w.Header().Set("Content-encoding", "utf-8")
	log.Printf("[%s] INDEX [%s]: %s \n", r.RemoteAddr, r.Method, r.URL)

	page, err := loadPage("index")

	if err != nil {
		http.Redirect(w, r, "/edit/index", http.StatusFound)
		return
	}

	content := renderMarkdown(page.Body)
	t, err := template.ParseFS(tpls, "templates/base.html", "templates/view.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	p, _ := listPages()
	a, _ := listAttachments()
	tr := TemplateRender{Pages: p, Attachments: a, Page: page, Content: content}

	if err := t.ExecuteTemplate(w, "base", tr); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func viewHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	w.Header().Set("Content-encoding", "utf-8")
	log.Printf("[%s] VIEW [%s]: %s \n", r.RemoteAddr, r.Method, r.URL)
	title := r.URL.Path[len("/view/"):]
	page, err := loadPage(title)

	if err != nil {
		http.Redirect(w, r, "/edit/"+title, http.StatusFound)
		return
	}
	content := renderMarkdown(page.Body)
	t, err := template.ParseFS(tpls, "templates/base.html", "templates/view.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	p, _ := listPages()
	a, _ := listAttachments()
	tr := TemplateRender{Pages: p, Attachments: a, Page: page, Content: content}

	if err := t.ExecuteTemplate(w, "base", tr); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func editHandler(w http.ResponseWriter, r *http.Request) {
	title := r.URL.Path[len("/edit/"):]
	log.Printf("[%s] EDIT [%s]: %s \n", r.RemoteAddr, r.Method, r.URL)
	page, err := loadPage(title)
	if err != nil {
		page = &Page{Title: title}
	}
	p, _ := listPages()
	a, _ := listAttachments()
	tr := TemplateRender{Pages: p, Attachments: a, Page: page, Content: string(page.Body)}

	t, err := template.ParseFS(tpls, "templates/base.html", "templates/edit.html")

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := t.ExecuteTemplate(w, "base", tr); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func saveHandler(w http.ResponseWriter, r *http.Request) {
	title := r.URL.Path[len("/save/"):]
	log.Printf("[%s] SAVE [%s]: %s \n", r.RemoteAddr, r.Method, r.URL)
	body := r.FormValue("body")
	p := &Page{Title: title, Body: []byte(body)}
	err := p.save()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/view/"+title, http.StatusFound)
}
