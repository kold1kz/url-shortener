// Package middleware содержит HTTP-middleware для сервиса сокращения URL.
//
// Включает:
//   - логирование запросов/ответов,
//   - поддержку gzip (ответ и опционально распаковка gzip-запроса),
//   - cookie-based идентификацию пользователя.
package middleware
