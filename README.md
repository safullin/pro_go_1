# go-musthave-metrics-tpl

Шаблон репозитория для трека «Сервер сбора метрик и алертинга».

## Начало работы

1. Склонируйте репозиторий в любую подходящую директорию на вашем компьютере.
2. В корне репозитория выполните команду `go mod init <name>` (где `<name>` — адрес вашего репозитория на GitHub без префикса `https://`) для создания модуля.

## Обновление шаблона

Чтобы иметь возможность получать обновления автотестов и других частей шаблона, выполните команду:

```
git remote add -m v2 template https://github.com/Yandex-Practicum/go-musthave-metrics-tpl.git
```

Для обновления кода автотестов выполните команду:

```
git fetch template && git checkout template/v2 .github
```

Затем добавьте полученные изменения в свой репозиторий.

## Запуск автотестов

Для успешного запуска автотестов называйте ветки `iter<number>`, где `<number>` — порядковый номер инкремента. Например, в ветке с названием `iter4` запустятся автотесты для инкрементов с первого по четвёртый.

При мёрже ветки с инкрементом в основную ветку `main` будут запускаться все автотесты.

Подробнее про локальный и автоматический запуск читайте в [README автотестов](https://github.com/Yandex-Practicum/go-autotests).

## Информация о сборке

По умолчанию агент и сервер выводят `N/A` для версии, даты и коммита. Значения можно задать при компиляции через `-ldflags`:

```sh
go build -ldflags "-X main.buildVersion=v1.0.0 -X main.buildDate=2026-08-01 -X main.buildCommit=abcdef1" -o agent ./cmd/agent
go build -ldflags "-X main.buildVersion=v1.0.0 -X main.buildDate=2026-08-01 -X main.buildCommit=abcdef1" -o server ./cmd/server
```

## Структура проекта

Приведённая в этом репозитории структура проекта является рекомендуемой, но не обязательной.

Это лишь пример организации кода, который поможет вам в реализации сервиса.

При необходимости можно вносить изменения в структуру проекта, использовать любые библиотеки и предпочитаемые структурные паттерны организации кода приложения, например:
- **DDD** (Domain-Driven Design)
- **Clean Architecture**
- **Hexagonal Architecture**
- **Layered Architecture**

## Бенчмарки и профилирование

Бенчмарки измеряют пакетное обновление метрик в HTTP-хендлере и в памяти:

```sh
go test -run=^$ -bench='Benchmark(UpdateMetricsJSON|MemStorageUpdateMetrics)' -benchmem ./internal/handler ./internal/repository
```

Профили памяти сняты на бенчмарке `BenchmarkUpdateMetricsJSON` с реалистичным батчем из четырёх метрик:

```sh
go test -run=^$ -bench=^BenchmarkUpdateMetricsJSON$ -benchtime=5s -benchmem -memprofile=profiles/base.pprof -memprofilerate=1 ./internal/handler
go test -run=^$ -bench=^BenchmarkUpdateMetricsJSON$ -benchtime=5s -benchmem -memprofile=profiles/result.pprof -memprofilerate=1 ./internal/handler
```

Для анализа использовались `top`, `list`, `peek` и визуализация графа в `pprof`. Лишнее выделение среза имён метрик перенесено за проверку включённого аудита.

До оптимизации пакетный HTTP-хендлер создавал срез имён метрик даже при отключённом аудите и показывал `8050 B/op` и `51 allocs/op`. После переноса формирования среза в ветку с активным аудитом результат составил `7986 B/op` и `50 allocs/op`: на каждый запрос стало приходиться на одну аллокацию и 64 байта меньше. В сравнении профилей это видно как уменьшение `alloc_space` функции `metricNames` примерно на 4,8 МБ за время прогона.

```text
$ go tool pprof -top -diff_base=profiles/base.pprof profiles/result.pprof
File: handler.test
Type: alloc_space
Showing nodes accounting for 2473.20kB, 0.41% of 606685.54kB total
      flat  flat%   sum%        cum   cum%
-4822.44kB  0.79%  0.79% -4822.44kB  0.79%  github.com/safullin/pro_go_1/internal/handler.metricNames (inline)
```
