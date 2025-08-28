# MarketFlow - Real-Time Market Data Processing System

## Описание

MarketFlow - это система обработки рыночных данных в реальном времени, построенная на Go с использованием принципов шестиугольной архитектуры и паттернов конкурентности.

## Функциональность

- **Режимы работы**: Live (реальные данные с бирж) и Test (синтетические данные)
- **Конкурентная обработка**: Worker Pool, Fan-In, Fan-Out паттерны
- **Кэширование**: Redis для быстрого доступа к последним ценам
- **Хранение данных**: PostgreSQL для агрегированных данных
- **REST API**: Полный набор endpoints для получения рыночных данных
- **Graceful Shutdown**: Корректное завершение работы

## Архитектура

Проект следует принципам гексагональной архитектуры:

- **Domain Layer**: Бизнес-логика и модели
- **Application Layer**: Use cases и оркестрация
- **Adapters**: Интеграции (PostgreSQL, Redis, HTTP, Exchange)

## Установка и запуск

1. Установите зависимости:
```bash
go mod tidy
```

2. Настройте базы данных:
```bash
make docker-setup
```

3. Скомпилируйте проект:
```bash
make build
```

4. Запустите приложение:
```bash
./marketflow --port 8080
```

## API Endpoints

### Цены
- `GET /prices/latest/{symbol}` - Последняя цена
- `GET /prices/latest/{exchange}/{symbol}` - Последняя цена с конкретной биржи
- `GET /prices/highest/{symbol}?period=1m` - Максимальная цена за период
- `GET /prices/lowest/{symbol}?period=1m` - Минимальная цена за период
- `GET /prices/average/{symbol}?period=1m` - Средняя цена за период

### Управление режимами
- `POST /mode/test` - Переключение в тестовый режим
- `POST /mode/live` - Переключение в режим реальных данных

### Здоровье системы
- `GET /health` - Статус системы

## Паттерны конкурентности

1. **Generator**: Генерация синтетических данных в тестовом режиме
2. **Fan-Out**: Распределение обновлений между воркерами
3. **Worker Pool**: Пул воркеров для обработки данных (по 5 на биржу)
4. **Fan-In**: Агрегация данных от всех источников

## Конфигурация

Настройка через файл `config.yaml`:

```yaml
server:
  port: 8080

database:
  host: localhost
  port: 5432
  user: marketflow
  password: password
  dbname: marketflow
  sslmode: disable

redis:
  host: localhost
  port: 6379
  password: ""
  db: 0
  ttl: 300s

exchanges:
  - name: "exchange1"
    host: "127.0.0.1"
    port: 40101
  # ...
```

## Особенности реализации

- **Failover**: Автоматическое переподключение к биржам
- **Батчинг**: Группировка записей для эффективной работы с БД
- **Fallback**: PostgreSQL продолжает работать даже при недоступности Redis
- **Очистка данных**: Автоматическое удаление устаревших данных из кэша
- **Graceful Shutdown**: Корректное завершение всех горутин