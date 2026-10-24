package camprocessing

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Reguläre Ausdrücke für die Kamera-Typen (Groß-/Kleinschreibung egal)
var (
	// Pattern A: YYYYMMDDHHMMSS_XXXXXX (z.B. 20260327122939_000194.MP4)
	patternCamA = regexp.MustCompile(`(?i)^(\d{8})\d{6}_\d+\.(mp4|mov|ts|avi)$`)

	// Pattern B: FILEYYMMDD-HHMMSSF (z.B. FILE250923-042437F.MP4)
	patternCamB = regexp.MustCompile(`(?i)^FILE(\d{2})(\d{2})(\d{2})-\d{6}F\.(mp4|mov|ts|avi)$`)

	// Pattern C: YYYY_MMDD_HHMMSS (z.B. 2018_1121_080217.MOV)
	patternCamC = regexp.MustCompile(`(?i)^(\d{4})_(\d{4})_\d{6}\.(mp4|mov|ts|avi)$`)
)

type DayBlock struct {
	DateStr string   // Standardisiertes Format: YYYYMMDD
	Files   []string // Aufsteigend sortierte Dateinamen
}

type ProcessingResult struct {
	CamType string
	Blocks  []DayBlock
	Ignored int // Dateien anderer Kamera-Schemata, die nicht berücksichtigt werden
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
	var winnerCount int

	switch {
	case countA > 0 && countA >= countB && countA >= countC:
		activeMap, winnerCount = dayMapA, countA
		detectedCam = "Cam A (Standard Dashcam)"
	case countB > 0 && countB >= countA && countB >= countC:
		activeMap, winnerCount = dayMapB, countB
		detectedCam = "Cam B (FILE-Schema mit F-Suffix)"
	case countC > 0 && countC >= countA && countC >= countB:
		activeMap, winnerCount = dayMapC, countC
		detectedCam = "Cam C (YYYY_MMDD-Schema)"
	default:
		return nil, errors.New("keine Dateien mit den bekannten Kamera-Mustern (A, B oder C) gefunden")
	}

	// Tage sortieren
	dates := make([]string, 0, len(activeMap))
	for dateStr := range activeMap {
		dates = append(dates, dateStr)
	}
	sort.Strings(dates)

	// Tages-Blöcke aufbauen und Dateinamen chronologisch sortieren
	blocks := make([]DayBlock, 0, len(dates))
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
		Ignored: countA + countB + countC - winnerCount,
	}, nil
}

// CreateConcatList erzeugt die temporäre Textdatei für FFmpeg für einen einzelnen Tagesblock
func CreateConcatList(dirPath string, block DayBlock) (string, error) {
	listFileName := fmt.Sprintf("concat_list_%s.txt", block.DateStr)
	listPath := filepath.Join(dirPath, listFileName)

	var sb strings.Builder
	for _, fileName := range block.Files {
		// Einfache Anführungszeichen für FFmpegs concat-Syntax maskieren: ' -> '\''
		escaped := strings.ReplaceAll(fileName, "'", `'\''`)
		sb.WriteString("file '" + escaped + "'\n")
	}

	if err := os.WriteFile(listPath, []byte(sb.String()), 0644); err != nil {
		return "", fmt.Errorf("Konnte Concat-Liste nicht erstellen: %w", err)
	}

	return listPath, nil
}

// uniqueFileName liefert einen Dateinamen, der im Ordner noch nicht existiert:
// stem+ext, danach stem_(1)+ext, stem_(2)+ext, ...
func uniqueFileName(dirPath, stem, ext string) string {
	name := stem + ext
	for i := 1; ; i++ {
		if _, err := os.Stat(filepath.Join(dirPath, name)); err != nil {
			return name
		}
		name = fmt.Sprintf("%s_(%d)%s", stem, i, ext)
	}
}

// BuildFFmpegCmd liefert Programmname, Argumente und den Namen der Ausgabedatei.
// Beispiel: ffmpeg -f concat -safe 0 -i concat_list_20260327.txt -c copy Kombiniert_2026-03-27.MP4
// Existiert die Ausgabedatei schon, wird ein Zähler angehängt (Kombiniert_2026-03-27_(1).MP4).
func BuildFFmpegCmd(dirPath, listPath, dateStr, extension string) (string, []string, string) {
	// Datum von YYYYMMDD in YYYY-MM-DD umwandeln (sicheres Slicing)
	formattedDate := dateStr
	if len(dateStr) == 8 {
		formattedDate = fmt.Sprintf("%s-%s-%s", dateStr[0:4], dateStr[4:6], dateStr[6:8])
	}

	outputFileName := uniqueFileName(dirPath, "Kombiniert_"+formattedDate, extension)

	args := []string{
		"-hide_banner", "-loglevel", "error", "-nostdin",
		"-f", "concat",
		"-safe", "0",
		"-i", listPath,
		"-c", "copy",
		"-n", // niemals eine vorhandene Datei überschreiben
		filepath.Join(dirPath, outputFileName),
	}

	return "ffmpeg", args, outputFileName
}

// FFmpegAvailable prüft, ob FFmpeg im PATH gefunden wird.
func FFmpegAvailable() bool {
	_, err := exec.LookPath("ffmpeg")
	return err == nil
}
