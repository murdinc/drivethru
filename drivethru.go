package main

import (
	"archive/tar"
	"compress/gzip"
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
	Name        string `ini:"-"` // actually the section name
	Source      string `ini:"source"`
	Destination string `ini:"destination"`
	Github      string `ini:"github"`
	Universal   bool   `ini:"universal"`
	URL         string `ini:"-"`
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
		r.Route("/:name", func(r chi.Router) {
			r.Get("/", getUniversalDownload)
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
			source := menu.Root + profile.Source + "/" + osName + "/" + archName + "/"
			terminal.Information(fmt.Sprintf("Download request file source: %s\n", source))

			// gzip writer
			gz := gzip.NewWriter(w)
			defer gz.Close()
			gz.Name = profile.Name

			// tarball
			tarball := tar.NewWriter(gz)
			defer tarball.Close()

			info, err := os.Stat(source)
			if err != nil {
				terminal.ErrorLine(err.Error())
				return
			}

			var baseDir string
			if info.IsDir() {
				baseDir = filepath.Base(profile.Name)
			}

			err = filepath.Walk(source,
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

			if err != nil {
				terminal.ErrorLine(err.Error())
			}

			return
		}
	}
}

func getUniversalDownload(w http.ResponseWriter, r *http.Request) {
	profileName := chi.URLParam(r, "name")

	terminal.Information(fmt.Sprintf("Download request for: %s (universal)\n", profileName))

	for _, profile := range menu.Profiles {
		if profile.Name == profileName {
			source := menu.Root + profile.Source + "/"
			terminal.Information(fmt.Sprintf("Download request file source: %s\n", source))

			// gzip writer
			gz := gzip.NewWriter(w)
			defer gz.Close()
			gz.Name = profile.Name

			// tarball
			tarball := tar.NewWriter(gz)
			defer tarball.Close()

			info, err := os.Stat(source)
			if err != nil {
				terminal.ErrorLine(err.Error())
				return
			}

			var baseDir string
			if info.IsDir() {
				baseDir = filepath.Base(profile.Name)
			}

			err = filepath.Walk(source,
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

			if err != nil {
				terminal.ErrorLine(err.Error())
			}

			return
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
TARBALL="{{ .Name }}-$$.$FORMAT"
OS=$(uname)
ARCH=$(uname -m)
URL="http://{{ .URL }}/download/{{ .Name }}/$OS/$ARCH/"
DEST={{ .Destination }}

echo "Downloading $URL"

curl -o $TARBALL -L -f $URL
if [ $? -eq 0 ]
then
    echo "\nCopying {{ .Name }} into $DEST\n"
    sudo mkdir -p $DEST/
    sudo mkdir -p /tmp/$$/ && sudo chmod 777 /tmp/$$/
    tar -xzf $TARBALL -C /tmp/$$/ && sudo cp -av /tmp/$$/{{ .Name }}/* $DEST/ && rm -rf {{ .Name }} && sudo rm -rf /tmp/$$
    if [ $? -eq 0 ]
    then
        rm $TARBALL
        echo "\n{{ .Name }} has been installed into $DEST\n"
        echo "Done!"
        exit 0
    fi
else
	echo "Failed to install {{ .Name }}.\nPlease try downloading from {{ .Github }} instead."
fi

exit 1
`

var universalScriptTemplate = `#!/bin/sh

FORMAT="tar.gz"
TARBALL="{{ .Name }}-$$.$FORMAT"
URL="http://{{ .URL }}/download/{{ .Name }}/"
DEST={{ .Destination }}

echo "Downloading $URL"

curl -o $TARBALL -L -f $URL
if [ $? -eq 0 ]
then
    echo "\nCopying {{ .Name }} into $DEST\n"
    sudo mkdir -p $DEST/
    sudo mkdir -p /tmp/$$/ && sudo chmod 777 /tmp/$$/
    tar -xzf $TARBALL -C /tmp/$$/ && sudo cp -av /tmp/$$/{{ .Name }}/* $DEST/ && rm -rf {{ .Name }} && sudo rm -rf /tmp/$$
    if [ $? -eq 0 ]
    then
        rm $TARBALL
        echo "\n{{ .Name }} has been installed into $DEST\n"
        echo "Done!"
        exit 0
    fi
else
    echo "Failed to install {{ .Name }}.\nPlease try downloading from {{ .Github }} instead."
fi

exit 1
`

var errorScriptTemplate = `#!/bin/sh

echo "There was an error building your script for the download of {{ . }}, please contact the developer."
exit 1
`
