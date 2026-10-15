# Project_template

Это шаблон для решения проектной работы. Структура этого файла повторяет структуру заданий. Заполняйте его по мере работы над решением.

# Задание 1. Анализ и планирование

### 1. Описание функциональности монолитного приложения

**Управление отоплением:**

- Пользователи могут просматривать текущее состояние работы отопительного оборудования (активно/неактивно, текущий статус сенсора и реле).
- Пользователи могут удалённо включать и выключать отопление в доме через вызов HTTP REST API (`PATCH /api/v1/sensors/:id/value` либо `PUT /api/v1/sensors/:id`).
- Система хранит информацию об отопительных устройствах в единой таблице базы данных (поля `id`, `name`, `type`, `location`, `value`, `unit`, `status`, `last_updated`, `created_at`).
- Система отправляет прямые синхронные команды на исполнительные механизмы (реле отопительного котла/нагревателя).
- Первичное подключение, настройка сетевых адресов и регистрация отопительного модуля в системе выполняются только вручную выездным сервисным инженером компании.

**Мониторинг температуры:**

- Пользователи могут удалённо просматривать текущую температуру в помещениях через веб-интерфейс:
  - получение общего списка датчиков со свежими показаниями (`GET /api/v1/sensors`);
  - получение данных по конкретному датчику (`GET /api/v1/sensors/:id`);
  - получение актуальной температуры по наименованию локации/комнаты (`GET /api/v1/sensors/temperature/:location`).
- Система поддерживает опрос физических температурных датчиков по синхронной pull-модели: при обработке входящего клиентского запроса монолит обращается к внешнему HTTP API датчика (`GET /temperature/{sensor_id}` или `GET /temperature?location={location}`), считывает показания и отдаёт их клиенту.
- Система фиксирует метку времени последнего обновления данных (`last_updated`) и время добавления сенсора (`created_at`).
- При недоступности физического датчика или сетевых сбоях возвращаются сохранённые в БД предыдущие значения либо ошибка обращения к внешнему сервису.

### 2. Анализ архитектуры монолитного приложения

- **Стек технологий и рантайм:**
  - Язык программирования: **Go 1.22**.
  - Веб-фреймворк: **Gin Web Framework** (`github.com/gin-gonic/gin`) для маршрутизации и обработки REST API.
  - Драйвер базы данных: **pgx v5** (`github.com/jackc/pgx/v5/pgxpool`) с пулом соединений к PostgreSQL.
  - Внешний HTTP-клиент: стандартный пакет `net/http` с жестко заданным таймаутом 10 секунд на вызов (`services/temperature_service.go`).
- **База данных и модель данных:**
  - СУБД: **PostgreSQL 16**.
  - Архитектура данных: предельно упрощенная, денормализованная плоская таблица `sensors`.
  - Индексы: простые B-tree индексы по полям `type`, `location`, `status`.
  - Отсутствие доменных связей: в базе данных нет таблиц пользователей (`users`), домов (`houses`), участков, комнат, прав доступа и журнала измерений телеметрии. В одной строке смешаны метаданные оборудования, его конфигурация и текущие измерения.
- **Архитектурный стиль:**
  - Классический трехуровневый монолит (Monolithic Layered Architecture) в рамках одного процесса и бинарного файла:
    - транспортный уровень (контроллеры / handlers на Gin);
    - слой сервисов / бизнес-логики (`services.TemperatureService`);
    - слой доступа к данным (`db.DB`, прямой SQL без ORM).
- **Взаимодействие и сетевые протоколы:**
  - Полностью **синхронное взаимодействие** по протоколу HTTP/1.1 (REST JSON API).
  - Реализована **pull-модель опроса**: сервер сам опрашивает датчики по HTTP. При запросе списка датчиков `GET /api/v1/sensors` монолит в цикле последовательно выполняет HTTP-запросы к каждому датчику ($N+1$ сетевых обращений).
  - Полное отсутствие асинхронного обмена данными, очередей сообщений и брокеров (Kafka/RabbitMQ/MQTT). Система не является событийно-ориентированной (No Event-Driven Architecture).
- **Масштабируемость:**
  - Горизонтальное масштабирование монолита неэффективно, так как невозможно масштабировать подсистемы независимо (например, тяжелый опрос 10 000 датчиков отдельно от легкого чтения метаданных пользователями).
  - Синхронный опрос быстро исчерпывает пул горутин, сокетов и файловых дескрипторов при росте количества подключенных домов и датчиков.
- **Развертывание и эксплуатация:**
  - Упаковка приложения в Docker-контейнер на базе минимального образа Alpine Linux.
  - Локальный запуск через `docker-compose.yml` (сервисы: `app`, `postgres`, `temperature-api`).
  - Любое изменение бизнес-логики или устранение дефекта требует полной пересборки и перезапуска всего приложения с простоем (downtime).
- **Отказоустойчивость и надежность:**
  - Монолит является единой точкой отказа (**Single Point of Failure — SPOF**). Зависание внешнего API датчика приводит к блокировке потоков обработки запросов всех клиентов.
  - Отсутствуют механизмы Circuit Breaker, повторных попыток (Retry с экспоненциальным backoff) и фонового асинхронного сбора данных.

---

### 3. Определение доменов и границы контекстов

В соответствии с методологией **Domain-Driven Design (DDD)** для перехода от текущего монолита к масштабируемой целевой SaaS-экосистеме «Умный посёлок» выделены следующие домены и ограниченные контексты (Bounded Contexts):

