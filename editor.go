package main

import (
	"encoding/json"
	"fmt"
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
	filename := config["pages"] + filepath.Clean(p.Title+".md")

	dir := filepath.Dir(filename)

	if _, err := os.Stat(dir); os.IsNotExist(err) {

		err := os.MkdirAll(dir, 0755)
		if err != nil {
			return fmt.Errorf("ERROR creating directory : %v", err)
		}
	}

	return os.WriteFile(filename, p.Body, 0600)
}

type Attachment struct {
	Filename string
	Content  []byte
}

func (p *Attachment) save() error {
	filename := config["files"] + filepath.Clean(p.Filename)
	dir := filepath.Dir(filename)

	if _, err := os.Stat(dir); os.IsNotExist(err) {

		err := os.MkdirAll(dir, 0755)
		if err != nil {
			return fmt.Errorf("ERROR creating directory : %v", err)
		}
	}
	return os.WriteFile(filename, p.Content, 0600)
}

type TemplateRender struct {
	Page    *Page
	Content string
	Sidebar string
	Data    any
	Title   string
}

type BS5TreeE struct {
	Text     string      `json:"text"`
	Icon     string      `json:"icon"`
	Expanded bool        `json:"expanded,omitempty"`
	Class    string      `json:"class,omitempty"`
	Href     string      `json:"href,omitempty"`
	Data     string      `json:"data,omitempty"`
	ID       string      `json:"id,omitempty"`
	Nodes    []*BS5TreeE `json:"nodes,omitempty"`
}

func GenerateSidebarPages(dir string) (*BS5TreeE, error) {
	node := &BS5TreeE{Text: filepath.Base(dir)}
	node.Icon = "fa fa-folder"
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, file := range files {
		if file.IsDir() {
			childNode, err := GenerateSidebarPages(filepath.Join(dir, file.Name()))
			if err != nil {
				return nil, err
			}
			node.Nodes = append(node.Nodes, childNode)
		} else {

			ext := filepath.Ext(file.Name())
			if ext != ".md" {
				continue
			}

			filewoext := file.Name()[0 : len(file.Name())-len(ext)]
			n := &BS5TreeE{Text: filewoext, Icon: "fa fa-file-text-o"}
			n.Href = "/" + strings.Replace(filepath.Join(dir, filewoext), "pages/", "view/", 1)
			node.Nodes = append(node.Nodes, n)
		}
	}
	return node, nil
}

func GenerateSidebar(dir string) (*BS5TreeE, error) {
	node := &BS5TreeE{Text: filepath.Base(dir)}
	node.Icon = "fa fa-folder"
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, file := range files {
		if file.IsDir() {
			childNode, err := GenerateSidebar(filepath.Join(dir, file.Name()))
			if err != nil {
				return nil, err
			}
			node.Nodes = append(node.Nodes, childNode)
		} else {
			n := &BS5TreeE{Text: file.Name(), Icon: "fa fa-file-text-o"}
			n.ID = "/" + strings.Replace(filepath.Join(dir, file.Name()), "files/", "dl/", 1)
			n.Class = "binfiles list-group-item"
			node.Nodes = append(node.Nodes, n)
		}
	}
	return node, nil
}

func GenerateJsonNav() string {

	p, _ := GenerateSidebarPages(config["pages"])
	p.Expanded = true
	a, _ := GenerateSidebar(config["files"])
	menu := []BS5TreeE{*p, *a}

	jsonA, _ := json.MarshalIndent(menu, "", "  ")

	return string(jsonA)
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

	tr := TemplateRender{Title: page.Title, Page: page, Content: content, Sidebar: GenerateJsonNav()}

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

	tr := TemplateRender{Title: page.Title, Page: page, Content: content, Sidebar: GenerateJsonNav()}

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

	tr := TemplateRender{Title: page.Title, Page: page, Content: string(page.Body), Sidebar: GenerateJsonNav()}

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
