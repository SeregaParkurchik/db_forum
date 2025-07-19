# Используем многостадийную сборку для уменьшения размера итогового образа
# Этап сборки Go-приложения
FROM golang:1.24.1 AS builder

WORKDIR /app

# Копируем go.mod и go.sum, чтобы кэшировать зависимости
COPY go.mod go.sum ./
RUN go mod download

# Копируем остальной исходный код
COPY . .

# Собираем приложение
# Отключаем CGO, чтобы не зависеть от системных библиотек, что делает образ более переносимым
# Собираем бинарник в /app/main
# Убедитесь, что путь к main.go правильный (он у вас в cmd/main.go)
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/forum-server ./cmd/main.go

# Этап финального образа с PostgreSQL и вашим приложением
# Используем официальный образ PostgreSQL в качестве основы
FROM postgres:16-alpine

# Копируем собранный бинарник из предыдущего этапа
COPY --from=builder /app/forum-server /usr/local/bin/forum-server

# Копируем скрипт инициализации БД
# Это путь по умолчанию, где PostgreSQL ищет скрипты инициализации при первом запуске
COPY migrations/init.sql /docker-entrypoint-initdb.d/

# Устанавливаем переменные окружения для PostgreSQL (соответствуют вашему docker-compose)
ENV POSTGRES_DB=dbhw \
    POSTGRES_USER=admin \
    POSTGRES_PASSWORD=123456

# Устанавливаем переменные окружения для Go-приложения
# Поскольку БД и приложение в одном контейнере, host для подключения к БД будет 'localhost' или '127.0.0.1'
# Порт PostgreSQL по умолчанию - 5432
ENV PG_DSN="host=localhost port=5432 user=${POSTGRES_USER} password=${POSTGRES_PASSWORD} dbname=${POSTGRES_DB} sslmode=disable" \
    HTTP_HOST="0.0.0.0" \
    HTTP_PORT="5000"

# Выставляем порт, на котором будет доступно ваше API (5000)
# И порт PostgreSQL (5432)
EXPOSE 5000
EXPOSE 5432

USER postgres
# Команда для запуска: запускаем PostgreSQL в фоновом режиме, затем ваше приложение.
# Используем bash для последовательного выполнения команд.
# Важно: `pg_isready` ждет, пока PostgreSQL будет готов принимать соединения.
CMD sh -c "(docker-entrypoint.sh postgres &) && \
           until pg_isready -h localhost -p 5432 -U ${POSTGRES_USER}; do \
               echo 'Waiting for PostgreSQL to be ready...'; \
               sleep 1; \
           done; \
           echo 'PostgreSQL is ready. Starting forum-server...'; \
           /usr/local/bin/forum-server"