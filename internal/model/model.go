package model

type URL struct {
	ID       string `json:"id"`
	Original string `json:"original"`
	Short    string `json:"short"`
}

type ShortenRequest struct {
	URL string `json:"url" binding:"required"`
}

type ShortenResponse struct {
	Result string `json:"result"`
}
