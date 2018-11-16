package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strconv"
	"strings"
)

var (
	flagListenAddr = flag.String("listen", ":8080", "the http listen address 'host:port'")
	flagRoot       = flag.String("root", ".", "the document root to serve")
	flagEntryPoint = flag.String("entry", "bash", "the main entrypoint that executes the cgi scripts")
)

func main() {
	flag.Parse()
	http.ListenAndServe(*flagListenAddr, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		req.ParseForm()

		norm := func(s string) string {
			return regexp.MustCompile(`[^a-zA-Z0-9_]+`).ReplaceAllString(strings.ToUpper(s), "_")
		}

		currentFilename := path.Join(*flagRoot, req.URL.Path)
		_, err := os.Stat(currentFilename)
		if err != nil {
			http.Error(w, "file not found", 404)
			return
		}
		isCGI := strings.HasSuffix(currentFilename, ".cgi")

		if !isCGI {
			http.ServeFile(w, req, currentFilename)
			return
		}

		env := map[string]string{}

		// adding the form(s) params in the 'env.PARAM_' prefix
		for k, v := range req.Form {
			env["CGI_PARAM_"+norm(k)] = v[0]
		}

		// adding the header values in the 'env.HEADER_' prefix
		for k, v := range req.Header {
			env["CGI_HEADER_"+norm(k)] = v[0]
		}

		// adding more info
		env["CGI_URL_HOST"] = req.Host
		env["CGI_URL_PORT"] = req.URL.Port()
		env["CGI_URL_PATH"] = req.URL.Path
		env["CGI_URL_QUERY"] = req.URL.RawQuery
		env["CGI_SERVER_FILENAME"] = currentFilename
		env["CGI_DOCUMENT_ROOT"] = *flagRoot

		fileData, err := ioutil.ReadFile(currentFilename)
		if err != nil {
			http.Error(w, "file not found", 404)
			return
		}
		fileParts := strings.SplitN(string(fileData), "\n", 2)
		if len(fileParts) < 1 {
			http.Error(w, "no output", 404)
			return
		}

		scriptData, _ := ioutil.ReadFile(currentFilename)
		scriptDataParts := bytes.SplitN(scriptData, []byte("\n"), 2)
		execCMD := strings.TrimSpace(strings.TrimLeft(string(scriptDataParts[0]), "#!"))
		envSlice := []string{}
		for k, v := range env {
			envSlice = append(envSlice, fmt.Sprintf("%s=%s", k, v))
		}

		out := output{data: bytes.NewBuffer([]byte{})}
		cmd := exec.Command(execCMD, currentFilename)
		cmd.Env = append(cmd.Env, envSlice...)
		cmd.Dir = path.Dir(currentFilename)
		cmd.Stderr = out
		cmd.Stdout = out
		cmd.Run()
		out.Pipe(w)
	}))
}

type output struct {
	data   *bytes.Buffer
	body   string
	status int
}

func (o output) Write(d []byte) (int, error) {
	return o.write(d)
}

func (o *output) write(d []byte) (int, error) {
	o.data.Write(d)
	return len(d), nil
}

func (o output) Pipe(w http.ResponseWriter) {
	parts := strings.SplitN(o.data.String(), "\n\n", 2)
	if len(parts) < 2 {
		o.body = ""
	} else {
		o.body = parts[1]
	}
	o.status = 200
	lines := strings.Split(parts[0], "\n")
	for _, l := range lines {
		h := strings.SplitN(l, ":", 2)
		if len(h) < 2 {
			continue
		}
		if h[0] == "Status" {
			o.status, _ = strconv.Atoi(h[1])
		} else {
			w.Header().Add(strings.TrimSpace(h[0]), strings.TrimSpace(h[1]))
		}
	}
	w.WriteHeader(o.status)
	w.Write([]byte(o.body))
}
