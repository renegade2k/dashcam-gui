package videocut

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

var (
	// Alle Teile außer dem letzten: nur ganze Zahlen (Stunden / Minuten)
	intPartRe = regexp.MustCompile(`^\d{1,6}$`)
	// Letzter Teil: Sekunden, optional mit Nachkommastellen
	secPartRe = regexp.MustCompile(`^\d{1,6}(\.\d{1,3})?$`)
)

// ParseTime wandelt "50", "50.5", "50,5", "1:20" oder "1:02:03" in Sekunden um.
// Alles andere wird abgelehnt, damit nichts Unerwartetes an FFmpeg gelangt.
func ParseTime(s string) (float64, error) {
	s = strings.TrimSpace(strings.ReplaceAll(s, ",", "."))
	if s == "" {
		return 0, errors.New("leere Zeitangabe")
	}

	parts := strings.Split(s, ":")
	if len(parts) > 3 {
		return 0, fmt.Errorf("ungültige Zeitangabe %q (erlaubt: Sekunden, MM:SS oder HH:MM:SS)", s)
	}

	var total float64
	for i, p := range parts {
		isLast := i == len(parts)-1

		re := intPartRe
		if isLast {
			re = secPartRe
		}
		if !re.MatchString(p) {
			return 0, fmt.Errorf("ungültige Zeitangabe %q (erlaubt: Sekunden, MM:SS oder HH:MM:SS)", s)
		}

		v, err := strconv.ParseFloat(p, 64)
		if err != nil {
			return 0, fmt.Errorf("ungültige Zeitangabe %q", s)
		}

		// Bei Angaben mit Doppelpunkt müssen Minuten und Sekunden unter 60 liegen
		if len(parts) > 1 && i > 0 && v >= 60 {
			return 0, fmt.Errorf("ungültige Zeitangabe %q (Minuten/Sekunden müssen unter 60 liegen)", s)
		}

		total = total*60 + v
	}

	return total, nil
}

// ValidateRange prüft Start und Ende und liefert Startzeit und Dauer in Sekunden.
func ValidateRange(startTime, stopTime string) (start, duration float64, err error) {
	start, err = ParseTime(startTime)
	if err != nil {
		return 0, 0, fmt.Errorf("Startzeit: %w", err)
	}

	stop, err := ParseTime(stopTime)
	if err != nil {
		return 0, 0, fmt.Errorf("Endzeit: %w", err)
	}

	if stop <= start {
		return 0, 0, errors.New("Die Endzeit muss nach der Startzeit liegen")
	}

	return start, stop - start, nil
}

// ProcessCut führt den verlustfreien Schnitt via FFmpeg durch.
// Hinweis: Mit "-c copy" wird nur an Keyframes geschnitten, Start und Ende
// sind daher nicht auf die Sekunde genau.
func ProcessCut(inputPath, outputPath, startTime, stopTime string) error {
	start, duration, err := ValidateRange(startTime, stopTime)
	if err != nil {
		return err
	}

	cmd := exec.Command("ffmpeg",
		"-hide_banner", "-loglevel", "error", "-nostdin",
		"-n", // niemals eine vorhandene Datei überschreiben
		"-ss", strconv.FormatFloat(start, 'f', 3, 64),
		"-i", inputPath,
		"-t", strconv.FormatFloat(duration, 'f', 3, 64),
		"-c", "copy",
		outputPath,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return errors.New("FFmpeg wurde nicht gefunden. Bitte installieren und im PATH verfügbar machen")
		}
		return fmt.Errorf("%v\nFFmpeg-Ausgabe: %s", err, strings.TrimSpace(string(output)))
	}

	// Sicherheitsnetz: FFmpeg kann sich mit Exit-Code 0 beenden, ohne etwas zu
	// schreiben. Daher prüfen, ob die Ausgabedatei wirklich existiert.
	if info, statErr := os.Stat(outputPath); statErr != nil || info.Size() == 0 {
		return fmt.Errorf("FFmpeg hat keine Ausgabedatei erzeugt\nFFmpeg-Ausgabe: %s", strings.TrimSpace(string(output)))
	}

	return nil
}