#### 3.1. Текущие контексты (As-Is)
1. **Контекст «Монолит умного дома» (Smart Home Monolith Context):** единое пространство, совмещающее CRUD оборудования, синхронный опрос датчиков температуры, переключение реле и примитивную привязку к строковым локациям.
2. **Контекст «Ручной монтаж и наладка» (Field Installation Context):** ручной оффлайн-процесс выезда специалистов для физического подключения и конфигурации датчиков.

#### 3.2. Целевые контексты экосистемы (To-Be)

##### А. Основные домены (Core Domains) — уникальное конкурентное преимущество бизнеса
- **Домен управления микроклиматом и отоплением (Heating & Climate Management Context):**
  - *Границы и ответственность:* управление целевыми температурными профилями (комфортный, эко, расписание день/ночь, режим отпуска), реализация алгоритмов терморегулирования (гистерезис, ПИД-регулирование котлов), контроль работы контуров отопления, защита от замерзания трубопроводов.
- **Домен сценариев и автоматизации (Smart Scenarios & Automation Engine Context):**
  - *Границы и ответственность:* декларативное задание и реактивное исполнение пользовательских правил автоматизации ("If-This-Then-That"). Координация между различными типами устройств (например: «при падении температуры ниже 18°C включить контур отопления», «при открытии ворот в темное время суток включить подсветку въезда», «при уходе из дома выключить свет и перевести отопление в эконом-режим»).
- **Домен управления устройствами и шлюзами (Device Management & Gateway Context):**
  - *Границы и ответственность:* реестр и жизненный цикл физических устройств (отопление, освещение, приводы ворот, камеры, датчики открытия/протечки/движения). Поддержка самостоятельного сопряжения устройств пользователем (Self-Onboarding / Pairing) по стандартным IoT-протоколам (MQTT, Zigbee/Z-Wave через шлюзы, HTTP/REST), драйверы абстракции устройств сторонних производителей-партнеров.

##### Б. Поддерживающие домены (Supporting Domains) — необходимы для реализации сценариев умного дома
- **Домен приема и обработки телеметрии (Telemetry Ingestion & Processing Context):**
  - *Границы и ответственность:* высокопроизводительный прием входящего потока метрик и телеметрии от датчиков по push-модели (MQTT/AMQP/gRPC/HTTP), валидация, дедупликация, сохранение временных рядов (Time-Series DB) для построения аналитических графиков за периоды, генерация доменных событий телеметрии в брокер для сценарного движка.
- **Домен пространственной структуры и объектов (Smart Village & Property Context):**
  - *Границы и ответственность:* ведение иерархической структуры недвижимости: Регион -> Посёлок -> Земельный участок / Дом -> Зона / Этаж / Помещение. Привязка устройств к конкретным объектам и помещениям.
- **Домен управления освещением (Lighting Context):**
  - *Границы и ответственность:* включение и выключение света, яркость, группы светильников (например, «первый этаж», «подсветка въезда»), расписания освещения.
- **Домен управления воротами (Gates Context):**
  - *Границы и ответственность:* шлагбаумы посёлка, въездные и гаражные ворота, калитки. Постоянные и гостевые доступы, журнал въездов и выездов.
- **Домен видеонаблюдения (Surveillance Context):**
  - *Границы и ответственность:* доступ к видеопотокам камер (RTSP → WebRTC/HLS), права участников дома на просмотр, журнал событий движения.

##### В. Универсальные домены (Generic Domains) — стандартные типовые функции платформы
- **Домен аутентификации и авторизации (Identity & Access Management / IAM Context):**
  - *Границы и ответственность:* регистрация учетных записей, многофакторная аутентификация, ролевой доступ (RBAC: Администратор управляющей компании посёлка, Владелец дома, Член семьи, Гость, Сервисный инженер), управление JWT/OAuth2 токенами и правами на объекты.
- **Домен уведомлений (Notification Context):**
  - *Границы и ответственность:* отправка тревожных и информационных оповещений пользователям (push в мобильное приложение и email) при критических событиях: авария котла, угроза заморозки, открытие ворот в отсутствие жильцов, движение перед камерой.
- **Домен каталога модулей и интернет-магазина (Store & Device Catalog Context):**
  - *Границы и ответственность:* витрина готовых комплектов модулей («Комплект Отопление», «Комплект Свет», «Комплект Безопасность/Ворота»), оформление заказов, привязка серийных номеров проданных комплектов к аккаунту покупателя для быстрого самообслуживания.

---

### 4. Проблемы монолитного решения

Текущее архитектурное решение не позволяет компании реализовать новые бизнес-цели в рамках тендера по следующим ключевым причинам:

1. **Блокирующий синхронный pull-опрос ($N+1$ сетевых вызовов) и деградация производительности:**
   - Монолит обращается к каждому внешнему датчику синхронным HTTP-запросом прямо в цикле обработки клиентского запроса списка сенсоров (`GET /api/v1/sensors`).
   - При 100 датчиках задержка ответа клиенту уже составляет критические секунды, а при росте до сотен и тысяч устройств система неизбежно исчерпает свободные потоки/сокеты и будет возвращать ошибки `HTTP 504 Gateway Timeout`.
2. **Отсутствие реактивности и брокера сообщений (No Event-Driven Architecture):**
   - Устройства не могут самостоятельно отправить push-уведомление или сигнал тревоги на сервер в момент инцидента.
   - Сервер узнает о критическом падении температуры только при очередном ручном опросе клиентом, что недопустимо для систем предотвращения аварий отопления и заморозки помещений в зимний период.
