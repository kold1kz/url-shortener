package pprof

import (
	"bytes"
	"fmt"
	"net/http"
)

func workload() {
	url := "http://localhost:8080/api/shorten"
	url2 := "http://localhost:8080/api/user/urls"

	for i := 0; i < 1000; i++ {
		// Тело запроса
		body := fmt.Sprintf("{\"url\": \"https://example%d.com\"}", i)
		jsonBody := []byte(body)

		// Создаем запрос с телом
		req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
		if err != nil {
			fmt.Println(err)
			continue
		}

		// Добавляем заголовки
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Cookie", "auth=7f7f01ff1d833fe2c862e1c444cf8d55%3A5c97f86d6811dcd1b0bc919e5f28d44d46ce702d96f6f00ee1420b077b53ad75")

		// Выполняем запрос
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			fmt.Println(err)
			continue
		}

		// Читаем ответ
		bs := make([]byte, 1024)
		for {
			n, err := resp.Body.Read(bs)
			fmt.Print(string(bs[:n]))

			if n == 0 || err != nil {
				break
			}
		}

		resp.Body.Close()
	}

	req, err := http.NewRequest("GET", url2, nil)
	if err != nil {
		fmt.Println(err)
	}

	// Добавляем заголовки
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Cookie", "auth=7f7f01ff1d833fe2c862e1c444cf8d55%3A5c97f86d6811dcd1b0bc919e5f28d44d46ce702d96f6f00ee1420b077b53ad75")

	// Выполняем запрос
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println(err)
	}

	// Читаем ответ
	bs := make([]byte, 1024)
	for {
		n, _ := resp.Body.Read(bs)
		fmt.Print(string(bs[:n]))
	}
	resp.Body.Close()
	return
}
