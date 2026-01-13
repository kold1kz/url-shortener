package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
)

func main() {
	url := "http://localhost:8080/api/shorten"
	url2 := "http://localhost:8080/api/user/urls"

	cookie := "auth=7f7f01ff1d833fe2c862e1c444cf8d55%3A5c97f86d6811dcd1b0bc919e5f28d44d46ce702d96f6f00ee1420b077b53ad75"

	for i := 0; i < 1000; i++ {
		body := fmt.Sprintf("{\"url\": \"https://example%d.com\"}", i)

		req, err := http.NewRequest(http.MethodPost, url, bytes.NewBufferString(body))
		if err != nil {
			fmt.Println(err)
			continue
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Cookie", cookie)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			fmt.Println(err)
			continue
		}

		// гарантируем закрытие
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}

	req, err := http.NewRequest(http.MethodGet, url2, nil)
	if err != nil {
		fmt.Println(err)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Cookie", cookie)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer resp.Body.Close()

	// читаем до EOF и выходим
	_, _ = io.Copy(io.Discard, resp.Body)
}
