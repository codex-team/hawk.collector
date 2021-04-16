package alerts

import (
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
)

func Notify(addr string, message string) error {
	body := url.Values{}
	body.Set("message", message)
	body.Set("parse_mode", "Markdown")

	client := &http.Client{}
	r := strings.NewReader(body.Encode())
	req, err := http.NewRequest(http.MethodPost, addr, r)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	_, err = ioutil.ReadAll(res.Body)
	return err
}
