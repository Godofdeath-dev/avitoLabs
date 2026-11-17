# TripGo — лабораторные работы 1 и 2

Сервис создаёт, возвращает и завершает поездки. Контракт находится в
`contracts/openapi/trip-service.openapi.yaml`; лабораторные 3–5 пока не реализованы.

## Запуск

Нужны Go 1.26.4+, golangci-lint v2, Docker и tripgoctl. Для Makefile нужны GNU Make
и Bash. Для `-race` на Windows нужен GCC x86-64 и `CGO_ENABLED=1`.

```bash
tripgoctl cluster start
tripgoctl environment start
tripgoctl connect
make migrate
make run
```

`environment.toml` включает PostgreSQL и observability для лабораторной 2.
`tripgoctl` создаёт локальный `.env`; адреса UI берите из `tripgoctl connect`.
`.env` и `.tripgo/` не коммитятся. Переменные с примерами перечислены в `.env.example`.
Без tripgoctl скопируйте `.env.example` в `.env` и укажите свои адреса.

В PowerShell загрузить `.env` в окружение текущего процесса можно так:

```powershell
Get-Content .env | ForEach-Object {
    if ($_ -match '^([A-Z][A-Z0-9_]*)=(.*)$') {
        [Environment]::SetEnvironmentVariable($matches[1], $matches[2].Trim('"'), 'Process')
    }
}
go tool goose -dir migrations postgres "$env:DATABASE_URL" up
go run ./cmd/trip-service
```

## Конфигурация

Обязательны `HTTP_ADDR` и `DATABASE_URL`. Остальные настройки имеют значения
по умолчанию из `.env.example`: уровень логов, бюджет остановки, размеры пула,
таймауты соединения/запросов БД и чтения/записи HTTP. Нулевые и отрицательные
таймауты, некорректные размеры пула отклоняются при старте.

Трейсы и метрики экспортируются по OTLP HTTP/protobuf. Используются стандартные
`OTEL_SERVICE_NAME`, `OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_RESOURCE_ATTRIBUTES`,
`OTEL_TRACES_SAMPLER`; интервалы и таймауты также задаются стандартными OTel env.
Без endpoint SDK создаёт локальные trace_id, но не отправляет телеметрию.
Недоступный коллектор не препятствует обработке HTTP-запросов.

## Архитектура

`main` читает конфигурацию, запускает контейнер и организует остановку по SIGINT/SIGTERM.
`internal/container` вручную собирает зависимости, создаёт пул, репозиторий,
usecase и HTTP-обработчики. DI-фреймворка нет.

- `domain/trip`: правила создания и завершения, типизированная валидация и доменные ошибки.
- `model`: входные данные usecase.
- `usecase/trip/{create,get,finish}`: сценарии и транзакционные границы.
- `http/trip/{create,get,finish}`: JSON, параметры пути и ответы по контракту.
- `http/domainerror`: единое преобразование доменных ошибок в Problem Details.
- `repository/trip`: параметризованный SQL и преобразование записей в доменные модели.
- `infra/postgres`: пул, инструментирование otelpgx и менеджер транзакций.
- `infra/telemetry`: OTel SDK, метрики и slog JSON с корреляцией трейсов.

Транспорт зависит от интерфейсов usecase, usecase — от интерфейсов репозитория и
менеджера транзакций. Интерфейсы объявляет потребитель в своём `contract.go`;
моки генерируются рядом в `mocks_test.go`. Домен не знает про HTTP, БД и OTel.
Правила импортов проверяет depguard.

Перед цепочкой middleware отдельный адаптер находит шаблон маршрута через
`chi.Match`. Он использует отдельный контекст и не меняет состояние самого роутера.
Это позволяет увеличить активные запросы с правильным `http.route` до обработки,
а уменьшить с теми же метками в `defer`. Основной порядок сохраняется:
`Recover → Tracing → Logging → Metrics → роутер`.

`otelpgx` создаёт SQL-спаны в пуле, адаптер добавляет им имя таблицы и тип ошибки,
а также измеряет запросы БД. Параметры SQL не записываются в телеметрию.
Логи запросов содержат шаблон маршрута, статус и время, тела запросов не логируются.
Создание/завершение поездки логируются и считаются только после commit.

## Проверки

```bash
make generate
make lint
make test
make test-integration
```

`make test` запускает unit-тесты с `-race -count=2` и печатает покрытие.
Интеграционные тесты отделены тегом `integration`, используют PostgreSQL из `.env`,
создают уникальные записи и удаляют только свои данные. Проверяются успешный цикл
поездки, занятый водитель, освобождение водителя и откат обеих таблиц при ошибке истории.
Без `DATABASE_URL` интеграционные тесты пропускаются; это не подтверждает работу БД.

Эквивалентные команды для PowerShell после загрузки `.env`:

```powershell
$env:CGO_ENABLED = '1'
# Укажите путь к своему GCC, если его нет в PATH:
$env:CC = 'C:\msys64\mingw64\bin\gcc.exe'
go generate ./...
golangci-lint run
go test -race -count=2 '-coverprofile=coverage.out' ./...
go tool cover '-func=coverage.out'
go test -race -count=2 -tags=integration ./...
```

Для проверки телеметрии создайте и завершите поездку, найдите её `trace_id` в
JSON-логе и откройте этот трейс в Jaeger. В Grafana используйте источник Prometheus:
`http_server_request_duration_seconds_count`, `http_server_active_requests`,
`db_client_operation_duration_seconds_count`, `tripgo_trips_created_total`,
`tripgo_trips_completed_total`. Скриншоты Jaeger и Grafana нужно приложить к PR.

Результаты проверки и скриншоты: [docs/lab2/verification.md](docs/lab2/verification.md).
