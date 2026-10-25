# Changelog

Формат основан на [Keep a Changelog](https://keepachangelog.com/ru/1.1.0/),
версии следуют [Semantic Versioning](https://semver.org/lang/ru/).

## [1.0.0] - 2026-10-25

### Добавлено

- Запрос согласия `consent.permissions` (ЕСИА v2) и получение ФИО и email
  через REST API ЕСИА (`/rs/prns/{oid}`, `/rs/prns/{oid}/ctts`).
- Настройка `oauth.scopes` с проверкой допустимых значений.
- Обёртка `deploy/csptest-wrapper.sh`: подставляет PIN контейнера ключа в
  команды подписи.
- Аргумент сборки `CPRO_LICENSE` для серверной лицензии КриптоПро CSP.
- Hardened `compose.example.yaml`: `cap_drop: ALL`, `no-new-privileges`,
  лимиты памяти и процессов, `tmpfs` для `/tmp`.
- Схемы C4 (контекст, контейнеры, компоненты, развёртывание) и UML-диаграмма
  последовательности входа, исходники в draw.io.
- Тесты хранилища state: повтор, чужой браузер, истечение срока, перезапись.
- Тест, который держит `config.example.json` валидным.

### Изменено

- ГОСТ-подпись ID token разворачивается в порядок байтов, который ожидает
  `csptest`, перед проверкой.
- Обход `permissions=null` больше не мешает настоящему запросу согласия.
- Заголовок `sbt` в ID token больше не обязателен.
- Документация дополнена разделами о PIN, правах на ключ, лицензии CryptoPro
  и типовых ошибках в Docker.

[1.0.0]: https://github.com/AlchemistBro/esia-oauth2/releases/tag/v1.0.0
