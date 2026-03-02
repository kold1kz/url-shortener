// Package main предоставляет утилиту staticlint — multichecker для статического анализа проекта.
//
// Запуск:
//
//	go run ./cmd/staticlint ./...
//
// или собрать бинарник:
//
//	go build -o staticlint ./cmd/staticlint
//	./staticlint ./...
//
// Состав проверок:
//
//  1. Анализаторы из golang.org/x/tools/go/analysis/passes:
//     printf, shadow, structtag, lostcancel, loopclosure, copylock, unmarshal, unreachable и др.
//     Они ловят типовые ошибки (форматирование, shadowing, теги, утечки cancel, баги с замыканиями и т.п.)
//
//  2. Все анализаторы класса SA из staticcheck (SAxxxx):
//     неправильное использование стандартных библиотек, ошибочные конструкции и т.п.
//
//  3. Минимум один анализатор других классов staticcheck:
//     simple (Sxxxx) и stylecheck (STxxxx) — упрощения и стиль.
//
//  4. Дополнительные публичные анализаторы:
//     - go-critic (набор проверок на качество кода)
//     - bodyclose (проверяет корректное закрытие HTTP response body)
//
//  5. Собственный анализатор noosexit:
//     запрещает прямой вызов os.Exit в функции main пакета main.
//     Причина: os.Exit не выполняет defer, что может ломать закрытие ресурсов/flush логов.
//
// Требование инкремента: исходный код проекта должен проходить анализ данным multichecker.
package main
