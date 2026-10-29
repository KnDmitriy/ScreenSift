package main

import (
	"strings"

	"github.com/otiai10/gosseract/v2"
)

func main() {
}

func RecognizeText(screenPath string) (string, error) {
	client := gosseract.NewClient()
	err := client.SetLanguage("rus+eng")
	if err != nil {
		return "", err
	}
	err = client.SetImage(screenPath)
	if err != nil {
		return "", err
	}
	text, err := client.Text()
	err = client.Close()
	if err != nil {
		return "", err
	}
	return NormalizeText(text), nil
}

func NormalizeText(inputText string) string {
	return strings.ToLower(strings.Join(strings.Fields(inputText), " "))
}
