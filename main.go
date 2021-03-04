package main

import (
	"bytes"
	"context"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	//"errors"
	"io/ioutil"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"syscall"
	"time"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-lambda-go/events"
)

var Version string

var ApplicationPort string
var ApplicationExec string
var ForceGzip bool
var WaitCode string
var WaitPath string
var Debug bool

const PrefixDebug = "re:Web -- debug:  "
const PrefixInfo  = "re:Web -- "
const PrefixError = "re:Web -- ERROR:  "

func debug(s string) {
	if Debug {
		fmt.Printf("%s%s\n", PrefixDebug, s)
	}
}

type lambdaHandler struct {}

func (h lambdaHandler) Invoke(ctx context.Context, payload []byte) ([]byte, error) {
	var isALB = false

	var err error
	var albRequest events.ALBTargetGroupRequest
	var apiGwRequest events.APIGatewayV2HTTPRequest

	type Event struct {
		Body string
		IsBase64Encoded bool
		Path string
		Headers map[string]string
		HTTPMethod string
	}
	var event Event

	debug("=== Invoke =============================================================")

	//debug("EVENT:")
	//debug(string(payload))

	err = json.NewDecoder(bytes.NewReader(payload)).Decode(&apiGwRequest)
	if err != nil {
		e := fmt.Errorf(PrefixError + "cannot parse payload as API Gateway request: %v", err)
		fmt.Println(e)
		return nil, e
	}

	if apiGwRequest.RequestContext.APIID != "" {
		// if an API ID is present, we can be sure it was an API Gateway request
		isALB = false
		debug(fmt.Sprintf("Parsed as API Gateway request"))
		//debug(fmt.Sprintf("%+v", apiGwRequest))

		event = Event{
			Body: apiGwRequest.Body,
			IsBase64Encoded: apiGwRequest.IsBase64Encoded,
			Path: apiGwRequest.RequestContext.HTTP.Path,
			HTTPMethod: apiGwRequest.RequestContext.HTTP.Method,
		}
	} else {
		// otherwise, we'll assume we were not invoked via API Gateway but via ALB instead.
		isALB = true

		err = json.NewDecoder(bytes.NewReader(payload)).Decode(&albRequest)
		if err != nil {
			e := fmt.Errorf(PrefixError + "cannot parse payload as ALB  request: %v", err)
			fmt.Println(e)
			return nil, e
		}
		debug(fmt.Sprintf("Parsed as ALB request"))
		debug(fmt.Sprintf("%+v", albRequest))

		event = Event{
			Body: albRequest.Body,
			IsBase64Encoded: albRequest.IsBase64Encoded,
			Path: albRequest.Path,
			HTTPMethod: albRequest.HTTPMethod,
		}
	}

	debug("--- Build Request ------------------------------------------------------")

	var body []byte
	if event.IsBase64Encoded {
		body, err = base64.StdEncoding.DecodeString(event.Body)
		if err != nil {
			e := fmt.Errorf(PrefixError + "cannot base64-decode request body: %v", err)
			fmt.Println(e)
			return nil, e
		}
	} else {
		body = []byte(event.Body)
	}

	path := event.Path
	debug("PATH: " + path)

	if !isALB {
		if apiGwRequest.RawQueryString != "" {
			debug("PATH NEEDS QS")
			path += "?" + apiGwRequest.RawQueryString
			debug("PATH POST QS: " + path)
		}
	} else {
		if len(albRequest.MultiValueQueryStringParameters) > 0 {
			debug("PATH NEEDS QS")
			kv := []string{}
			for mvqsp, v := range albRequest.MultiValueQueryStringParameters {
				for _, s := range v {
					kv = append(kv, mvqsp + "=" + s)
				}
			}
			path += "?" + strings.Join(kv, "&")
			debug("PATH POST QS: " + path)
		}
	}

	req, err := http.NewRequest(
		event.HTTPMethod,
		"http://localhost:" + ApplicationPort + path,
		bytes.NewReader(body),
	)

	if err != nil {
		e := fmt.Errorf(PrefixError + "http.NewRequest: %v", err)
		fmt.Println(e)
		return nil, e
	}

	debug(fmt.Sprintf("REQUEST: %s %s\n", req.Method, req.URL))

	if isALB {
		// ALB: Payload *must* use MultiValueHeaders, because this can only be configured to
		// to MVHeaders exclusively -- for Request AND Response -- or not at all.
		// Unfortunately, "not at all" does not work with Cookies at all, because returning
		// multiple Cookies (Set-Cookie headers) cannot work without MVHeaders.

		if len(albRequest.Headers) > 0 {
			e := fmt.Errorf(PrefixError + "ALB payload without MultiValueHeaders -- check the ALB Target Group settings!")
			fmt.Println(e)
			return nil, e
		}

		for h, v := range albRequest.MultiValueHeaders {
			// as the net/http docs say: "To use non-canonical keys, assign to the map directly." :E
			// apparently, this also applies to multi-value headers.
			req.Header[http.CanonicalHeaderKey(h)] = v
		}
	} else {
		// API Gateway: V2 payload *never* uses MultiValue{Headers,QS}, so everything
		// is in .Headers as usual -- except for Cookies, of course.
		for h, v := range apiGwRequest.Headers {
			req.Header.Set(h, v)
		}

		// set Cookie header from event.Cookies (it gets eaten by API Gateway and stored there)
		req.Header.Set("Cookie", strings.Join(apiGwRequest.Cookies, "; "))

	}

	// net/http will set/overwrite Host: using the destination address (localhost!), even if
	// it is explicitly present in req.Header. the workaround is to set request.Host instead.
	req.Host = req.Header.Get("Host")

	debug(fmt.Sprintf("REQUEST OBJECT: %+v", req))

	debug("--- Send Request -------------------------------------------------------")

	httpClient := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		e := fmt.Errorf(PrefixError + "HTTP REQUEST FAILED: %v", err)
		fmt.Println(e)
		return nil, e
	}
	debug("RESPONSE: " + resp.Status)

	debug("--- Build Response -----------------------------------------------------")

	var respBody string
	respBodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		e := fmt.Errorf(PrefixError + "ReadAll() on response body: %v", err)
		fmt.Println(e)
		return nil, e
	}

	if !ForceGzip {
		respBody = base64.StdEncoding.EncodeToString(respBodyBytes)
	} else {
		respBodyGz := bytes.Buffer{}
		gzipWriter := gzip.NewWriter(&respBodyGz)
		gzipWriter.Write(respBodyBytes)
		gzipWriter.Close()
		respBody = base64.StdEncoding.EncodeToString(respBodyGz.Bytes())
	}

	var responseHeaders = map[string]string{}
	var responseCookies = []string{}

	for h, v := range resp.Header {
		if h == "Set-Cookie" {
			for _, s := range v {
				responseCookies = append(responseCookies, s)
			}
		} else {
			responseHeaders[h] = strings.Join(v, ", ")
		}
	}

	//
	// any absolute redirect targeting *this* hostname, or localhost,
	// i.e. the site we're running, might be a garbage redirect.
	//
	// classic example is apache redirecting to http://hostname:<internalPort>,
	// which might be different from the protocol and/or port that is seen by any
	// consumers "in front of" the API Gateway / ALB.
	//
	// if there is such a Location header, we double-check a few things and change
	// as necessary.
	//
	if responseHeaders["Location"] != "" {
		responseLocation, err := url.Parse(responseHeaders["Location"])
		if err != nil {
			e := fmt.Errorf(PrefixError + "url.Parse() for 'Location' response header: %v", err)
			fmt.Println(e)
			return nil, e
		}

		if responseLocation.Hostname() == "localhost" {
			responseLocation.Host = fmt.Sprintf("%s:%s", req.Header.Get("Host"), responseLocation.Port())
			debug("LocationFix: localhost -> " + responseLocation.Host)
		}

		if strings.ToLower(responseLocation.Hostname()) == strings.ToLower(req.Header.Get("Host")) {
			hostOrig := responseLocation.Host

			// 1) if the redirect is targeting the REWEB_APPLICATION_PORT, replace it
			// with the port where the *request* went to (X-Forwarded-Port).
			if responseLocation.Port() == ApplicationPort && ApplicationPort != req.Header.Get("X-Forwarded-Port") {
				responseLocation.Host = fmt.Sprintf(
					"%s:%s",
					req.Header.Get("Host"),
					req.Header.Get("X-Forwarded-Port"),
				)
				debug("LocationFix: " + hostOrig + " -> " + responseLocation.Host)
			}

			// 2) for API Gateway requests, we always fix up http to https, as http://
			// is not supported on API Gateway.
			if responseLocation.Scheme == "http" && !isALB {
				responseLocation.Scheme = "https"
				debug("LocationFix: http -> https")
			}

			// 3) for ALB requests, we fixup "http" to "https" only if the *request* was on
			// https as well. (this is somewhat opinionated and may need to be revisited)
			if responseLocation.Scheme == "http" && strings.ToLower(req.Header.Get("X-Forwarded-Proto")) == "https" {
				responseLocation.Scheme = "https"
				debug("LocationFix: http -> https")
			}

			responseHeaders["Location"] = responseLocation.String()
		}
	}

	if ForceGzip {
		responseHeaders["Content-Encoding"] = "gzip"
	}

	if !isALB {
		r := events.APIGatewayV2HTTPResponse{
			StatusCode: resp.StatusCode,
			Body: respBody,
			IsBase64Encoded: true,
			Headers: responseHeaders,
			Cookies: responseCookies,
		}
		jsonResponse, _ := json.Marshal(r)
		debug(string(jsonResponse))
		return jsonResponse, nil
	} else {
		r := events.ALBTargetGroupResponse{
			StatusCode: resp.StatusCode,
			Body: respBody,
			IsBase64Encoded: true,
			MultiValueHeaders: map[string][]string{},
		}

		for k, v := range responseHeaders {
			r.MultiValueHeaders[k] = append(r.MultiValueHeaders[k], v)
		}

		for _, s := range responseCookies {
			debug("Adding cookie: " + s)
			r.MultiValueHeaders["Set-Cookie"] = append(r.MultiValueHeaders["Set-Cookie"], s)
		}

		jsonResponse, _ := json.Marshal(r)
		debug(string(jsonResponse))
		return jsonResponse, nil
	}
}

