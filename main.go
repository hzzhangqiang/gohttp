package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"code.google.com/p/rsc/qr"

	"github.com/go-martini/martini"
	"github.com/martini-contrib/render"
)

type Configure struct {
	port    int
	root    string
	private bool
}

var (
	gcfg = Configure{}
	m    *martini.ClassicMartini
)

func init() {
	r := martini.NewRouter()
	u := martini.New()
	u.Use(martini.Logger())
	u.Use(martini.Recovery())
	u.Use(render.Renderer(render.Options{
		Extensions: []string{".tmpl", ".html"},
	}))
	u.MapTo(r, (*martini.Routes)(nil))
	u.Action(r.Handle)
	m = &martini.ClassicMartini{u, r}
}

func formatSize(file os.FileInfo) string {
	if file.IsDir() {
		return "-"
	}
	size := file.Size()
	switch {
	case size > 1024*1024:
		return fmt.Sprintf("%.1fM", float64(size)/1024/1024)
	case size > 1024:
		return fmt.Sprintf("%.1fK", float64(size)/1024)
	default:
		return strconv.Itoa(int(size))
	}
	return ""
}

func errHandler(r render.Render, err error) {
	r.HTML(200, "error", map[string]string{
		"error": err.Error(),
	})
}

func dirHandler(host, path string, f *os.File, r render.Render) {
	if dirs, err := f.Readdir(-1); err == nil {
		files := make([]map[string]string, len(dirs)+1)
		files[0] = map[string]string{
			"name": "..", "href": "..", "size": "-", "mtime": "-",
		}
		for i, d := range dirs {
			href := d.Name()
			if d.IsDir() {
				href += "/"
			}

			files[i+1] = map[string]string{
				"href":  href,
				"name":  d.Name(),
				"size":  formatSize(d),
				"mtime": d.ModTime().Format("2006-01-02 15:04:05"),
				"host":  host,
				"path":  filepath.Join(path, d.Name()),
			}
		}
		r.HTML(200, "dirlist", map[string]interface{}{
			"dir":   f.Name(),
			"files": files,
		})
	}
}

func restoreAssets() {
	selfDir := filepath.Dir(os.Args[0])
	for _, folder := range []string{"data", "templates", "public"} {
		if _, err := os.Stat(folder); err != nil {
			if er := RestoreAssets(selfDir, folder); er != nil {
				log.Fatal("RestoreAssets", er)
			}
		}
	}
}

func main() {
	flag.IntVar(&gcfg.port, "port", 8000, "Which port to listen")
	flag.StringVar(&gcfg.root, "root", ".", "Watched root directory for filesystem events, also the HTTP File Server's root directory")
	flag.BoolVar(&gcfg.private, "private", false, "Only listen on lookback interface, otherwise listen on all interface")
	flag.Parse()

	// extract files
	restoreAssets()

	//m.Get("/", func(w http.ResponseWriter, r *http.Request) {
	//	http.Redirect(w, r, "/files/", http.StatusTemporaryRedirect)
	//})
	m.Get("/_qr", func(r *http.Request, w http.ResponseWriter) {
		text := r.FormValue("text")
		code, _ := qr.Encode(text, qr.M)
		w.Header().Set("Content-Type", "image/png")
		w.Write(code.PNG())
	})
	m.Get("/_/**", func(r *http.Request, w http.ResponseWriter, p martini.Params) {
		http.ServeFile(w, r, filepath.Join("public", p["_1"]))
	})
	m.Get("/**", func(req *http.Request, w http.ResponseWriter, params martini.Params, r render.Render) {
		path := params["_1"]
		if path == "" {
			path = "."
		}
		fullpath := filepath.Join(gcfg.root, path)
		log.Println(path)
		f, err := os.Open(fullpath)
		if err != nil {
			errHandler(r, err)
			return
		}
		defer f.Close()

		d, er := f.Stat()
		if er != nil {
			errHandler(r, er)
			return
		}
		if d.IsDir() {
			dirHandler(req.Host, path, f, r)
		} else {
			w.Header().Set("Content-Disposition", "attachment; filename="+filepath.Base(path))
			http.ServeFile(w, req, filepath.Join(gcfg.root, path))
		}
	})

	http.Handle("/", m)

	int := ":" + strconv.Itoa(gcfg.port)
	p := strconv.Itoa(gcfg.port)
	mesg := "; please visit http://127.0.0.1:" + p
	if gcfg.private {
		int = "localhost" + int
		log.Printf("listens on 127.0.0.1@" + p + mesg)
	} else {
		log.Printf("listens on 0.0.0.0@" + p + mesg)
	}
	if err := http.ListenAndServe(int, nil); err != nil {
		log.Fatal(err)
	}
}
