package healthchecker

import (
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/coredns/coredns/plugin"
)

type (
	Checker interface {
		Check(record string) bool
	}

	HttpChecker struct {
		client *http.Client
		port   string
	}
)

const (
	defaultHttpPort    = "80"
	defaultHttpTimeout = 2 * time.Second
)

func NewHttpChecker(httpParams string) (*HttpChecker, error) {
	errMsg := "the following format is supported: http OR http:PORT OR http:PORT:TIMEOUT_IN_MS"

	params := strings.Split(httpParams, ":")
	if len(params) > 3 || params[0] != "http" {
		return nil, plugin.Error(pluginName, fmt.Errorf("invalid http method: %s, %s", params[0], errMsg))
	}

	var (
		port    = defaultHttpPort
		timeout = defaultHttpTimeout
	)

	// parsing port
	if len(params) > 1 {
		num, err := strconv.Atoi(params[1])
		if err != nil || num <= 0 {
			return nil, plugin.Error(pluginName, fmt.Errorf("invalid port: %s, %s", params[1], errMsg))
		}
		port = params[1]

	}
	// parsing timeout
	if len(params) > 2 {
		num, err := strconv.ParseInt(params[2], 10, 64)
		if err != nil || num <= 0 {
			return nil, plugin.Error(pluginName, fmt.Errorf("invalid timeout: %s, %s", params[2], errMsg))
		}
		timeout = time.Duration(num) * time.Millisecond
	}

	client := &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Timeout: timeout,
	}

	return &HttpChecker{
		client: client,
		port:   port,
	}, nil
}

func (h HttpChecker) Check(endpoint string) bool {
	response, err := h.client.Get("http://" + net.JoinHostPort(endpoint, h.port))
	if err != nil {
		log.Debugf(err.Error())
		return false
	}
	_ = response.Body.Close()

	return response.StatusCode < http.StatusInternalServerError
}
