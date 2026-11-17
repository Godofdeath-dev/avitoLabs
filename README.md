# TripGo — лабораторные работы 1–3

Сервис создаёт и завершает поездки, сохраняет координаты и возвращает маршрут.
В фоне запрашивает координаты у устройств через Push Service. Контракт находится
в `contracts/openapi/trip-service.openapi.yaml`. Лабораторные 4–5 пока не реализованы.

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

`environment.toml` включает PostgreSQL, observability и Push Service для лабораторной 3.
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

Обязательны `HTTP_ADDR`, `DATABASE_URL` и `PUSH_HTTP_URL`. Остальные настройки имеют значения
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

`make test` запускает unit- и интеграционные тесты с `-race -count=2` и печатает покрытие.
Перед запуском поднимите окружение и выполните `make migrate`.
Интеграционные тесты отделены тегом `integration`, используют PostgreSQL из `.env`,
создают уникальные записи и удаляют только свои данные. Проверяются успешный цикл
поездки, занятый водитель, освобождение водителя и откат обеих таблиц при ошибке истории.
Цели Makefile загружают `.env` и завершаются с ошибкой, если `DATABASE_URL` не задан.
При прямом запуске `go test -tags=integration` без этой переменной проверки БД пропускаются.

Эквивалентные команды для PowerShell после загрузки `.env`:

```powershell
$env:CGO_ENABLED = '1'
# Укажите путь к своему GCC, если его нет в PATH:
$env:CC = 'C:\msys64\mingw64\bin\gcc.exe'
go generate ./...
golangci-lint run
go test -race -count=2 -tags=integration '-coverprofile=coverage.out' ./...
go tool cover '-func=coverage.out'
go test -race -count=2 -tags=integration ./...
```

Для проверки телеметрии создайте и завершите поездку, найдите её `trace_id` в
JSON-логе и откройте этот трейс в Jaeger. В Grafana используйте источник Prometheus:
`http_server_request_duration_seconds_count`, `http_server_active_requests`,
`db_client_operation_duration_seconds_count`, `tripgo_trips_created_total`,
`tripgo_trips_completed_total`. Скриншоты Jaeger и Grafana нужно приложить к PR.

Результаты проверки и скриншоты: [docs/lab2/verification.md](docs/lab2/verification.md).

## Опрос устаревших поездок

Схема: сканер по тикеру → ограниченный буферизованный канал → фиксированные воркеры → HTTP Push Service.
Компонент находится в `internal/worker/positions`, клиент — в `internal/gateway/push`.
Нужна релизная сборка tripgoctl со встроенным Push Service (проверено на v0.1.2).
Обычная сборка CLI без embedded-файлов не может поднять заглушку. Установка описана
в [репозитории инфраструктуры](https://github.com/course-go-autumn-2026/course-infra).

| Переменная | По умолчанию | Назначение |
|---|---|---|
| POSITION_CHECK_INTERVAL | 30s | Период обхода БД; повторные запросы по поездке не чаще тиков |
| POSITION_STALE_AFTER | 60s | Возраст координаты, после которого нужен опрос |
| POSITION_WORKERS | 4 | Число постоянно работающих воркеров |
| POSITION_QUEUE_SIZE | 100 | Ёмкость канала задач |
| PUSH_HTTP_URL | обязательна, из .env | Базовый URL Push Service |
| PUSH_REQUEST_TIMEOUT | 1s | Таймаут обработки задачи и HTTP-клиента |
| SHUTDOWN_TIMEOUT | 10s | Общий бюджет остановки всего сервиса |

Сканер выполняет один SQL без LIMIT: только активные поездки, у которых
COALESCE(last_position_at, started_at) старше заданного порога. Сравнение времени
выполняется в SQL. Сначала идут самые давно не обновлявшиеся поездки.
Составной индекс включает status, выражение COALESCE и id. Индексы создаются
CONCURRENTLY отдельными миграциями; EXPLAIN проверяется на реальных тестовых данных.

Если очередь полна, сканер не ждёт: задача отбрасывается, пишется Warn и увеличиваются
stale_checks{result=dropped} и position_requests{result=dropped}. На следующем тике
поездка снова может попасть в выборку. Ошибки Push Service не останавливают воркер.
Ретраев и rate limiter нет; время отправки команд не хранится.

Отдельная защищённая mutex структура учитывает ожидающие и выполняемые задачи.
После успешного commit координаты сценарий отменяет задачу поездки. Запись в канале
может остаться до чтения: воркер проверяет её актуальность и не отправляет отменённый
пуш. Для уже выполняемого запроса отменяется контекст. При завершении старая задача
удаляет только собственную запись, чтобы не затереть более новую задачу той же поездки.

POST /api/v1/trips/{tripId}/positions сохраняет точку и обновляет last_position_at
одной транзакцией. Строка поездки блокируется FOR UPDATE, чтобы сохранение координаты
не прошло одновременно с завершением. GREATEST в SQL защищает last_position_at от
опоздавших точек. Отмена задачи и счётчик сохранённых координат происходят после commit.
GET /api/v1/trips/{tripId}/positions возвращает все точки в порядке recorded_at, id;
пустой маршрут — positions: [], отсутствующая поездка — 404.

Порядок остановки виден в container.Shutdown: HTTP дожидается текущих запросов,
останавливается сканер, воркеры дочитывают канал, закрываются HTTP-клиент и пул БД.
При исчерпании общего бюджета отменяются контексты запросов воркеров.
Очередь хранится в памяти; после перезапуска задачи восстанавливаются следующей
выборкой из БД. Долговечного брокера в этой работе нет.

Метрики обязательной части:
- tripgo.positions.saved;
- tripgo.stale_checks{result=queued|duplicate|dropped};
- tripgo.position_requests.queue.size;
- tripgo.position_requests.workers.active;
- tripgo.position_requests{result=success|error|dropped};
- tripgo.position_requests.duration{result=success|error}.

Размер очереди и занятость воркеров — observable gauges, читающие состояние пула.
HTTP-вызов Push Service — дочерний спан обработки задачи; обработка начинает новый
трейс со span link на проверку сканера. Контекст передаётся Push Service через traceparent.

В тестах тикер и время подменяются clockwork, внешние зависимости — gomock.
Пакет воркеров проверяет утечки через goleak.VerifyTestMain. Проверяются переполнение,
повторная постановка, отмена, ошибки соседа, отсутствие параллельных обходов,
доработка очереди и отмена по бюджету остановки.

План: [docs/lab3/plan.md](docs/lab3/plan.md).
Результаты проверок и скриншот нагрузки: [docs/lab3/verification.md](docs/lab3/verification.md).
