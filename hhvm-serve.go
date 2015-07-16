package main

import "github.com/LionsAd/hhvm-serve/fcgiclient"
import "errors"
import "net/http"
import "io"
import "fmt"
import "flag"
import "os"
import "strings"
import "strconv"

var documentRoot string
var listen string
var staticHandler *http.ServeMux

func handler(w http.ResponseWriter, r *http.Request) {
	var filename string
	var scriptName string
	var length int64

	if r.URL.Path == "/" {
		scriptName = "/index.php"
		filename = documentRoot + "/index.php"
	} else {
		scriptName = r.URL.Path
		filename = documentRoot + r.URL.Path
	}

	_, err := os.Stat(filename)
	if !strings.HasSuffix(filename, ".php") && err == nil {
		staticHandler.ServeHTTP(w, r)
		return
	}

	if os.IsNotExist(err) {
		scriptName = "/index.php"
		filename = documentRoot + "/index.php"
	}

	env := make(map[string]string)
	env["REQUEST_METHOD"] = r.Method
	env["SCRIPT_FILENAME"] = filename
	env["SCRIPT_NAME"] = scriptName
	env["SERVER_SOFTWARE"] = "go / fcgiclient "
	env["REMOTE_ADDR"] = "127.0.0.1"
	env["SERVER_PROTOCOL"] = "HTTP/1.1"
	env["PATH_INFO"] = r.URL.Path
	env["DOCUMENT_ROOT"] = documentRoot
	env["QUERY_STRING"] = r.URL.RawQuery

	fcgi, err := fcgiclient.New("127.0.0.1", 9000)
	if err != nil {
		fmt.Printf("err: %v", err)
	}

        var resp *http.Response
        contentLength, _ := strconv.Atoi(r.Header.Get("Content-Length"))
        switch r.Method {
        case "HEAD":
		resp, err = fcgi.Head(env)
        case "GET":
		resp, err = fcgi.Get(env)
        case "OPTIONS":
		resp, err = fcgi.Options(env)
        case "POST":
		resp, err = fcgi.Post(env, r.Header.Get("Content-Type"), r.Body, contentLength)
        case "PUT":
		resp, err = fcgi.Put(env, r.Header.Get("Content-Type"), r.Body, contentLength)
        case "PATCH":
		resp, err = fcgi.Patch(env, r.Header.Get("Content-Type"), r.Body, contentLength)
        case "DELETE":
		resp, err = fcgi.Delete(env, r.Header.Get("Content-Type"), r.Body, contentLength)
        default:
		fmt.Printf("ERROR: %s - Bad method\n", r.URL.Path)
		return
        }

	if resp.Body != nil {
		defer resp.Body.Close()
	}

	if err != nil {
		fmt.Printf("ERROR: %s - %v\n", r.URL.Path, err)
	}

        // Write the response header
	for key, vals := range resp.Header {
		for _, val := range vals {
			w.Header().Add(key, val)
		}
	}

	w.WriteHeader(resp.StatusCode)

        length, err = io.Copy(w, resp.Body)
        if err != nil {
		fmt.Printf("ERROR: %s - %v\n", r.URL.Path, err)
        }

	fmt.Printf("%s \"%s %s %s\" %d %d \"%s\"\n", r.RemoteAddr, r.Method, r.URL.Path, r.Proto, resp.StatusCode, length, r.UserAgent())
}

func ParseFastCgiResponse(content string) (int, map[string]string, string, error) {
	var headers map[string]string

	parts := strings.SplitN(content, "\r\n\r\n", 2)

	if len(parts) < 2 {
		return 502, headers, "", errors.New("Cannot parse FastCGI Response")
	}

	headerParts := strings.Split(parts[0], ":")
	body := parts[1]
	status := 200

	if strings.HasPrefix(headerParts[0], "Status:") {
		lineParts := strings.SplitN(headerParts[0], " ", 3)
		status, _ = strconv.Atoi(lineParts[1])
	}

	for _, line := range headerParts {
		lineParts := strings.SplitN(line, ":", 2)

		if len(lineParts) < 2 {
			continue
		}

		lineParts[1] = strings.TrimSpace(lineParts[1])

		if lineParts[0] == "Status" {
			continue
		}

		headers[lineParts[0]] = lineParts[1]
	}

	return status, headers, body, nil
}

func main() {

	cwd, _ := os.Getwd()
	flag.StringVar(&documentRoot, "document-root", cwd, "The document root to serve files from")
	flag.StringVar(&listen, "listen", "localhost:8080", "The webserver bind address to listen to.")

	flag.Parse()

	staticHandler = http.NewServeMux()
	staticHandler.Handle("/", http.FileServer(http.Dir(documentRoot)))

	fmt.Printf("Listening on http://%s\n", listen)
	fmt.Printf("Document root is %s\n", documentRoot)
	fmt.Printf("Press Ctrl-C to quit.\n")

	http.HandleFunc("/", handler)
	http.ListenAndServe(listen, nil)
}