3. **Единая точка отказа (SPOF) и отсутствие изоляции сбоев:**
   - Монолит объединяет весь функционал в одном процессе. Сбой в обработчике датчиков температуры, утечка памяти или падение базы данных останавливают обслуживание всех клиентов во всех домах.
   - Ошибка или задержка ответа одного неисправного датчика парализует работу веб-сервера для остальных исправных устройств.
4. **Невозможность независимого горизонтального масштабирования:**
   - Разные функциональные блоки имеют принципиально различный профиль нагрузки: прием потока телеметрии от тысяч датчиков требует оптимизации по записи (Write-heavy), тогда как изменение сценариев и авторизация — это редкие транзакционные операции (Read/Write-light). Монолит заставляет масштабировать всё целиком, нерационально расходуя серверные ресурсы.
5. **Высокая стоимость подключения и отсутствие самообслуживания (SaaS-блокер):**
   - Необходимость личного выезда сервисного инженера к каждому клиенту делает невозможным масштабирование сервиса на территорию нескольких регионов страны.
   - Отсутствуют механизмы автоматического обнаружения (zero-touch provisioning), QR-сопряжения и самостоятельного конфигурирования устройств пользователем.
6. **Отсутствие многопользовательской модели (Multi-Tenancy) и учета недвижимости:**
   - В текущей схеме базы данных таблица `sensors` не имеет внешних ключей к пользователям (`user_id`) или домам (`house_id`).
   - Невозможно разграничить права доступа между разными жильцами, членами семьи или управляющей компанией посёлка.
7. **Замедление темпов разработки и сложности развертывания:**
   - Команда из 5 разработчиков вынуждена вносить изменения в одну общую кодовую базу. Добавление новых модулей (свет, ворота, наблюдение) неизбежно порождает конфликты слияния (merge conflicts), регрессионные ошибки и требует остановки сервиса при деплое.

---

### 5. Визуализация контекста системы — диаграмма С4

Диаграмма системного контекста (C4 System Context) As-Is разработана в нотации модели C4 и сохранена в репозитории:

- Исходный файл диаграммы: [schemas/c4_context_as_is.puml](schemas/c4_context_as_is.puml)

```plantuml
@startuml C4_Context
!include https://raw.githubusercontent.com/plantuml-stdlib/C4-PlantUML/master/C4_Context.puml

LAYOUT_WITH_LEGEND()

title Диаграмма системного контекста (C4 System Context) — Текущая система «Тёплый дом» (As-Is)

Person(user, "Пользователь (Клиент)", "Владелец дома, использующий веб-интерфейс для контроля температуры и переключения отопления")
Person(installer, "Инженер монтажа / Специалист компании", "Сотрудник, выезжающий на объект для первичной установки и привязки датчиков")

System(warmhouse_monolith, "Монолит «Тёплый дом» (Smart Home Core)", "Монолитное приложение на Go с СУБД PostgreSQL. Реализует CRUD датчиков, синхронный опрос температуры и прямое управление статусом нагрева")

System_Ext(temp_sensor, "Датчик температуры (Модуль)", "Физический датчик в доме клиента с HTTP API (например, /temperature). Отвечает на запросы сервера о текущей температуре")
System_Ext(heating_relay, "Реле / Модуль управления отоплением", "Физическое реле котла/отопителя. Принимает синхронные команды включения/выключения от монолита")

Rel(user, warmhouse_monolith, "Просмотр температуры и управление отоплением", "HTTPS / JSON API / Web UI")
Rel(installer, warmhouse_monolith, "Ручная регистрация и привязка датчиков к дому", "HTTPS / Admin UI")
Rel(installer, temp_sensor, "Физический монтаж и локальная настройка сети", "Физическое подключение")
Rel(installer, heating_relay, "Физический монтаж и подключение к котлу", "Физическое подключение")

Rel(warmhouse_monolith, temp_sensor, "Синхронный pull-запрос температуры (GET /temperature)", "HTTP")
Rel(warmhouse_monolith, heating_relay, "Синхронная отправка команды вкл/выкл", "HTTP / TCP")

@enduml
```

# Задание 2. Проектирование микросервисной архитектуры

В этом задании вам нужно предоставить только диаграммы в модели C4. Мы не просим вас отдельно описывать получившиеся микросервисы и то, как вы определили взаимодействия между компонентами To-Be системы. Если вы правильно подготовите диаграммы C4, они и так это покажут.

