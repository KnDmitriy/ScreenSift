package config

import (
	"flag"
	"time"
)

type Config struct {
	RootDir       string
	DBPath        string
	HTTPAddr      string
	TesseractPath string
	OCRLanguages  string
	OCRTimeout    time.Duration
	QueueSize     int
	Workers       int
	Debounce      time.Duration
	MaxImageSize  int64
	MaxWidth      int
	MaxHeight     int
	MaxPixels     int64
	IgnoreDirs    []string
}

func FromFlags() Config {
	var cfg Config
	flag.StringVar(&cfg.RootDir, "root", "./screenshots", "directory with images")
	flag.StringVar(&cfg.DBPath, "db", "./data/screensift.db", "SQLite database path")
	flag.StringVar(&cfg.HTTPAddr, "addr", ":8080", "HTTP listen address")
	flag.StringVar(&cfg.TesseractPath, "tesseract", "tesseract", "Tesseract executable")
	flag.StringVar(&cfg.OCRLanguages, "langs", "rus+eng", "Tesseract languages")
	flag.DurationVar(&cfg.OCRTimeout, "ocr-timeout", 30*time.Second, "OCR timeout")
	flag.IntVar(&cfg.QueueSize, "queue-size", 100, "bounded indexing queue size")
	flag.IntVar(&cfg.Workers, "workers", 2, "number of OCR workers")
	flag.DurationVar(&cfg.Debounce, "debounce", 500*time.Millisecond, "filesystem event debounce")
	flag.Int64Var(&cfg.MaxImageSize, "max-image-size", 25*1024*1024, "maximum image file size")
	flag.IntVar(&cfg.MaxWidth, "max-width", 30000, "maximum image width")
	flag.IntVar(&cfg.MaxHeight, "max-height", 30000, "maximum image height")
	flag.Int64Var(&cfg.MaxPixels, "max-pixels", 50_000_000, "maximum image pixels")
	flag.Parse()

	if cfg.Workers < 1 {
		cfg.Workers = 1
	}
	if cfg.QueueSize < 1 {
		cfg.QueueSize = 1
	}
	return cfg
}
