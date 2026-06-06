# pinterest-collage

Генератор Pinterest-style коллажа из 12-15 фотографий с переменной шириной столбцов, скруглёнными углами и автоприменением в качестве обоев Windows. Подбирает число столбцов и распределение фото по ним так, чтобы aspect ratio коллажа максимально совпадал с aspect ratio экрана.

## Стек

- **Go 1.22+** (тестировано на 1.26.3)
- Стандартная библиотека: `image`, `image/jpeg`, `image/png`, `image/gif`, `image/draw`, `syscall`, `encoding/json`
- `golang.org/x/image/draw` — ресэмплинг Catmull-Rom
- `golang.org/x/sys/windows/registry` — `HKCU\...\Run` для автозапуска
- Windows API: `GetSystemMetrics`, `SystemParametersInfoW(SPI_SETDESKWALLPAPER)`

## Структура

```
.
├── main.go           — CLI, флаги, конфиг, экран, обои, автозапуск
├── config.go         — config.json: загрузка, валидация, парсинг hex-цвета
├── collage.go        — алгоритм раскладки (Pinterest masonry)
├── render.go         — рендер с маской скруглённых углов, scaleFit
├── images.go         — загрузка JPEG/PNG/GIF из каталога
├── autostart.go      — register/unregister в HKCU\...\Run
├── config.json       — настройки (создаётся через -init)
├── images/           — папка для фотографий
└── pinterest-collage.exe
```

## Сборка

```bash
go build -o pinterest-collage.exe
```

Под Windows и под линуксом тоже собирается (если убрать `autostart.go`); на других ОС работает генерация PNG, но не установка обоев.

## Конфигурация

Опции читаются из `config.json` (пути поиска: `./`, `<exe-dir>/`, `%APPDATA%\pinterest-collage\`). Файл создаётся через `-init` или вручную:

```bash
.\pinterest-collage.exe -init
```

```json
{
    "min_tiles": 12,
    "max_tiles": 15,
    "interval_seconds": 180,
    "margin_top": 40,
    "margin_bottom": 10,
    "margin_left": 10,
    "margin_right": 10,
    "gap": 12,
    "images_dir": "images",
    "background_color": "#f0edec"
}
```

Приоритет: **CLI-флаг > config.json > встроенные дефолты**. Флаг, который пользователь не передавал, берётся из конфига.

| Поле | Смысл |
| --- | --- |
| `min_tiles`, `max_tiles` | диапазон числа плиток в одном коллаже (по умолчанию 12-15) |
| `interval_seconds` | пауза между пересборкой обоев; 0 = один раз и выход |
| `margin_top`/`bottom`/`left`/`right` | отступы от края экрана в px (GlazeWM + Zebar) |
| `gap` | зазор между плитками в px |
| `images_dir` | путь к каталогу фото (относительный или абсолютный) |
| `background_color` | hex `#rrggbb` — фон канвы и зазоров |

## Флаги

| Флаг | Назначение |
| --- | --- |
| `-init` | записать дефолтный `config.json` и выйти |
| `-config-out <path>` | куда писать при `-init` (по умолчанию `<exe-dir>/config.json`) |
| `-config <path>` | явный путь к `config.json` |
| `-out <path>` | путь к PNG (по умолчанию `%TEMP%\pinterest-collage-wallpaper.png`) |
| `-no-apply` | только сгенерировать картинку, обои не менять |
| `-min`, `-max` | переопределить диапазон числа плиток |
| `-interval <dur>` | переопределить период (например `5m`, `30s`, `0` = one-shot) |
| `-images <dir>` | переопределить каталог фото |
| `-margin-top`/`-bottom`/`-left`/`-right` | переопределить отступ |
| `-gap <n>` | переопределить зазор |
| `-install` | зарегистрировать в `HKCU\...\Run` и выйти |
| `-uninstall` | снять регистрацию и выйти |

## Использование

```bash
# Один раз обновить обои
.\pinterest-collage.exe -interval 0

# Запустить в фоне, обновлять каждые N (настраивается в config.json)
.\pinterest-collage.exe

# Зарегистрировать автозапуск при логине
.\pinterest-collage.exe -install

# Создать config.json
.\pinterest-collage.exe -init
```

## Горячая клавиша (GlazeWM)

В `~/.glzr/glazewm/config.yaml`:

```yaml
keybindings:
  - commands: ['shell-exec --hide-window E:/Dev/projects/pinterest-collage/pinterest-collage.exe -interval 0']
    bindings: ['ctrl+alt+space']
```

`-interval 0` заставляет процесс сделать один проход (генерация + apply) и выйти — горячая клавиша порождает лишь короткоживущий воркер. Периодический рефрешер при этом зарегистрирован в автозапуске отдельно (`-install`).

## Алгоритм

`collage.go::generate`:

1. Случайно выбирает `count` фото в диапазоне `[min_tiles, max_tiles]`.
2. Для каждой кандидатуры числа столбцов от 2 до `min(count, 10)`:
   - Раскидывает фото по столбцам поровну (с разницей ≤ 1).
   - Считает `col_aspect = 1 / sum(1 / image_aspect)` (гармонический стиль).
   - Сумма column-aspect'ов — общий aspect коллажа.
   - Скоринг: `min(collage_aspect, target_aspect) / max(...)` — чем ближе к 1, тем лучше.
3. Лучший по скорингу расклад идёт в работу.
4. Если `collage_aspect > safe_aspect` — коллаж ограничен шириной экрана и центрирован вертикально; иначе — ограничен высотой и центрирован горизонтально.
5. `col_w = col_aspect * collage_h`. Для каждого фото: `h = col_w / image_aspect`.
6. «Ячейка» = `(col_w - gap) × (h - gap)`, с отступом `gap/2` от краёв столбца.
7. Фото вписывается в ячейку с сохранением aspect ratio (contain) и центрируется. Пустое место в ячейке = цвет фона канвы.

`render.go::render`:

1. Заливает канву `background_color`.
2. Для каждой плитки: `scaleFit` (CatmullRom) → `roundedMask` (8-16 px радиус) → `draw.DrawMask`.

## Скруглённые углы

Радиус = `min(W, H) / 20`, в диапазоне `[8, 16]` px. Маска строится как `image.Alpha` с quarter-circle в углах и линейным сглаживанием в 1 пиксель по краю.

## Как это устроено

- `main.go` — точка входа, флаги, `screenBounds`, `setWallpaper`, `resolveImagesDir` (CWD → `<exe-dir>/<dir>`).
- `config.go` — `Config` struct, поиск `config.json` в трёх путях, парсинг hex-цвета, запись дефолта через `-init`.
- `collage.go` — алгоритм выше.
- `render.go` — рендер + маска.
- `images.go` — `loadImages(dir)`: нерекурсивный поиск JPEG/PNG/GIF, битые файлы пропускаются с warn.
- `autostart.go` — `setAutoStart` / `removeAutoStart` через `golang.org/x/sys/windows/registry`.

## Известные особенности

- `go mod tidy` апгрейдит `go 1.22` → `go 1.25.0` каждый раз. Это потому что зависимости (`x/sys v0.45+` или `x/image`) требуют 1.25. Если нужен честный 1.22 — фиксируйте `x/sys` на `v0.30.0` и не запускайте `tidy` (или собирайте с `GOTOOLCHAIN=local`).
- Если папка `images/` пуста, программа отработает пустым набором (сейчас без fallback'а на цветные плитки).
- GlazeWM keybinding нужно перезагрузить (`alt+shift+r`) после правки `config.yaml`.
