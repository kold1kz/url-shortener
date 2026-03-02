package model

// URL представляет сокращённый URL в системе.
//
// Используется как доменная модель в сервисе и репозиториях.
// Поле UserID не сериализуется в JSON-ответах.
// generate:reset
type URL struct {
	// ID — уникальный идентификатор URL.
	ID string `json:"id"`
	// Original — исходный (длинный) URL.
	Original string `json:"original"`
	// Short — сокращённый URL.
	Short string `json:"short"`
	// UserID — идентификатор пользователя-владельца.
	// Не возвращается клиенту.
	UserID string `json:"-"`
	// IsDeleted — признак логического удаления URL.
	IsDeleted bool `json:"is_deleted"`
}

// ShortenRequest описывает JSON-запрос
// на создание сокращённого URL.
// generate:reset
type ShortenRequest struct {
	// URL — исходный URL для сокращения.
	URL string `json:"url" binding:"required"`
}

// ShortenResponse описывает JSON-ответ
// с результатом сокращения URL.
// generate:reset
type ShortenResponse struct {
	// Result — сокращённый URL.
	Result string `json:"result"`
}

// BatchRequest описывает элемент batch-запроса
// на массовое сокращение URL.
// generate:reset
type BatchRequest struct {
	// CorrelationID — идентификатор запроса клиента,
	// используется для сопоставления ответа.
	CorrelationID string `json:"correlation_id"`

	// OriginalURL — исходный URL для сокращения.
	OriginalURL string `json:"original_url"`
}

// BatchResponse описывает элемент ответа
// batch-операции сокращения URL.
// generate:reset
type BatchResponse struct {
	// CorrelationID — идентификатор запроса клиента.
	CorrelationID string `json:"correlation_id"`

	// ShortURL — сокращённый URL.
	ShortURL string `json:"short_url"`
}

// UserURLResponse описывает URL,
// принадлежащий конкретному пользователю.
// generate:reset
type UserURLResponse struct {
	// ShortURL — сокращённый URL.
	ShortURL string `json:"short_url"`

	// OriginalURL — исходный URL.
	OriginalURL string `json:"original_url"`
}
