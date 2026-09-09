package camprocessing

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
)

// Reguläre Ausdrücke für die Kamera-Typen
var (
	// Pattern A: YYYYMMDDHHMMSS_XXXXXX (z.B. 20260327122939_000194.MP4)
	patternCamA = regexp.MustCompile(`^(\d{8})\d{6}_\d+\.(mp4|mov|ts|avi|MP4|MOV|TS|AVI)$`)

	// Pattern B: FILEYYMMDD-HHMMSSF (z.B. FILE250923-042437F.MP4)
	patternCamB = regexp.MustCompile(`^FILE(\d{2})(\d{2})(\d{2})-\d{6}F\.(mp4|mov|ts|avi|MP4|MOV|TS|AVI)$`)

	// Pattern C: YYYY_MMDD_HHMMSS (z.B. 2018_1121_080217.MOV)
	patternCamC = regexp.MustCompile(`^(\d{4})_(\d{4})_\d{6}\.(mp4|mov|ts|avi|MP4|MOV|TS|AVI)$`)
)

type DayBlock struct {
	DateStr string   // Standardisiertes Format: YYYYMMDD
	Files   []string // Aufsteigend sortierte Dateinamen
}

type ProcessingResult struct {
	CamType string
	Blocks  []DayBlock
}

func AnalyzeAndGroup(dirPath string) (*ProcessingResult, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("Fehler beim Lesen des Ordners: %w", err)
	}

	// Zwischenspeicher für Treffer pro Kameraschema
	dayMapA := make(map[string][]string)
	dayMapB := make(map[string][]string)
	dayMapC := make(map[string][]string)

	countA, countB, countC := 0, 0, 0

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		filename := entry.Name()

		// 1. Prüfe Cam A
		if matches := patternCamA.FindStringSubmatch(filename); len(matches) >= 2 {
			datePart := matches[1] // YYYYMMDD
			dayMapA[datePart] = append(dayMapA[datePart], filename)
			countA++
			continue
		}

		// 2. Prüfe Cam B (FILEYYMMDD...)
		if matches := patternCamB.FindStringSubmatch(filename); len(matches) >= 4 {
			yy, mm, dd := matches[1], matches[2], matches[3]
			datePart := "20" + yy + mm + dd // Umwandeln in YYYYMMDD
			dayMapB[datePart] = append(dayMapB[datePart], filename)
			countB++
			continue
		}

		// 3. Prüfe Cam C (YYYY_MMDD_...)
		if matches := patternCamC.FindStringSubmatch(filename); len(matches) >= 3 {
			yyyy, mmdd := matches[1], matches[2]
			datePart := yyyy + mmdd // Umwandeln in YYYYMMDD
			dayMapC[datePart] = append(dayMapC[datePart], filename)
			countC++
			continue
		}
	}

	// Bestimme den Gewinner-Typ anhand der meisten Treffer
	var activeMap map[string][]string
	var detectedCam string

	if countA > 0 && countA >= countB && countA >= countC {
		activeMap = dayMapA
		detectedCam = "Cam A (Standard Dashcam)"
	} else if countB > 0 && countB >= countA && countB >= countC {
		activeMap = dayMapB
		detectedCam = "Cam B (FILE-Schema mit F-Suffix)"
	} else if countC > 0 && countC >= countA && countC >= countB {
		activeMap = dayMapC
		detectedCam = "Cam C (YYYY_MMDD-Schema)"
	} else {
		return nil, fmt.Errorf("keine Dateien mit den bekannten Kamera-Mustern (A, B oder C) gefunden")
	}

	// Tage sortieren
	var dates []string
	for dateStr := range activeMap {
		dates = append(dates, dateStr)
	}
	sort.Strings(dates)

	// Tages-Blöcke aufbauen und Dateinamen chronologisch sortieren
	var blocks []DayBlock
	for _, dateStr := range dates {
		files := activeMap[dateStr]
		sort.Strings(files)

		blocks = append(blocks, DayBlock{
			DateStr: dateStr,
			Files:   files,
		})
	}

	return &ProcessingResult{
		CamType: detectedCam,
		Blocks:  blocks,
	}, nil
}

func WriteSummaryFile(dirPath string, result *ProcessingResult) (string, error) {
	outputPath := filepath.Join(dirPath, "kombinieren_uebersicht.txt")
	file, err := os.Create(outputPath)
	if err != nil {
		return "", fmt.Errorf("Konnte Übersicht-Datei nicht erstellen: %w", err)
	}
	defer file.Close()

	fmt.Fprintf(file, "=== DASHCAM BATCH PROCESSING ÜBERSICHT ===\n")
	fmt.Fprintf(file, "Erkannte Kamera: %s\n", result.CamType)
	fmt.Fprintf(file, "Arbeitsordner: %s\n", dirPath)
	fmt.Fprintf(file, "Gefundene Tagesblöcke: %d\n\n", len(result.Blocks))

	for i, block := range result.Blocks {
		formattedDate := fmt.Sprintf("%s.%s.%s", block.DateStr[6:8], block.DateStr[4:6], block.DateStr[0:4])
		fmt.Fprintf(file, "--- Block %d: Tag %s [%d Dateien] ---\n", i+1, formattedDate, len(block.Files))
		for _, fileName := range block.Files {
			fmt.Fprintf(file, "  - %s\n", fileName)
		}
		fmt.Fprintf(file, "\n")
	}

	return outputPath, nil
}

// CreateConcatList erzeugt die temporäre Textdatei für FFmpeg für einen einzelnen Tagesblock
func CreateConcatList(dirPath string, block DayBlock) (string, error) {
	listFileName := fmt.Sprintf("concat_list_%s.txt", block.DateStr)
	listPath := filepath.Join(dirPath, listFileName)

	file, err := os.Create(listPath)
	if err != nil {
		return "", fmt.Errorf("Konnte Concat-Liste nicht erstellen: %w", err)
	}
	defer file.Close()

	for _, fileName := range block.Files {
		// Single quotes Maskierung für sichere Pfade in FFmpeg
		fmt.Fprintf(file, "file '%s'\n", fileName)
	}

	return listPath, nil
}

// BuildFFmpegCmd liefert die Argumente für den FFmpeg-Aufruf
// Beispiel: ffmpeg -f concat -safe 0 -i concat_list_20260327.txt -c copy Kombiniert_2026-03-27.MP4
func BuildFFmpegCmd(dirPath, listPath, dateStr, extension string) (string, []string) {
	// Datum von YYYYMMDD in YYYY-MM-DD umwandeln (sicheres Slicing)
	formattedDate := dateStr
	if len(dateStr) == 8 {
		formattedDate = fmt.Sprintf("%s-%s-%s", dateStr[0:4], dateStr[4:6], dateStr[6:8])
	}

	outputFileName := fmt.Sprintf("Kombiniert_%s%s", formattedDate, extension)

	args := []string{
		"-f", "concat",
		"-safe", "0",
		"-i", listPath,
		"-c", "copy",
		filepath.Join(dirPath, outputFileName),
	}

	return "ffmpeg", args
}
