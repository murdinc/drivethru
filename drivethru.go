package main

import (
	"archive/tar"
	"compress/gzip"
	"crypto/md5"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	ini "gopkg.in/ini.v1"

	"github.com/asaskevich/govalidator"
	"github.com/goware/cors"
	"github.com/murdinc/terminal"
	"github.com/pressly/chi"
	"github.com/pressly/chi/middleware"
)

// Global
var menu *Menu

type Menu struct {
	URL      string
	Host     string
	Port     string
	Root     string
	Profiles []Profile
}

type Profile struct {
	Name        string   `ini:"-"` // actually the section name
	Source      string   `ini:"source"`
	Destination string   `ini:"destination"`
	Github      string   `ini:"github"`
	Universal   bool     `ini:"universal"`
	Extra       []string `ini:"extra"`
	URL         string   `ini:"-"`
}

func main() {
	terminal.Information("loading config...")
	var err error
	menu, err = loadMenu()
	if err != nil {
		terminal.ErrorLine(err.Error())
		return
	}
	terminal.Delta("config loaded.")

	r := chi.NewRouter()

	cors := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET"},
		AllowCredentials: true,
	})

	r.Use(cors.Handler)
	r.Use(middleware.StripSlashes)
	r.Use(middleware.Recoverer)

	r.Route("/get", func(r chi.Router) {
		r.Route("/:name", func(r chi.Router) {
			r.Get("/", getScript)
		})
	})

	r.Route("/hash", func(r chi.Router) {
		r.Route("/:name", func(r chi.Router) {
			r.Get("/", getHash)
			r.Route("/:os/:arch", func(r chi.Router) {
				r.Get("/", getHash)
			})
		})
	})

	r.Route("/download", func(r chi.Router) {
		r.Route("/:name", func(r chi.Router) {
			r.Get("/", getDownload)
			r.Route("/:os/:arch", func(r chi.Router) {
				r.Get("/", getDownload)
			})
		})
	})

	// Default port
	if menu.Port == "" {
		menu.Port = "2468"
	}

	// Default host
	if menu.Host == "" {
		menu.Host = "localhost"
	}

	// Default URL
	if menu.URL == "" {
		menu.URL = menu.Host + ":" + menu.Port
	}

	terminal.Delta("server started: " + menu.Host + ":" + menu.Port + ", with URL: " + menu.URL)
	http.ListenAndServe(menu.Host+":"+menu.Port, r)

}

func getScript(w http.ResponseWriter, r *http.Request) {
	profileName := chi.URLParam(r, "name")

Loop:
	for _, profile := range menu.Profiles {
		if profile.Name == profileName {
			profile.URL = menu.URL

			_, err := os.Stat(menu.Root + profile.Source)
			if err != nil {
				terminal.ErrorLine(err.Error() + " : " + profile.Source)
				break Loop
			}

			// If universal, pass them the universal script
			if profile.Universal {
				t, _ := template.New("script").Parse(universalScriptTemplate)
				t.Execute(w, profile)
				return
			}

			t, _ := template.New("script").Parse(scriptTemplate)
			t.Execute(w, profile)
			return
		}
	}

	t, _ := template.New("script").Parse(errorScriptTemplate)
	t.Execute(w, profileName)
}

func getDownload(w http.ResponseWriter, r *http.Request) {
	profileName := chi.URLParam(r, "name")
	osName := chi.URLParam(r, "os")
	archName := chi.URLParam(r, "arch")

	switch archName {
	case "x86_64":
		archName = "amd64"
	}

	terminal.Information(fmt.Sprintf("Download request for: %s, OS: %s, Arch: %s\n", profileName, osName, archName))

	for _, profile := range menu.Profiles {
		if profile.Name == profileName {
			source := menu.Root + profile.Source + "/"

			if !profile.Universal {
				if osName != "" && archName != "" {
					source = source + osName + "/" + archName + "/"
				} else {
					terminal.ErrorLine("Invalid request.")
					http.Error(w, "Invalid request.", 500)
					return
				}
			}

			terminal.Information(fmt.Sprintf("Download request file source: %s\n", source))

			err := zipIt(profile.Name, source, w)
			if err != nil {
				terminal.ErrorLine(err.Error())
			}

			return
		}
	}
}

