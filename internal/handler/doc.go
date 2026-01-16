// Package handler содержит HTTP-хендлеры сервиса сокращения URL.
//
// Хендлеры реализованы на gin и используют сервисный слой (service.URLService),
// а также (опционально) публикуют события аудита через audit.Publisher.
package handler
