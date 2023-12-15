package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"text/template"

	"github.com/glebarez/sqlite"
	"github.com/gorilla/mux"
	"github.com/tomsteele/go-nmap"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Script struct {
	gorm.Model
	ID       uint
	Title    string
	ScriptId string
	Output   string
}

type Port struct {
	gorm.Model
	ID       uint
	PortId   uint
	Port     uint
	Protocol string
	State    string
	Service  string
	Scripts  []Script `gorm:"foreignKey:ScriptId;constraint:OnDelete:CASCADE"`
}

type Host struct {
	gorm.Model
	ID          uint
	IP          string `gorm:"unique;IP"`
	Hostname    string
	Comment     string
	Raw         datatypes.JSON
	Ports       []Port   `gorm:"foreignKey:PortId;constraint:OnDelete:CASCADE"`
	HostScripts []Script `gorm:"foreignKey:ScriptId;constraint:OnDelete:CASCADE"`
}

func (h *Host) Exists(db *gorm.DB, ip string) bool {
	var Exist bool
	db.Raw("select exists(select 1 from hosts where IP= ? ) AS found;",
		ip).Scan(&Exist)
	return Exist
}

func (h *Host) AfterDelete(tx *gorm.DB) (err error) {
	tx.Clauses(clause.Returning{}).Where("id = ?", h.ID).Delete(&Port{})
	tx.Clauses(clause.Returning{}).Where("id = ?", h.ID).Delete(&Script{})
	return
}

func (h *Port) AfterDelete(tx *gorm.DB) (err error) {
	tx.Clauses(clause.Returning{}).Where("id = ?", h.ID).Delete(&Script{})
	return
}

func SetupGorm() error {
	db, err := gorm.Open(sqlite.Open(config["gorm"]), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("unable to connect database")
	}

	db.AutoMigrate(Port{})
	db.AutoMigrate(Script{})
	db.AutoMigrate(Host{})

	return nil
}

func parseNmap(w http.ResponseWriter, toparse []byte, update bool) error {

	db, err := gorm.Open(sqlite.Open(config["gorm"]), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("unable to connect database")
	}

	nn, err := nmap.Parse(toparse)
	if err != nil {
		w.Write([]byte("unable to parse xml nmap\n"))
		return fmt.Errorf("unable to parse xml nmap")
	}
	var batchInsert []Host
	for _, host := range (*nn).Hosts {

		if host.Status.State == "up" {
			if len(host.Addresses) != 0 {

				hostobj := &Host{}
				ip := host.Addresses[0].Addr
				if hostobj.Exists(db, ip) {
					fmt.Println("exists ", ip)
					if !update {
						continue
					} else {
						db.Take(&hostobj, "IP = ?", ip)
						db.Unscoped().Delete(&hostobj)
					}
				}

				hostobj.IP = ip
				hostobj.Comment = host.Comment

				obj, err := json.Marshal(host)
				if err != nil {
					w.Write([]byte(fmt.Sprintf("unable to marshall host %s \n", ip)))
				} else {
					hostobj.Raw = obj
				}

				if len(host.Hostnames) != 0 {
					hostobj.Hostname = host.Hostnames[0].Name
				}

				for _, port := range host.Ports {
					portobj := &Port{}
					portobj.Port = uint(port.PortId)
					portobj.Protocol = port.Protocol
					portobj.State = port.State.State
					portobj.Service = port.Service.Name

					w.Write([]byte(fmt.Sprintf("adding %s:%d \n", hostobj.IP, portobj.Port)))

					for _, script := range port.Scripts {
						scriptobj := &Script{}
						scriptobj.Title = script.Id
						scriptobj.Output = script.Output
						portobj.Scripts = append(portobj.Scripts, *scriptobj)
					}
					hostobj.Ports = append(hostobj.Ports, *portobj)
				}
				for _, script := range host.HostScripts {
					scriptobj := &Script{}
					scriptobj.Title = script.Id
					scriptobj.Output = script.Output
					hostobj.HostScripts = append(hostobj.HostScripts, *scriptobj)
				}
				batchInsert = append(batchInsert, *hostobj)
			}
		}
	}
	if len(batchInsert) > 0 {
		db.Session(&gorm.Session{FullSaveAssociations: true}).Create(batchInsert)
		//db.Create(batchInsert)
	}
	return nil
}

