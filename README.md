# wikix
One file mini wiki with webdav capabalities

# build
```
$ go build
$ ./wikix -h
Usage of ./wikix:
  -auth string
    	user:pass
  -listen string
    	listen addr  (default ":8888")
$ ./wikix -listen 127.0.0.1:8888
2023/12/15 13:22:38 started on 127.0.0.1:8888
```

# usage
```
create new page :
newpage http://127.0.0.1:8888/edit/newpage

view raw attachment 

127.0.0.1:8888/dl/rawattachment
            
upload bin :
curl --data-binary @/etc/passwd 127.0.0.1:8888/up/filename

download bin :
curl -v 127.0.0.1:8888/dl/filename

webdav

dav://127.0.0.1:8888/dav/pages
dav://127.0.0.1:8888/dav/files

( also inside wiki via webdav js client )

nmap
upload results
curl -N -d @scan.xml  http://127.0.0.1:8888/nmap/up

upload results with forced update
curl -N -d @scan.xml  http://127.0.0.1:8888/nmap/up?forced=1

get open ports for 1.2.3.4
http://127.0.0.1:8888/nmap/show/1.2.3.4

get summary for 1.2.3.4
http://127.0.0.1:8888/nmap/show/1.2.3.4/sum

get all info as json for 1.2.3.4
http://127.0.0.1:8888/nmap/show/1.2.3.4/all

get ips for port 22 
http://127.0.0.1:8888/nmap/ports/22

get all ips
http://127.0.0.1:8888/nmap/ips



bash :

to get wi cli command on shell :

. <( 127.0.0.1:8888/rc )

$ wi
wi up <path>    # upload bin to wiki
wi dl <name>    # dl file from wiki
wi lsp          # list pages
wi lsf          # list files
wi upn <file>    # upload nmap xml scan
wi upnf <file>   # upload nmap xml scan and force update
wi port <port>  # get ip list for open <port> 
wi ip <ip>      # get <ip> opened ports
wi ipsum <ip>   # get <ip> detail
wi ips          # list of ips 

```