func getHash(w http.ResponseWriter, r *http.Request) {
	profileName := chi.URLParam(r, "name")
	osName := chi.URLParam(r, "os")
	archName := chi.URLParam(r, "arch")

	switch archName {
	case "x86_64":
		archName = "amd64"
	}

	terminal.Information(fmt.Sprintf("Download request for: %s, OS: %s, Arch: %s\n", profileName, osName, archName))

	for _, profile := range menu.Profiles {
		if profile.Name == profileName {

			source := menu.Root + profile.Source + "/"

			if !profile.Universal {
				if osName != "" && archName != "" {
					source = source + osName + "/" + archName + "/"
				} else {
					terminal.ErrorLine("Invalid request.")
					http.Error(w, "Invalid request.", 500)
					return
				}
			}

			terminal.Information(fmt.Sprintf("Download request file source: %s\n", source))

			md5Hash := md5.New()

			err := zipIt(profile.Name, source, md5Hash)
			if err != nil {
				terminal.ErrorLine(err.Error())
				http.Error(w, err.Error(), 500)
				return
			}

			hashString := fmt.Sprintf("%x", md5Hash.Sum(nil))

			io.WriteString(w, hashString)

			terminal.Information("Returned Hash: " + hashString + " for file source: " + source + ".\n")
			return
		}
	}
}

func zipIt(name, source string, w io.Writer) (err error) {
	// gzip writer
	gz := gzip.NewWriter(w)
	defer gz.Close()
	gz.Name = name

	// tarball
	tarball := tar.NewWriter(gz)
	defer tarball.Close()

	info, err := os.Stat(source)
	if err != nil {
		return
	}

	var baseDir string
	if info.IsDir() {
		baseDir = filepath.Base(name)
	}

	return filepath.Walk(source,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			header, err := tar.FileInfoHeader(info, info.Name())
			if err != nil {
				return err
			}

			if baseDir != "" {
				header.Name = filepath.Join(baseDir, strings.TrimPrefix(path, source))
			}

			if err := tarball.WriteHeader(header); err != nil {
				return err
			}

			if info.IsDir() {
				return nil
			}

			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()
			_, err = io.Copy(tarball, file)
			return err
		})
}

func loadMenu() (*Menu, error) {
	menu := new(Menu)

	cfg, err := ini.Load("/etc/drivethru/drivethru.conf")
	if err != nil {
		return menu, err
	}

	apps := cfg.Sections()

	for _, app := range apps {

		if app.Name() == "DEFAULT" {
			menu.URL = app.Key("url").String()
			menu.Host = app.Key("host").String()
			menu.Port = app.Key("port").String()
			menu.Root = app.Key("root").String()
			continue
		}

		profile := new(Profile)

		err := app.MapTo(profile)
		if err != nil {
			return menu, err
		}

		profile.Name = app.Name()
		menu.Profiles = append(menu.Profiles, *profile)

		// Add path separators if needed

		// Source
		if profile.Source[0] != os.PathSeparator {
			profile.Source = fmt.Sprintf("%s%s", string(os.PathSeparator), profile.Source)
		}
		if profile.Source[len(profile.Source)-1] != os.PathSeparator {
			profile.Source = fmt.Sprintf("%s%s", profile.Source, string(os.PathSeparator))
		}

		// Destination
		if profile.Destination[0] != os.PathSeparator {
			profile.Destination = fmt.Sprintf("%s%s", string(os.PathSeparator), profile.Destination)
		}
		if profile.Destination[len(profile.Destination)-1] != os.PathSeparator {
			profile.Destination = fmt.Sprintf("%s%s", profile.Destination, string(os.PathSeparator))
		}

		terminal.Information(fmt.Sprintf("-	found profile named %s.", profile.Name))
	}

	if len(menu.Host) > 0 && !govalidator.IsHost(menu.Host) {
		return menu, errors.New("host specified in conf is invalid: " + menu.Host)
	}

	if len(menu.Port) > 0 && !govalidator.IsPort(menu.Port) {
		return menu, errors.New("port specified in conf is invalid: " + menu.Host)
	}

	// Add path separator if needed
	if menu.Root[len(menu.Root)-1] != os.PathSeparator {
		menu.Root = fmt.Sprintf("%s%s", menu.Root, string(os.PathSeparator))
	}

	terminal.Delta(fmt.Sprintf("loaded %d profiles.", len(menu.Profiles)))

	return menu, err
}