Все диаграммы выполнены в PlantUML с библиотекой [C4-PlantUML](https://github.com/plantuml-stdlib/C4-PlantUML) и лежат в каталоге [schemas](schemas). Чтобы посмотреть их, откройте `.puml`-файл в PlantUML-редакторе или плагине IDE.

**Принципы декомпозиции**

- Границы микросервисов взяты из доменов задания 1: у каждого сервиса своя БД, общих таблиц нет.
- Отопление, освещение, ворота и видеонаблюдение — отдельные микросервисы, как требуют бизнес-цели. Они не общаются с устройствами напрямую, а отправляют команды через Device Management Service.
- Синхронные REST-вызовы используются там, где нужен немедленный ответ: запросы клиентов через API Gateway, проверка прав, отправка команды (Device Management отвечает `202 Accepted`, а дальше команда идёт асинхронно).
- Данные от устройств и команды устройствам идут асинхронно через RabbitMQ. Устройства сами отправляют данные по MQTT (push-модель), поэтому опрос устройств, из-за которого деградирует монолит, больше не нужен.
- Новые типы устройств подключаются без изменения кода: возможности и команды описаны в каталоге типов устройств (`DeviceType.capabilities`), а новые форматы сообщений добавляются адаптерами в Device Gateway.
- Языки: Kotlin (Spring Boot) — для сервисов с насыщенной доменной логикой и для Device Gateway, которому нужна JVM для встроенного HiveMQ. Go — для тонких и высоконагруженных сервисов, как и существующий монолит.
- Миграция с монолита (паттерн Strangler Fig): первыми выносятся Device Management и Telemetry. На время переноса монолит публикует в RabbitMQ изменения датчиков (`legacy.sensors`) и каждое показание температуры (`telemetry.raw`), так что новые сервисы получают данные, а клиенты монолита ничего не замечают. Датчики, созданные до запуска новых сервисов, переносятся один раз скриптом (соответствие полей — в задании 3). Затем API Gateway переключает маршруты `/api/v1/sensors` на новые сервисы, и монолит выводится из эксплуатации. Отдельный слой ACL не нужен: монолит маленький, хватает временных событий.

**Диаграмма контейнеров (Containers)**

Файл: [schemas/c4_container_to_be.puml](schemas/c4_container_to_be.puml)

| Контейнер | Домен (задание 1) | Язык / технология | Хранилище |
|---|---|---|---|
| API Gateway | — | Kong | — |
| IAM Service | Identity & Access Management | Kotlin | PostgreSQL |
| Property Service | Smart Village & Property | Go | PostgreSQL |
| Device Management Service | Device Management | Kotlin | PostgreSQL |
| Device Gateway | Device Management & Gateway | Kotlin + встроенный HiveMQ CE | — |
| Telemetry Service | Telemetry Ingestion & Processing | Go | TimescaleDB |
| Heating Service | Heating | Go | PostgreSQL |
| Lighting Service | Lighting | Go | PostgreSQL |
| Gates Service | Gates | Go | PostgreSQL |
| Surveillance Service | Surveillance | Go | PostgreSQL |
| Scenario Service | Smart Scenarios & Automation | Kotlin | PostgreSQL |
| Notification Service | Notification | Go | — |
| Catalog & Orders Service | Store & Device Catalog | Kotlin | PostgreSQL |
| Cache | — | Redis | — |
| Message Broker | — | RabbitMQ | — |
| Legacy-монолит и temperature-api | As-Is | Go | PostgreSQL |

Exchanges RabbitMQ (тип `topic`):

| Exchange | Кто публикует | Кто читает | Ключ маршрутизации |
|---|---|---|---|
| `telemetry.raw` | Device Gateway | Telemetry | `{deviceType}.{deviceId}` |
| `telemetry.normalized` | Telemetry | Heating (`temperature.*`), Scenario | `{metric}.{deviceType}` |
| `device.commands` | Device Management | Device Gateway | `{deviceType}.{deviceId}` |
| `device.events` | Device Gateway | Device Management, Lighting (`light.*`), Gates (`gate.*`), Surveillance (`camera.*`), Scenario | `{deviceDomain}.{eventType}`, где домен — `heating`, `light`, `gate`, `camera` или `sensor` |
| `notifications` | Heating, Gates, Surveillance, Scenario | Notification | `{severity}` |

У каждого сервиса-потребителя своя очередь, привязанная к exchange по нужным ключам. Несколько экземпляров одного сервиса читают одну очередь (competing consumers), так сервисы масштабируются горизонтально.

**MQTT-брокер.** HiveMQ Community Edition встроен в Device Gateway как библиотека (`hivemq-community-edition-embedded`). Аутентификация устройств, перехват сообщений и отслеживание подключений сделаны расширениями HiveMQ внутри того же процесса. Ограничение: в Community Edition нет кластеризации, поэтому каждый экземпляр Gateway — отдельный брокер. При масштабировании устройства распределяются по экземплярам (например, по домам), а команды маршрутизируются в нужный экземпляр. Для MVP достаточно одного экземпляра.

**Диаграммы компонентов (Components)**

| Микросервис | Язык | Файл |
|---|---|---|
| Device Management Service | Kotlin | [schemas/c4_component_device_service.puml](schemas/c4_component_device_service.puml) |
| Device Gateway | Kotlin | [schemas/c4_component_device_gateway.puml](schemas/c4_component_device_gateway.puml) |
| Telemetry Service | Go | [schemas/c4_component_telemetry_service.puml](schemas/c4_component_telemetry_service.puml) |
| Heating Service | Go | [schemas/c4_component_heating_service.puml](schemas/c4_component_heating_service.puml) |
| Lighting Service | Go | [schemas/c4_component_lighting_service.puml](schemas/c4_component_lighting_service.puml) |
| Gates Service | Go | [schemas/c4_component_gates_service.puml](schemas/c4_component_gates_service.puml) |
| Surveillance Service | Go | [schemas/c4_component_surveillance_service.puml](schemas/c4_component_surveillance_service.puml) |
| Scenario Service | Kotlin | [schemas/c4_component_scenario_service.puml](schemas/c4_component_scenario_service.puml) |
| Notification Service | Go | [schemas/c4_component_notification_service.puml](schemas/c4_component_notification_service.puml) |
| IAM Service | Kotlin | [schemas/c4_component_iam_service.puml](schemas/c4_component_iam_service.puml) |
| Property Service | Go | [schemas/c4_component_property_service.puml](schemas/c4_component_property_service.puml) |
| Catalog & Orders Service | Kotlin | [schemas/c4_component_catalog_service.puml](schemas/c4_component_catalog_service.puml) |

**Диаграммы кода (Code)**

Выбраны два самых критичных места системы:

- [schemas/c4_code_device_command_classes.puml](schemas/c4_code_device_command_classes.puml) — диаграмма классов отправки команды устройству в Device Management Service. Команда и запись outbox сохраняются в одной транзакции, а в RabbitMQ публикуются отдельно и помечаются отправленными только после подтверждения брокером (transactional outbox + publisher confirms).
- [schemas/c4_code_telemetry_to_heating_sequence.puml](schemas/c4_code_telemetry_to_heating_sequence.puml) — диаграмма последовательности от показания датчика до включения отопления: Device Gateway → RabbitMQ → Telemetry → Heating → Device Management → Device Gateway → реле. При угрозе замерзания владелец получает уведомление.

# Задание 3. Разработка ER-диаграммы

ER-диаграмма: [schemas/er_diagram.puml](schemas/er_diagram.puml) (PlantUML, нотация «воронья лапка»).

### Принципы модели

- Сущности сгруппированы по микросервисам из задания 2. Каждый сервис владеет своими таблицами. Связи внутри сервиса — настоящие внешние ключи (сплошные линии на диаграмме).
- Связи между сервисами — логические ссылки по UUID без внешних ключей в БД (пунктир), чтобы сервисы можно было развёртывать и масштабировать независимо. Целостность данных обеспечивается через события RabbitMQ и проверки по REST.
- Идентификаторы — UUID, чтобы их можно было создавать без обращения к общей БД.

### Сущности и сервисы

| Сервис | Сущности |
|---|---|
| IAM | `User`, `RefreshToken`, `NotificationPreference` |
| Property | `Village`, `House`, `Room`, `HouseMember`, `HouseInvitation` |
| Device Management | `DeviceType`, `DeviceCapability`, `Module`, `Device`, `DeviceState`, `DeviceCommand` |
| Telemetry | `TelemetryData` |
| Heating | `HeatingProfile`, `HeatingSchedule` |
| Lighting | `LightGroup`, `LightGroupMember`, `LightSchedule` |
| Gates | `GatePermission`, `GateEvent` |
| Surveillance | `CameraPermission`, `MotionEvent` |
| Scenario | `Scenario`, `ScenarioTrigger`, `ScenarioAction`, `ScenarioExecution` |
| Catalog & Orders | `Kit`, `KitItem`, `Order`, `OrderItem`, `SerialNumber` |

### Ключевые связи

- **Пользователь — Дом:** многие-ко-многим через `HouseMember` с ролью (`OWNER`, `FAMILY`, `GUEST`, `INSTALLER`). Это шире примера из задания, где у дома один пользователь: в семье доступ к дому нужен нескольким людям. Владелец дополнительно хранится в `House.owner_id`.
- **Посёлок — Дом — Помещение:** один-ко-многим на каждом уровне. `Village` у дома необязателен: дом может быть вне посёлка.
- **Дом — Устройство:** один дом содержит много устройств, каждое устройство принадлежит одному дому. Помещение (`room_id`) необязательно.
- **Тип устройства — Устройство:** один-ко-многим. `DeviceType` вместе с `DeviceCapability` описывает возможности и команды типа, поэтому новые датчики подключаются без изменения кода.
- **Модуль — Устройство:** один модуль (блок управления, шлюз, хаб) управляет несколькими устройствами. Устройство может быть и без модуля (например, Wi-Fi-датчик).
- **Устройство — Состояние:** один-к-одному (`DeviceState` — последнее известное состояние).
- **Устройство — Команда:** один-ко-многим. Статус команды меняется по подтверждению от устройства; `idempotency_key` защищает от повторной отправки.
- **Устройство — Телеметрия:** один-ко-многим. Первичный ключ `TelemetryData` — `(device_id, ts, metric)`, таблица хранится как hypertable в TimescaleDB.
- **Сценарий — Триггер / Действие / Журнал:** один-ко-многим. Триггеры и действия ссылаются на устройства по `device_id`.
- **Группа света — Светильник:** многие-ко-многим через `LightGroupMember`: один светильник может входить в несколько групп. У группы может быть несколько расписаний (`LightSchedule`).
- **Ворота / Камера — Пользователь:** доступы хранятся в своём сервисе (`GatePermission`, `CameraPermission`) и ссылаются на устройство и пользователя по UUID. События (`GateEvent`, `MotionEvent`) — один-ко-многим от устройства.
- **Комплект — Тип устройства:** многие-ко-многим через `KitItem`. Серийный номер проданного модуля (`SerialNumber`) проверяется при подключении и после активации связывается с `Device`.

### Соответствие старой таблице `sensors`

| Поле в `sensors` (монолит) | Куда переносится |
|---|---|
| `id`, `name`, `location` | `Device.id`, `Device.name`, `Room.name` |
| `type` | `DeviceType.code` |
| `value`, `unit`, `last_updated` | `TelemetryData.value`, `unit`, `ts` (последнее значение также в `DeviceState.reported`) |
| `status` | `Device.status` и `DeviceState.is_online` |
| `created_at` | `Device.created_at` |

Связь старых и новых данных — колонка `sensors.device_id` (UUID): под этим идентификатором датчик известен Device Management и Telemetry. Пока монолит работает, новые изменения датчиков и показания он передаёт событиями RabbitMQ (см. задание 6). Датчики, созданные раньше, переносятся по этой таблице одноразовым скриптом. После переноса API Gateway переключает маршруты `/api/v1/sensors` на новые сервисы, и таблица `sensors` больше не используется.

# Задание 4. Создание и документирование API

### 1. Тип API

Используются два типа API — по характеру взаимодействия.

**REST (OpenAPI 3.1)** — когда вызывающему нужен немедленный ответ:
- запросы веб- и мобильного приложения через API Gateway (показать устройства, задать температуру, открыть ворота);
- синхронные вызовы между сервисами: отправка команды в Device Management Service, проверка доступа в Property Service.

Операции, которые физически выполняет устройство, тоже идут через REST, но отвечают `202 Accepted`: запрос принят и проверен, а результат приходит асинхронно и доступен по `GET /devices/{deviceId}/commands/{commandId}`. Так клиент сразу узнаёт об ошибках (нет прав, устройство офлайн, команда не поддерживается), но не ждёт ответа устройства.

**AsyncAPI 3.0** — для событийного взаимодействия, где отправителю не нужно ждать ответа:
- **RabbitMQ (AMQP)** — шина событий между микросервисами: телеметрия, команды, события устройств, уведомления. Так один источник событий обслуживает нескольких получателей, сервисы не зависят от доступности друг друга, а всплески телеметрии сглаживаются очередями.
- **MQTT** — контракт подключения устройств к Device Gateway. Устройства сами отправляют данные (push) и держат постоянное соединение, поэтому платформа больше не опрашивает их, как монолит. Это стандартный протокол, и по опубликованному контракту партнёры подключают свои устройства без доработки платформы.

Общие правила REST API:
- префикс `/api/v1`, аутентификация — JWT от IAM Service (у пользователей — после входа, у сервисов — OAuth2 client credentials);
- ошибки — в формате RFC 7807 (`application/problem+json`);
- идентификаторы — UUID, время — ISO 8601 в UTC;
- для команд поддерживается заголовок `Idempotency-Key`, чтобы повтор запроса не выполнил команду дважды.

### 2. Документация API

Спецификации лежат в каталоге [api](api). Открыть их можно в [Swagger Editor](https://editor.swagger.io/) (OpenAPI) и [AsyncAPI Studio](https://studio.asyncapi.com/) (AsyncAPI): скопируйте содержимое файла в редактор.

**REST API — 19 эндпоинтов**

| Сервис | Спецификация | Эндпоинты |
|---|---|---|
| Device Management (Kotlin) | [device-management.openapi.yaml](api/device-management.openapi.yaml) | `POST /devices/onboarding` — подключить устройство<br>`GET /devices` — устройства дома<br>`GET /devices/{deviceId}` — устройство и состояние<br>`POST /devices/{deviceId}/commands` — отправить команду<br>`GET /devices/{deviceId}/commands/{commandId}` — статус команды<br>`POST /device-types` — зарегистрировать новый тип устройства |
| Telemetry (Go) | [telemetry.openapi.yaml](api/telemetry.openapi.yaml) | `GET /telemetry/devices/{deviceId}/latest` — последние значения<br>`GET /telemetry/devices/{deviceId}/history` — история и агрегаты |
| Heating (Go) | [heating.openapi.yaml](api/heating.openapi.yaml) | `GET /houses/{houseId}/heating` — состояние отопления<br>`PUT /houses/{houseId}/heating/power` — включить/выключить<br>`PUT /houses/{houseId}/heating/rooms/{roomId}/target` — целевая температура |
| Lighting (Go) | [lighting.openapi.yaml](api/lighting.openapi.yaml) | `PUT /houses/{houseId}/lighting/lights/{deviceId}/state` — свет и яркость |
| Gates (Go) | [gates.openapi.yaml](api/gates.openapi.yaml) | `POST /houses/{houseId}/gates/{gateId}/actions` — открыть/закрыть<br>`POST /houses/{houseId}/gates/{gateId}/guest-passes` — гостевой доступ |
| Surveillance (Go) | [surveillance.openapi.yaml](api/surveillance.openapi.yaml) | `POST /houses/{houseId}/cameras/{cameraId}/stream-sessions` — ссылка на видеопоток |
| Scenario (Kotlin) | [scenario.openapi.yaml](api/scenario.openapi.yaml) | `POST /houses/{houseId}/scenarios` — создать сценарий<br>`PATCH /houses/{houseId}/scenarios/{scenarioId}` — включить/выключить |
| Property (Go) | [property.openapi.yaml](api/property.openapi.yaml) | `POST /houses/{houseId}/members` — пригласить участника<br>`GET /internal/access-check` — проверка доступа для сервисов |

Для каждого эндпоинта описаны формат запроса и ответа (JSON Schema), коды ответа и примеры в блоках `examples`.

**AsyncAPI — 11 операций**

| Спецификация | Операции |
|---|---|
| [rabbitmq.asyncapi.yaml](api/rabbitmq.asyncapi.yaml) — шина событий между сервисами | `publishTelemetryRaw` (Device Gateway → `telemetry.raw`)<br>`publishTelemetryNormalized` (Telemetry → `telemetry.normalized`)<br>`publishDeviceCommand` (Device Management → `device.commands`)<br>`publishDeviceEvent` (Device Gateway → `device.events`)<br>`publishNotification` (Heating, Gates, Surveillance, Scenario → `notifications`)<br>`publishLegacySensorEvent` (монолит → `legacy.sensors`, на время миграции) |
| [device-mqtt.asyncapi.yaml](api/device-mqtt.asyncapi.yaml) — контракт устройства | `sendTelemetry` — показания<br>`sendState` — состояние (retain)<br>`sendAvailability` — онлайн/офлайн (Last Will)<br>`receiveCommand` — команда от платформы<br>`sendCommandAck` — результат команды |

В спецификации RabbitMQ для каждой операции указаны отправитель, получатели с их очередями и привязками, ключи маршрутизации и AMQP-параметры (persistent-сообщения, TTL команд 30 секунд). В MQTT-контракте — топики, QoS, retain, аутентификация и правила обработки команд (истёкшие не выполнять, повторы с тем же `commandId` игнорировать).

# Задание 5. Работа с docker и docker-compose

### Что сделано

1. **[temperature_api](apps/temperature_api)** — приложение на Go (только стандартная библиотека), порт `8081`:
   - `GET /temperature?location=` — случайная температура 16–26 °C для комнаты;
   - `GET /temperature/{sensorId}` — то же по идентификатору датчика (этот путь вызывает монолит в `GET /api/v1/sensors`);
   - `GET /health`.

   Соответствие комнаты и `sensorId` — как в ТЗ (см. фрагмент ниже). Ответ содержит поля, которые ожидает монолит: `value`, `unit`, `timestamp`, `location`, `status`, `sensor_id`, `sensor_type`, `description`.
2. **[Dockerfile](apps/temperature_api/Dockerfile)** — многоэтапная сборка: `golang:1.22-alpine` → `alpine:3.20`.
3. **[docker-compose.yml](apps/docker-compose.yml)**:
   - `postgres` (16-alpine) со скриптом инициализации `./smart_home/init.sql`, томом `postgres_data` и healthcheck; монолит стартует только после готовности БД;
   - `temperature-api` на порту `8081`;
   - `app` — монолит на порту `8080`.

   `POSTGRES_DB` не задаётся: `init.sql` сам создаёт базу `smarthome`.
4. [init.sh](apps/init.sh) работает и с `docker compose` (v2), и с `docker-compose` (v1).

### Как проверить

```bash
cd apps
./init.sh                      # или docker compose up --build -d
```

В Postman (коллекция [smarthome-api.postman_collection.json](apps/smarthome-api.postman_collection.json)) вызвать **Create Sensor**, затем несколько раз **Get All Sensors** — значение `value` каждый раз другое. То же через curl:

```bash
curl -X POST localhost:8080/api/v1/sensors -H 'Content-Type: application/json' \
  -d '{"name":"Living Room Temperature","type":"temperature","location":"Living Room","unit":"°C"}'
curl localhost:8080/api/v1/sensors   # value: 16.7
curl localhost:8080/api/v1/sensors   # value: 25.9
```

### Исходное условие

Locations - название комнаты, sensorId - идентификатор названия комнаты

```
	// If no location is provided, use a default based on sensor ID
	if location == "" {
		switch sensorID {
		case "1":
			location = "Living Room"
		case "2":
			location = "Bedroom"
		case "3":
			location = "Kitchen"
		default:
			location = "Unknown"
		}
	}

	// If no sensor ID is provided, generate one based on location
	if sensorID == "" {
		switch location {
		case "Living Room":
			sensorID = "1"
		case "Bedroom":
			sensorID = "2"
		case "Kitchen":
			sensorID = "3"
		default:
			sensorID = "0"
		}
	}
```

# Задание 6. Разработка MVP

MVP — учебный прототип с простейшей логикой. Он показывает первый шаг перехода по схеме Strangler Fig: новые сервисы работают рядом с монолитом и получают от него данные через RabbitMQ, а клиенты монолита ничего не замечают. Целевая архитектура описана в заданиях 2–4; MVP реализует только её часть (см. «Упрощения»).

### Состав

| Сервис | Язык | Порт | Хранилище | Что делает |
|---|---|---|---|---|
| [smart_home](apps/smart_home) (монолит) | Go | 8080 | БД `smarthome` | Прежний API датчиков; дополнительно публикует события в RabbitMQ |
| [device_service](apps/device_service) | Kotlin, Spring Boot | 8082 | БД `devices` | Реестр устройств и отправка команд |
| [telemetry_service](apps/telemetry_service) | Go | 8083 | БД `telemetry` | Приём и хранение телеметрии, текущие значения и история |
| [temperature_api](apps/temperature_api) | Go | 8081 | — | Имитация датчиков (задание 5) |
| RabbitMQ | — | 5672, 15672 | — | Брокер сообщений |
| PostgreSQL | — | 5432 | — | Один сервер, у каждого сервиса своя база ([init-microservices.sql](apps/init-microservices.sql)) |

Все сервисы собираются из своих Dockerfile и запускаются одним [docker-compose.yml](apps/docker-compose.yml).

Новые сервисы написаны на разных языках: Device Service — на Kotlin, Telemetry Service — на Go. Go поддерживает ООП без классов: инкапсуляцию (типы с методами и неэкспортируемыми полями), полиморфизм через интерфейсы и композицию через встраивание типов. Telemetry Service построен из объектов с одной ответственностью: `Store` (хранилище), `Consumer` (чтение из RabbitMQ) и `Handler` (REST API).

### Взаимодействие с монолитом

```
                     legacy.sensors (sensor.created/updated/deleted)
          ┌───────────────────────────────────────────────────────────► device_service ──► device.commands
monolith ─┤                                                                 (Kotlin)
          └── telemetry.raw ({deviceType}.{deviceId}) ────────────────► telemetry_service
               при каждом чтении температуры из temperature-api              (Go)
```

1. У каждого датчика монолита появился `device_id` (UUID). Колонка добавляется при старте монолита, существующим датчикам UUID проставляется автоматически. По этому идентификатору датчик известен новым сервисам.
2. При создании, изменении и удалении датчика монолит публикует событие в exchange `legacy.sensors`. Device Service регистрирует устройство с тем же `id` (тип `temperature` → `temp_sensor`), а при удалении датчика помечает устройство `DISABLED`.
3. При каждом получении температуры (`GET /api/v1/sensors`, `GET /api/v1/sensors/{id}`) и при ручном `PATCH .../value` монолит публикует сообщение `TelemetryRaw` в exchange `telemetry.raw` в формате из [rabbitmq.asyncapi.yaml](api/rabbitmq.asyncapi.yaml). Telemetry Service читает очередь `telemetry.ingest` и сохраняет показания, так что история температуры копится в новом сервисе.
4. Публикация из монолита — best effort: если RabbitMQ недоступен, ошибка пишется в лог, а HTTP-запрос выполняется как раньше.
5. Новые устройства, которых нет в монолите, регистрируются сразу в Device Service (`POST /api/v1/devices`). Команды (`POST /api/v1/devices/{id}/commands`) публикуются в exchange `device.commands` с ключом `{deviceType}.{deviceId}` и TTL 30 секунд — в целевой архитектуре их получает Device Gateway.

Форматы сообщений (`TelemetryRaw`, `DeviceCommand`, `LegacySensorEvent`) и REST-эндпоинты MVP соответствуют спецификациям задания 4 в упрощённом виде (см. «Упрощения»).

Дальше функциональность переносится по шагам: клиенты переключаются на API новых сервисов (через API Gateway), монолит перестаёт опрашивать temperature-api, а таблица `sensors` выводится из эксплуатации.

### API

Device Service (Kotlin):
- `GET /api/v1/devices[?type=]` — список устройств;
- `GET /api/v1/devices/{id}` — устройство;
- `POST /api/v1/devices` — зарегистрировать устройство (`name`, `typeCode`, `location`);
- `POST /api/v1/devices/{id}/commands` — отправить команду (`name`, `params`, `source`), ответ `202 Accepted` с объектом `Command`.

Telemetry Service (Go):
- `GET /api/v1/telemetry/devices/{deviceId}/latest` — последнее значение каждой метрики;
- `GET /api/v1/telemetry/devices/{deviceId}/history?metric=&from=&to=&interval=` — история метрики: `raw` — исходные значения, `1m`/`1h`/`1d` — avg/min/max за интервал. По умолчанию — последние 24 часа, интервал `1h`.

Ошибки обоих сервисов — в формате RFC 7807 (`application/problem+json`).

### Как проверить

```bash
cd apps && ./init.sh

# 1. Создать датчик в монолите и пару раз прочитать температуру (как в задании 5)
curl -X POST localhost:8080/api/v1/sensors -H 'Content-Type: application/json' \
  -d '{"name":"Living Room Temperature","type":"temperature","location":"Living Room","unit":"°C"}'
curl localhost:8080/api/v1/sensors          # запомнить device_id

# 2. Датчик появился в Device Service с тем же id
curl localhost:8082/api/v1/devices

# 3. Показания попали в Telemetry Service
curl localhost:8083/api/v1/telemetry/devices/<device_id>/latest
curl 'localhost:8083/api/v1/telemetry/devices/<device_id>/history?metric=temperature&interval=raw'

# 4. Новое устройство и команда ему
curl -X POST localhost:8082/api/v1/devices -H 'Content-Type: application/json' \
  -d '{"name":"Котёл","typeCode":"heating_relay","location":"Boiler room"}'
curl -X POST localhost:8082/api/v1/devices/<id>/commands -H 'Content-Type: application/json' \
  -d '{"name":"turn_on","source":"USER"}'   # 202 Accepted
```

Очереди и сообщения можно посмотреть в RabbitMQ Management: http://localhost:15672 (`warmhouse` / `warmhouse`).

### Упрощения по сравнению с целевой архитектурой

- Нет API Gateway, IAM и проверки доступа: сервисы доступны напрямую на своих портах.
- Нет Device Gateway, MQTT и устройств: команды из `device.commands` никто не читает, статус команды не отслеживается.
- Нет outbox и publisher confirms: сообщение публикуется сразу, при сбое брокера оно может потеряться.
- Одна база PostgreSQL на сервере вместо отдельных серверов и TimescaleDB для телеметрии; vhost RabbitMQ по умолчанию (`/`) вместо `/warmhouse`.
- Модель устройства упрощена: нет домов, комнат, типов устройств с возможностями и онбординга по серийному номеру. `GET /devices` возвращает массив без пагинации и не требует `houseId`; `POST /devices` заменяет онбординг; эндпоинтов статуса команды и `/device-types` нет.
- Нет скрипта первичного переноса датчиков: в Device Service попадают датчики, созданные или изменённые после его запуска.
- Реализованы только сервисы устройств и телеметрии, как требует задание 6. Отопление, освещение, ворота и видеонаблюдение спроектированы в заданиях 2–4, но в MVP не входят.