func NmapRouter() http.Handler {

	nmapRouter := mux.NewRouter()

	db, err := gorm.Open(sqlite.Open(config["gorm"]), &gorm.Config{})
	if err != nil {
		log.Printf("unable to connect database %v", err)
	}

	nmapRouter.HandleFunc("/up", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[%s] NMAP UPLOAD [%s]: %s \n", r.RemoteAddr, r.Method, r.URL)
		content, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if r.URL.Query().Get("force") != "" {
			err = parseNmap(w, content, true)
		} else {
			err = parseNmap(w, content, false)
		}

		if err != nil {
			w.Write([]byte("NOK\n"))
		} else {
			w.Write([]byte("OK\n"))
		}

	}).Methods("POST")

	nmapRouter.HandleFunc("/show/{ip}/all", func(w http.ResponseWriter, r *http.Request) {
		ip := mux.Vars(r)["ip"]
		var host Host
		db.Preload("Ports").Preload("Ports.Scripts").Preload("HostScripts").Take(&host, "IP = ?", ip)
		res, _ := json.MarshalIndent(host, "", "  ")
		w.Write(res)
	}).Methods("GET")

	nmapRouter.HandleFunc("/show/{ip}", func(w http.ResponseWriter, r *http.Request) {
		ip := mux.Vars(r)["ip"]
		var host Host
		res := ""
		db.Preload("Ports").Take(&host, "IP = ?", ip)
		for _, port := range host.Ports {
			res = res + fmt.Sprintf("%d\n", port.Port)
		}
		w.Write([]byte(res))

	}).Methods("GET")

	nmapRouter.HandleFunc("/show/{ip}/sum", func(w http.ResponseWriter, r *http.Request) {
		ip := mux.Vars(r)["ip"]
		var host Host
		res := ""
		db.Preload("Ports").Preload("Ports.Scripts").Take(&host, "IP = ?", ip)
		for _, port := range host.Ports {
			res = res + fmt.Sprintf("%d:\n", port.Port)
			for _, script := range port.Scripts {
				res = res + fmt.Sprintf("\t%s:\n\t\t%s\n", script.Title, script.Output)
			}
			res = res + "---------------------------\n"
		}
		w.Write([]byte(res))

	}).Methods("GET")

	nmapRouter.HandleFunc("/ports/{port}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("Content-encoding", "utf-8")
		port := mux.Vars(r)["port"]
		var hosts []string
		db.Raw("select hosts.IP from ports,hosts where port=? and ports.port_id = hosts.id;", port).Find(&hosts)
		res := ""
		for _, host := range hosts {
			res = res + fmt.Sprintf("%s\n", host)
		}
		w.Write([]byte(res))

	}).Methods("GET")

	nmapRouter.HandleFunc("/ips", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("Content-encoding", "utf-8")
		var hosts []string
		db.Raw("select hosts.IP from hosts").Find(&hosts)
		res := ""
		for _, host := range hosts {
			res = res + fmt.Sprintf("%s\n", host)
		}
		w.Write([]byte(res))
	}).Methods("GET")

	return nmapRouter
}

func GenerateSidebarNmap(ips []string) string {
	var toJson []*BS5TreeE
	for _, ip := range ips {
		n := &BS5TreeE{Text: ip, Icon: "fa fa-file-text-o", Class: "ipaddr", ID: ip}
		toJson = append(toJson, n)
	}

	jsonA, _ := json.MarshalIndent(toJson, "", "  ")

	return string(jsonA)
}

func NmapHandler(w http.ResponseWriter, r *http.Request) {

	log.Printf("[%s] NMAP VIEW [%s]: %s \n", r.RemoteAddr, r.Method, r.URL)

	db, err := gorm.Open(sqlite.Open(config["gorm"]), &gorm.Config{})
	if err != nil {
		log.Printf("unable to connect database %v", err)
	}
	t, err := template.ParseFS(tpls, "templates/base.html", "templates/nmap.html")

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var hosts []Host

	db.Select("IP").Find(&hosts)

	var ips []string
	for _, host := range hosts {
		ips = append(ips, host.IP)
	}

	tr := TemplateRender{Title: "nmap", Sidebar: GenerateSidebarNmap(ips)}

	if err := t.ExecuteTemplate(w, "base", tr); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
