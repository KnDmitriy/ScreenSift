package main

import (
	"testing"
)

func TestRecognizeTextRus(t *testing.T) {
	screenPath := "TestData/Книги.png"
	rightText := "книги"
	recognizedText, err := RecognizeText(screenPath)
	if err != nil {
		t.Error("Error shouldn't be here. RecognizeText error:", err)
	} else if !(recognizedText == rightText) {
		t.Error("RecognizeText error:\n" + recognizedText + "\ninstead of\n" + rightText)
	}
}

func TestRecognizeTextRusAndEng(t *testing.T) {
	screenPath := "TestData/Письмо_с_почты_rus+eng.png"
	rightText := "кому: timofeevane коноров. отчеты по 3, 4, 5 лабораторным labview 14:42 laboratornaya_... laboratornaya_... laboratornaya_..."
	recognizedText, err := RecognizeText(screenPath)
	if err != nil {
		t.Error("Error shouldn't be here. RecognizeText error:", err)
	} else if recognizedText != rightText {
		t.Errorf("Recognized text:\n%q", recognizedText)
		t.Logf("Expected:\n%q", rightText)
	}
}

func TestRecognizeTextInvalidPath(t *testing.T) {
	screenPath := "invalid_path"
	recognizedText, err := RecognizeText(screenPath)
	if err == nil {
		t.Error("RecognizeText error mustn't be nil.")
	}
	if recognizedText != "" {
		t.Error("RecognizeText must be empty in case of error.")
	}
}

func TestRecognizeTextImageWithoutText(t *testing.T) {
	screenPath := "TestData/Изображение_без_текста.png"
	rightText := ""
	recognizedText, err := RecognizeText(screenPath)
	if err != nil {
		t.Error("Error shouldn't be here. RecognizeText error:", err)
	} else if recognizedText != rightText {
		t.Errorf("Recognized text:\n%q", recognizedText)
		t.Logf("Expected:\n%q", rightText)
	}
}

func TestNormalizeText(t *testing.T) {
	inputText := " \n Очень важный text!!! \n\n\n\r\t\n Yeah!    Really important.\r       "
	rightNormalizedText := "очень важный text!!! yeah! really important."
	normalizedText := NormalizeText(inputText)
	if normalizedText != rightNormalizedText {
		t.Errorf("NormalizeText error:\n%q", normalizedText)
		t.Logf("Expected:\n%q", rightNormalizedText)
	}
}

func TestNormalizeTextEmptyString(t *testing.T) {
	inputText := ""
	rightNormalizedText := ""
	normalizedText := NormalizeText(inputText)
	if normalizedText != rightNormalizedText {
		t.Errorf("NormalizeText error:\n%q", normalizedText)
		t.Logf("Expected:\n%q", rightNormalizedText)
	}
}

func TestNormalizeTextOnlyWhitespace(t *testing.T) {
	inputText := " \n\t\r  "
	expected := ""
	normalizedText := NormalizeText(inputText)
	if normalizedText != expected {
		t.Errorf("NormalizeText error:\n%q", normalizedText)
		t.Logf("Expected:\n%q", expected)
	}
}
