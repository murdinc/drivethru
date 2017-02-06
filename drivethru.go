package main

import (
	"archive/tar"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"

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
	Source   string
	Profiles []Profile
}

type Profile struct {
	Name   string `ini:"-"` // actually the section name
	Folder string `ini:"folder"`
	Github string `ini:"github"`
	URL    string `ini:"-"`
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

	r.Route("/download", func(r chi.Router) {
		r.Route("/:name/:os/:arch", func(r chi.Router) {
			r.Get("/", getDownload)
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

	for _, profile := range menu.Profiles {
		if profile.Name == profileName {
			profile.URL = menu.URL
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

			path := menu.Source + "/" + profile.Folder + "/" + osName + "/" + archName + "/" + profile.Name

			terminal.Information(fmt.Sprintf("Download request file path: %s\n", path))

			tarball := tar.NewWriter(w)
			defer tarball.Close()

			info, err := os.Stat(path)
			if err != nil {
				terminal.ErrorLine(err.Error())
				return
			}

			header, err := tar.FileInfoHeader(info, info.Name())
			if err != nil {
				terminal.ErrorLine(err.Error())
				return
			}

			if err := tarball.WriteHeader(header); err != nil {
				terminal.ErrorLine(err.Error())
				return
			}

			file, err := os.Open(path)
			if err != nil {
				terminal.ErrorLine(err.Error())
				return
			}
			defer file.Close()

			_, err = io.Copy(tarball, file)
			if err != nil {
				terminal.ErrorLine(err.Error())
				return
			}
		}
	}
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
			menu.Source = app.Key("source_folder").String()
			continue
		}

		profile := new(Profile)

		err := app.MapTo(profile)
		if err != nil {
			return menu, err
		}

		profile.Name = app.Name()
		menu.Profiles = append(menu.Profiles, *profile)

		terminal.Information(fmt.Sprintf("-	found profile named %s.", profile.Name))

	}

	if len(menu.Host) > 0 && !govalidator.IsHost(menu.Host) {
		return menu, errors.New("host specified in conf is invalid: " + menu.Host)
	}

	if len(menu.Port) > 0 && !govalidator.IsPort(menu.Port) {
		return menu, errors.New("port specified in conf is invalid: " + menu.Host)
	}

	terminal.Delta(fmt.Sprintf("loaded %d profiles.", len(menu.Profiles)))

	return menu, err
}

// TEMPLATES
////////////////

var scriptTemplate = `#!/bin/sh

FORMAT="tar.gz"
TARBALL="{{ .Name }}-$$.$FORMAT"
OS=$(uname)
ARCH=$(uname -m)
URL="http://{{ .URL }}/download/{{ .Name }}/$OS/$ARCH/"
DEST=/usr/local/bin

echo "Downloading $URL"

curl -o $TARBALL -L -f $URL
if [ $? -eq 0 ]
then
    echo "Copying {{ .Name }} binary into $DEST"
    sudo mkdir -p $DEST/
    tar -xzf $TARBALL && sudo mv -f {{ .Name }} $DEST/
    if [ $? -eq 0 ]
    then
        rm $TARBALL
        echo "{{ .Name }} has been installed into $DEST/{{ .Name }}"
        exit 0
    fi
else
    echo "Failed to determine your platform.\nTry downloading from {{ .Github }}"
fi

exit 1
`

var errorScriptTemplate = `#!/bin/sh

echo "There was an error building your script for the download of {{ . }}, please open an issue on github."
exit 1
`