// TEMPLATES
////////////////

var scriptTemplate = `#!/bin/sh

FORMAT="tar.gz"
TEMPFOLDER="/tmp/drivethru-{{ .Name }}-$$"
TARBALL="$TEMPFOLDER/tar/{{ .Name }}.$FORMAT"
OS=$(uname)
ARCH=$(uname -m)
URL="http://{{ .URL }}/download/{{ .Name }}/$OS/$ARCH/"
DEST={{ .Destination }}

sudo mkdir -p /tmp/$$/ && sudo chmod 777 /tmp/$$/

sudo mkdir -p "$TEMPFOLDER/tar"
sudo mkdir -p "$TEMPFOLDER/expanded"
sudo chmod -R 777 "$TEMPFOLDER"

echo "Downloading $URL"

curl -o $TARBALL -L -f $URL
if [ $? -eq 0 ]
then
    echo "\nCopying {{ .Name }} into $DEST\n"
    sudo mkdir -p $DEST/
    tar -xzf $TARBALL -C $TEMPFOLDER/expanded && sudo cp -av $TEMPFOLDER/expanded/{{ .Name }}/* $DEST/ && rm -rf {{ .Name }}
    if [ $? -eq 0 ]
    then
        sudo rm -rf "$TEMPFOLDER"
        echo "\n{{ .Name }} has been installed into $DEST\n"
        {{ if .Extra }}{{ $url := .URL }}
        {{ range .Extra }}curl -s http://{{ $url }}/get/{{ . }} | sh
        {{end}}{{ else }}echo "Done!"{{end}}
        exit 0
    fi
else
    echo "Failed to install {{ .Name }}.\nPlease try downloading from {{ .Github }} instead."
    sudo rm -rf "$TEMPFOLDER"
fi

exit 1
`

var universalScriptTemplate = `#!/bin/sh

FORMAT="tar.gz"
TEMPFOLDER="/tmp/drivethru-{{ .Name }}-$$"
TARBALL="$TEMPFOLDER/tar/{{ .Name }}.$FORMAT"
URL="http://{{ .URL }}/download/{{ .Name }}/"
DEST="{{ .Destination }}"

sudo mkdir -p "$TEMPFOLDER/tar"
sudo mkdir -p "$TEMPFOLDER/expanded"
sudo chmod -R 777 "$TEMPFOLDER"

echo "Downloading $URL"

curl -o $TARBALL -L -f $URL
if [ $? -eq 0 ]
then
    echo "\nCopying {{ .Name }} into $DEST\n"
    sudo mkdir -p $DEST/
    tar -xzf $TARBALL -C $TEMPFOLDER/expanded && sudo cp -av $TEMPFOLDER/expanded/{{ .Name }}/* $DEST/ && rm -rf {{ .Name }}
    if [ $? -eq 0 ]
    then
        sudo rm -rf "$TEMPFOLDER"
        echo "\n{{ .Name }} has been installed into $DEST\n"
        {{ if .Extra }}{{range .Extra}}http://{{ .URL }}/get/{{ . }}/ | sh
        {{end}}{{end}}
        echo "Done!"
        exit 0
    fi
else
    echo "Failed to install {{ .Name }}.\nPlease try downloading from {{ .Github }} instead."
    sudo rm -rf "$TEMPFOLDER"
fi

exit 1
`

var errorScriptTemplate = `#!/bin/sh

echo "There was an error building your script for the download of {{ . }}, please contact the developer."
exit 1
`
