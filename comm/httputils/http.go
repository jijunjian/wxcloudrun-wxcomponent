package httputils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"github.com/WeixinCloud/wxcloudrun-wxcomponent/comm/config"
	"github.com/WeixinCloud/wxcloudrun-wxcomponent/comm/log"
)

// PostJson 发起post请求，格式为JSON
func PostJson(url string, data interface{}) ([]byte, error) {
	jsonByte, _ := json.Marshal(data)
	return Post(url, jsonByte, "application/json")
}

// PostFormData 发起post请求，格式为Form-Data
func PostFormData(url string, formFile multipart.File, fileName string, fieldName string) ([]byte, error) {
	client := &http.Client{Timeout: 20 * time.Second}
	log.Debugf("[post]http url: %s", safeURL(url))

	buf := new(bytes.Buffer)
	w := multipart.NewWriter(buf)
	if createFormFile, err := w.CreateFormFile(fieldName, fileName); err == nil {
		readAll, _ := ioutil.ReadAll(formFile)
		createFormFile.Write(readAll)
	}
	w.Close()
	req, err := http.NewRequest(http.MethodPost, url, buf)
	if err != nil {
		log.Error(err)
		return nil, err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("User-Agent", "WxComponent/"+config.CommConf.Version)
	resp, err := client.Do(req)
	if err != nil {
		log.Error(err)
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http code: %d", resp.StatusCode)
	}

	result, _ := ioutil.ReadAll(resp.Body)
	log.Debugf("[post]http response bytes=%d", len(result))
	return result, nil
}

// Post 发起post请求
func Post(url string, data []byte, contentType string) ([]byte, error) {
	_, body, err := RawPost(url, data, contentType)
	return body, err
}

// RawPost 发起post请求
func RawPost(url string, data []byte, contentType string) (*http.Response, []byte, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	log.Debugf("[post]http url: %s content-type: %s", safeURL(url), contentType)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(data))
	if err != nil {
		log.Error(err)
		return nil, nil, err
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("User-Agent", "WxComponent/"+config.CommConf.Version)
	started := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, logRequestError(req, started, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return resp, nil, upstreamStatusError(req, resp, started)
	}

	result, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return resp, nil, logRequestError(req, started, err)
	}
	if len(resp.Header["Content-Type"]) > 0 &&
		strings.Contains(strings.ToLower(resp.Header["Content-Type"][0]), "application/json") {
		log.Debugf("upstream request succeeded method=%s url=%s status=%d duration_ms=%d response_bytes=%d", req.Method, safeURL(url), resp.StatusCode, time.Since(started).Milliseconds(), len(result))
	} else {
		log.Debugf("[get]http resp Content-Type: %v", resp.Header["Content-Type"])
	}
	return resp, result, nil
}

// Get 发起get请求
func Get(url string) ([]byte, error) {
	_, body, err := RawGet(url)
	return body, err
}

// RawGet 发起get请求
func RawGet(url string) (*http.Response, []byte, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	log.Debugf("[get]http url: %s", safeURL(url))
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		log.Error(err)
		return nil, nil, err
	}
	req.Header.Set("User-Agent", "WxComponent/"+config.CommConf.Version)
	started := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, logRequestError(req, started, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return resp, nil, upstreamStatusError(req, resp, started)
	}
	result, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return resp, nil, logRequestError(req, started, err)
	}
	if len(resp.Header["Content-Type"]) > 0 &&
		strings.Contains(strings.ToLower(resp.Header["Content-Type"][0]), "application/json") {
		log.Debugf("upstream request succeeded method=%s url=%s status=%d duration_ms=%d response_bytes=%d", req.Method, safeURL(url), resp.StatusCode, time.Since(started).Milliseconds(), len(result))
	} else {
		log.Debugf("[get]http resp Content-Type: %v", resp.Header["Content-Type"])
	}
	return resp, result, nil
}
