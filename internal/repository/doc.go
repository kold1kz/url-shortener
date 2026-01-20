// Package repository содержит реализации слоя хранения данных (URLRepository).
//
// Пакет предоставляет несколько реализаций:
//   - InMemoryURLRepository — хранение в памяти процесса;
//   - FileURLRepository — хранение в JSON-файле;
//   - PostgresURLRepository — хранение в PostgreSQL.
//
// Репозитории потокобезопасны и возвращают nil,nil при отсутствии записи.
package repository
