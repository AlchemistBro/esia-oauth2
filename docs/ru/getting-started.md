# Начало работы

> [К оглавлению](./index.md)

Проект принимает начало OAuth flow, перенаправляет браузер в ЕСИА, обрабатывает
callback, выполняет Token Exchange и проверяет полученный ID token. Он не
создаёт пользовательскую сессию приложения и не возвращает токены браузеру.

## Требования

- Linux и Go версии из `go.mod`;
- CryptoPro CSP с утилитой `csptest`;
- зарегистрированная тестовая ИС ЕСИА;
- клиентский сертификат и связанный private key container;
- доверенный публичный сертификат TESIA;
- HTTPS reverse proxy для браузерного flow;
- Docker и Docker Compose — при контейнерном запуске.

Private key container, PIN, сертификаты и лицензированные CryptoPro packages
нужно получить и хранить отдельно от репозитория.

## Подготовка

1. Клонируйте репозиторий:

   ```bash
   git clone <your-repository-url>
   cd esia
   ```

2. Создайте локальную конфигурацию:

   ```bash
   cp config.example.json config.json
   ```

3. Замените placeholders в `config.json`. Поля перечислены в
   [описании конфигурации](./configuration.md).

4. Разместите публичный сертификат TESIA по пути из
   `esia_response_cert_path`.

5. Установите CryptoPro CSP, импортируйте клиентский сертификат и сделайте
   private key container доступным пользователю процесса. Детали зависят от
   лицензированной поставки и описаны в
   [разделе о CryptoPro](./cryptopro.md).

## Локальный запуск

```bash
go run . -config ./config.json
```

Прямой HTTP listener предназначен для внутренней сети. Для браузерной проверки
сначала настройте HTTPS reverse proxy, затем откройте:

```text
https://service.example.com/auth/esia/login
```

При успешном flow callback возвращает HTTP 200 и текст
`авторизация через ЕСИА завершена`.

## Следующие шаги

- [понять последовательность запросов](./architecture.md);
- [подготовить Docker-развёртывание](./deployment.md);
- [разобрать типовые ошибки](./troubleshooting.md).
