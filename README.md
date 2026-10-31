# ScreenSift

Локальный Go-сервис индексации изображений по тексту внутри них.

## Требования

- Go 1.23+
- Tesseract OCR
- SQLite/FTS5
- PNG/JPEG/JPG

Поиск регистронезависимый. Несколько слов объединяются через AND. `limit` ограничен 100.

## Установка Tesseract

macOS Apple Silicon:

```bash
brew install tesseract tesseract-lang
tesseract --list-langs
```

Должны быть доступны `eng` и `rus`.

## Запуск

```bash
go mod tidy
go test ./...
go test -race ./...
go run ./cmd/screensift -root ./screenshots
```

При необходимости:

```bash
go run ./cmd/screensift -root ./screenshots -tesseract /opt/homebrew/bin/tesseract
```

## Конфигурация

```text
-root            директория изображений
-db              SQLite database
-addr            HTTP address
-tesseract       путь к Tesseract
-langs           языки, по умолчанию rus+eng
-ocr-timeout     30s
-queue-size      100
-workers         2
-debounce        500ms
-max-image-size  25MB
-max-width       30000
-max-height      30000
-max-pixels      50000000
```

## Архитектура

filesystem -> fsnotify -> debounce -> bounded queue -> worker pool -> Tesseract -> normalized text -> SQLite FTS5.

Состояния: `pending`, `processing`, `indexed`, `failed`.

Изменение файла обрабатывается без полного rebuild. После OCR приложение повторно проверяет size и modification time, поэтому старый результат не записывается поверх новой версии.

Удаление удаляет документ из SQLite и FTS5. Rename обрабатывается как удаление старого пути; новое событие приводит к индексации нового пути.

После restart выполняется reconciliation: новые, изменённые и удалённые файлы синхронизируются с сохранённым состоянием.

## API

- `GET /search?q=connection+refused&limit=20`
- `GET /documents`
- `GET /documents/{id}`
- `GET /documents/{id}/reindex`
- `GET /stats`
- `POST /index/rebuild`
- `POST /documents/{id}/reindex`
- `GET /openapi.yaml`
- `GET /swagger`

## Ограничения

PDF, видео, облачный OCR, embeddings, vector DB, semantic search и собственная OCR-модель не используются.

## Dataset и исследование

`docs/evaluation.md` и `docs/ground_truth.csv` содержат шаблон исследования. Реальные precision/recall и benchmark-цифры должны быть получены на подготовленном dataset и не выдумываются проектом.