func main() {
	ApplicationExec = os.Getenv("REWEB_APPLICATION_EXEC")
	ApplicationPort = os.Getenv("REWEB_APPLICATION_PORT")
	ForceGzip = (os.Getenv("REWEB_FORCE_GZIP") != "")
	WaitPath = os.Getenv("REWEB_WAIT_PATH")
	WaitCode = os.Getenv("REWEB_WAIT_CODE")
	Debug = (os.Getenv("REWEB_DEBUG") != "")

	if WaitPath == "" { WaitPath = "/" }

	if ApplicationExec == "" { panic("Missing REWEB_APPLICATION_EXEC environment variable") }
	if ApplicationPort == "" { panic("Missing REWEB_APPLICATION_PORT environment variable") }

	debug("Version: " + Version)

	debug("ENVIRONMENT:")
	for _, e := range os.Environ() {
		debug("         ... " + e)
	}

	exec := syscall.ProcAttr{
		Env: os.Environ(),
		Files: []uintptr{os.Stdin.Fd(), os.Stdout.Fd(), os.Stderr.Fd()},
	}

	syscall.ForkExec("/bin/sh", []string{"/bin/sh", "-c", ApplicationExec + " 2>&1"}, &exec)

	tries := 0
	for true {
		httpClient := &http.Client{
			Timeout: 2 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}

		resp, err := httpClient.Get("http://localhost:" + ApplicationPort + WaitPath)
		if err != nil {
			if tries % 10 == 0 {
				fmt.Printf(PrefixInfo + "SERVICE NOT UP: %v\n", err)
			}
		} else {
			if WaitCode == "" || fmt.Sprint(resp.StatusCode) == WaitCode {
				fmt.Printf(PrefixInfo + "SERVICE UP: %s\n", resp.Status)
				break
			}

			status := resp.Status
			if resp.StatusCode == 302 {
				status = fmt.Sprintf(PrefixInfo + "%s, Location: %s", resp.Status,
					resp.Header.Get("Location"))
			}

			fmt.Printf(PrefixInfo + "SERVICE UP, NOT READY: Expecting HTTP %s, got: %s\n",
				WaitCode, status)
		}

		time.Sleep(50 * time.Millisecond)

		tries += 1
	}

	lambda.StartHandler(lambdaHandler{})
	// UNREACH
}

