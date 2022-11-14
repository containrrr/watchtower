package dashboard

import (
	"fmt"
	"html/template"
	"net/http"
	"strings"

	log "github.com/sirupsen/logrus"
)

// Dashboard is the http server responsible for serving the static Dashboard files
type Dashboard struct {
	port       string
	rootDir    string
	apiPort    string
	apiScheme  string
	apiVersion string
}

// New is a factory function creating a new Dashboard instance
func New() *Dashboard {
	const webRootDir = "./web/dist" // Todo: needs to work in containerized environment
	const webPort = "8001"          // Todo: make configurable?
	const apiPort = "8080"          // Todo: make configurable?

	return &Dashboard{
		apiPort:    apiPort,
		apiScheme:  "http",
		apiVersion: "v1",
		rootDir:    webRootDir,
		port:       webPort,
	}
}

// Start the Dashboard and serve over HTTP
func (d *Dashboard) Start() error {
	go func() {
		d.runHTTPServer()
	}()
	return nil
}

func (d *Dashboard) templatedHttpHandler(h http.Handler) http.HandlerFunc {
	const apiUrlTemplate = "%s://%s:%s/%s/"
	indexTemplate, err := template.ParseFiles(d.rootDir + "/index.html")
	if err != nil {
		log.Error("Error when parsing index template")
		log.Error(err)
		return nil
	}

	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			hostName := strings.Split(r.Host, ":")[0]
			apiUrl := fmt.Sprintf(apiUrlTemplate, d.apiScheme, hostName, d.apiPort, d.apiVersion)
			err = indexTemplate.Execute(w, struct{ ApiUrl string }{
				ApiUrl: apiUrl,
			})
			if err != nil {
				log.Error("Error when executing index template")
				log.Error(err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		} else {
			h.ServeHTTP(w, r)
		}
	}
}

func (d *Dashboard) getHandler() http.Handler {
	return d.templatedHttpHandler(http.FileServer(http.Dir(d.rootDir)))
}

func (d *Dashboard) runHTTPServer() {
	serveMux := http.NewServeMux()
	serveMux.Handle("/", d.getHandler())

	log.Debug("Starting http dashboard server")
	log.Fatal(http.ListenAndServe(":"+d.port, serveMux))
}
