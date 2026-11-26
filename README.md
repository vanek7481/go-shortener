# Сервис сокращения ссылок на Go

Учебный HTTP-сервис на стандартной библиотеке Go, без внешних зависимостей и базы данных. Нужен **Go 1.22 или новее**.

## Запуск

Откройте терминал в корне проекта — в папке, где лежит `go.mod`:

```sh
go run ./cmd/shortener
```

Сервер слушает порт 8080. Адрес: `http://localhost:8080`. Остановка: Ctrl+C.
Все ссылки хранятся в памяти и исчезают после перезапуска процесса.

## Проверка

```sh
go test ./...
go vet ./...
```

Дополнительно можно выполнить `go test -race ./...` (на Windows для этого нужен C-компилятор с поддержкой Go race detector).

Тесты на `net/http/httptest` проверяют создание ссылок, редирект 307, неизвестные ID, подпись cookie, замену поддельной cookie, изоляцию пользователей, ответы 204/401, неправильный JSON, URL и HTTP-методы. Отдельный тест проверяет конкурентный доступ к хранилищу и отсутствие коллизий.

## API

| Запрос | Результат |
| --- | --- |
| `POST /api/shorten` с `{"url":"https://example.com/long/path"}` | `200 OK`, JSON `{"result":"http://localhost:8080/1"}` |
| `GET /1` | `307 Temporary Redirect`, заголовок `Location` с исходным URL |
| `GET /unknown` | `404 Not Found` |
| `GET /api/user/urls` с cookie пользователя | `200 OK`, массив объектов `short_url` и `original_url` |
| `GET /api/user/urls` для пользователя без ссылок | `204 No Content`, без тела |
| `GET /api/user/urls` с пустой или некорректно сформированной cookie | `401 Unauthorized` |

Принимаются абсолютные URL с протоколом HTTP или HTTPS. Некорректные запросы сокращения получают 400; неподдерживаемые методы — 405 с заголовком `Allow`. Размер тела ограничен 1 МиБ. Повторное сокращение одного URL создаёт новую ссылку.

### Примеры (Bash)

```sh
curl -i -c cookies.txt -H 'Content-Type: application/json' -d '{"url":"https://example.com/some/long/path"}' http://localhost:8080/api/shorten
curl -i -b cookies.txt http://localhost:8080/api/user/urls
curl -i http://localhost:8080/1
```

### Примеры (PowerShell)

```powershell
$body = @{ url = 'https://example.com/some/long/path' } | ConvertTo-Json
$link = Invoke-RestMethod -Method Post -Uri 'http://localhost:8080/api/shorten' -ContentType 'application/json' -Body $body -SessionVariable session
$link.result
Invoke-RestMethod -Uri 'http://localhost:8080/api/user/urls' -WebSession $session
```

Откройте значение `$link.result` в браузере, чтобы проверить перенаправление.

## Идентификация пользователя

Cookie `user_id` содержит `userID:signature`. ID генерируется из 16 криптографически случайных байтов, подпись — HMAC-SHA256 в hex. Подписи сравниваются через `hmac.Equal`.

Если cookie отсутствует или её подпись не совпадает, создаётся новая идентичность. На `/api/user/urls` это означает 204. Пустое значение, отсутствие ID, неверное число частей, пустая подпись или подпись неправильного hex-формата/длины на этом маршруте означают 401. На `/api/shorten` такие cookie заменяются новыми.

Cookie имеет `Path=/`, `HttpOnly`, `SameSite=Lax`; при HTTPS добавляется `Secure`. Секрет `super-secret-key` находится в `cmd/shortener/main.go`, что разрешено условиями учебного задания. Для реального сервиса секрет нужно вынести из исходников.

## Структура

- `cmd/shortener/main.go` — точка входа и настройки HTTP-сервера.
- `internal/app` — маршруты, проверка запросов и HTTP-тесты.
- `internal/auth` — генерация ID и проверка подписанных cookie.
- `internal/storage` — хранилище, защищённое `sync.RWMutex`.

Идентификаторы ссылок генерируются общим счётчиком в системе счисления 36 под блокировкой. Сохранение ссылки и её привязка к пользователю выполняются атомарно. Список возвращается копией, чтобы вызывающий код не мог менять внутренние данные.

## Сдача

Загрузите содержимое этой папки в свой Git-репозиторий: `go.mod` должен находиться в корне репозитория. Передайте ревьюеру ссылку на репозиторий. Публикация не требуется для локального запуска и тестирования.
