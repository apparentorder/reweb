package main

import (
	"bytes"
	"context"
	"compress/gzip"
	"encoding/base64"
	"errors"
	"io/ioutil"
	"fmt"
	"net/http"
	"os"
	"strings"
	"syscall"
	"time"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-lambda-go/events"
)

var ApplicationPort string
var ApplicationExec string
var ForceGzip bool
var WaitCode string
var WaitPath string
var Debug bool

func debug(s string) {
	if Debug {
		fmt.Printf(s + "\n")
	}
}

func HandleRequest(ctx context.Context, event events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	defer func() {
		if err := recover(); err != nil {
			fmt.Println("RE:WEB PANIC:", err)

			if Debug {
				exec := syscall.ProcAttr{
					Env: os.Environ(),
					Files: []uintptr{os.Stdin.Fd(), os.Stdout.Fd(), os.Stderr.Fd()},
				}

				syscall.ForkExec("/bin/sh", []string{"/bin/sh", "-c", "netstat -plne 2>&1"}, &exec)
				syscall.ForkExec("/bin/sh", []string{"/bin/sh", "-c", "ps axufw 2>&1"}, &exec)
			}

			os.Exit(69)
		}
	}()

	debug(fmt.Sprintf("EVENT OBJECT: %+v", event))

	// do not follow redirects;
	// https://stackoverflow.com/questions/23297520/how-can-i-make-the-go-http-client-not-follow-redirects-automatically
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	var body []byte
	if event.IsBase64Encoded {
		body, _ = base64.StdEncoding.DecodeString(event.Body)
	} else {
		body = []byte(event.Body)
	}

	path := event.RequestContext.HTTP.Path
	debug("PATH: " + path)
	if event.RawQueryString != "" {
		debug("PATH NEEDS QS")
		path += "?" + event.RawQueryString
		debug("PATH POST QS: " + path)
	}

	req, _ := http.NewRequest(
		event.RequestContext.HTTP.Method,
		"http://localhost:" + ApplicationPort + path,
		bytes.NewReader(body),
	)

	debug(fmt.Sprintf("REQUEST: %s %s\n", req.Method, req.URL))

	// set request headers; can't assign all of them at once because
	// Request.Header values are []string instead of string.
	for h, v := range event.Headers {
		req.Header.Set(h, v)
	}

	// set Cookie header. it gets eaten by API Gateway and stored in event.Cookies instead.
	req.Header.Set("Cookie", strings.Join(event.Cookies, "; "))

	// net/http will set/overwrite Host: using the destination address (localhost!), even if
	// it is explicitly present in req.Header. the workaround is to set request.Host instead.
	req.Host = event.Headers["host"]

	debug(fmt.Sprintf("REQUEST OBJECT: %+v", req))

	resp, err := client.Do(req)
	if err != nil {
		fmt.Println(fmt.Sprintf("HTTP REQUEST FAILED: %v", err))
		return events.APIGatewayV2HTTPResponse{}, errors.New(fmt.Sprintf("HTTP REQUEST FAILED: %v", err))
	}
	debug("RESPONSE: " + resp.Status)

	var respBody string
	respBodyBytes, _ := ioutil.ReadAll(resp.Body)

	if !ForceGzip {
		respBody = base64.StdEncoding.EncodeToString(respBodyBytes)
	} else {
		respBodyGz := bytes.Buffer{}
		gzipWriter := gzip.NewWriter(&respBodyGz)
		gzipWriter.Write(respBodyBytes)
		gzipWriter.Close()
		respBody = base64.StdEncoding.EncodeToString(respBodyGz.Bytes())
	}

	r := events.APIGatewayV2HTTPResponse{
		StatusCode: resp.StatusCode,
		Body: respBody,
		IsBase64Encoded: true,
	}

	r.Headers = map[string]string{}
	for h, v := range resp.Header {
		if h == "Set-Cookie" {
			for _, s := range v {
				r.Cookies = append(r.Cookies, s)
			}
		} else {
			r.Headers[h] = strings.Join(v, ", ")
		}
	}

	if ForceGzip {
		r.Headers["Content-Encoding"] = "gzip"
	}

	return r, nil
}

func main() {
	ApplicationExec = os.Getenv("REWEB_APPLICATION_EXEC")
	ApplicationPort = os.Getenv("REWEB_APPLICATION_PORT")
	ForceGzip = (os.Getenv("REWEB_FORCE_GZIP") != "")
	WaitPath = os.Getenv("REWEB_WAIT_PATH")
	WaitCode = os.Getenv("REWEB_WAIT_CODE")

	if WaitPath == "" { WaitPath = "/" }

	if ApplicationExec == "" { panic("Missing REWEB_APPLICATION_EXEC environment variable") }
	if ApplicationPort == "" { panic("Missing REWEB_APPLICATION_PORT environment variable") }

	exec := syscall.ProcAttr{
		Env: os.Environ(),
		Files: []uintptr{os.Stdin.Fd(), os.Stdout.Fd(), os.Stderr.Fd()},
	}

	syscall.ForkExec("/bin/sh", []string{"/bin/sh", "-c", ApplicationExec + " 2>&1"}, &exec)

	tries := 0
	for true {
		c := &http.Client{
			Timeout: 2 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
		resp, err := c.Get("http://localhost:" + ApplicationPort + WaitPath)
		if err != nil {
			if tries % 10 == 0 {
				fmt.Printf("SERVICE NOT UP: %v\n", err)
			}
		} else {
			if WaitCode == "" || fmt.Sprint(resp.StatusCode) == WaitCode {
				fmt.Printf("SERVICE UP: %s\n", resp.Status)
				break
			}

			status := resp.Status
			if resp.StatusCode == 302 {
				status = fmt.Sprintf("%s, Location: %s", resp.Status,
					resp.Header.Get("Location"))
			}

			fmt.Printf("SERVICE UP, NOT READY: Expecting HTTP %s, got: %s\n",
				WaitCode, status)
		}

		time.Sleep(100 * time.Millisecond)

		tries += 1
	}

	lambda.Start(HandleRequest)
}

