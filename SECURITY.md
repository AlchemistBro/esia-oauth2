# Политика безопасности

## Сообщение об уязвимости

Не публикуйте сведения об уязвимости в обычном Issue. Используйте
[GitHub Private Security Advisory](https://github.com/AlchemistBro/esia/security/advisories/new)
и приложите минимальные шаги воспроизведения, ожидаемое влияние и затронутую
версию.

Не прикладывайте действующие credentials, OAuth tokens, cookies, CSP PIN,
private key containers, production-конфигурацию или реальные HAR-файлы.

## Область ответственности

Проект реализует OAuth callback flow и локальную проверку ID token. Настройка
CryptoPro CSP, защита host, TLS termination, управление secrets и выпуск
сертификатов остаются ответственностью оператора.

Подробная модель угроз и ограничения описаны в
[docs/ru/security.md](./docs/ru/security.md).
