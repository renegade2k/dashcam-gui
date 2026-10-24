package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"dashcam-gui/camprocessing"
	"dashcam-gui/videocut"
)

func main() {
	a := app.NewWithID("com.dashcam.gui")
	w := a.NewWindow("Dashcam GUI")
	w.Resize(fyne.NewSize(850, 480))

	var workDir string
	ffmpegOK := camprocessing.FFmpegAvailable()

	pathLabel := widget.NewLabel("Kein Arbeitsordner ausgewählt")
	pathLabel.TextStyle = fyne.TextStyle{Bold: true}

	showFilesBtn := widget.NewButton("Videos anzeigen", func() {
		openFileListWindow(a, workDir)
	})
	showFilesBtn.Disable()

	openFolderBtn := widget.NewButton("Arbeitsordner öffnen", func() {
		openWorkingFolder(workDir)
	})
	openFolderBtn.Disable()

	combineBtn := widget.NewButton("Kombinieren", func() {
		openCombineConfirmationWindow(a, workDir, w)
	})
	combineBtn.Disable()

	cutVideoBtn := widget.NewButton("Video schneiden", func() {
		openCutWindow(a, workDir, w)
	})
	cutVideoBtn.Disable()

	selectBtn := widget.NewButton("Arbeitsordner wählen...", func() {
		folderDialog := dialog.NewFolderOpen(func(uri fyne.ListableURI, err error) {
			if err != nil || uri == nil {
				return
			}
			workDir = uri.Path()
			pathLabel.SetText("Aktueller Pfad: " + workDir)

			showFilesBtn.Enable()
			openFolderBtn.Enable()
			// Kombinieren und Schneiden brauchen FFmpeg
			if ffmpegOK {
				combineBtn.Enable()
				cutVideoBtn.Enable()
			}
		}, w)
		folderDialog.Show()
	})

	actionRow := container.NewBorder(nil, nil, nil, showFilesBtn, openFolderBtn)

	content := container.NewVBox(
		widget.NewLabel("Schritt 1: Wähle den Ordner mit deinen Dashcam-Aufnahmen"),
		selectBtn,
		pathLabel,
		actionRow,
		widget.NewSeparator(), // Visuelle Trennlinie zwischen Schritt 1 und 2
		widget.NewLabel("Schritt 2: Operationen durchführen"),
		combineBtn,
		cutVideoBtn,
	)

	if !ffmpegOK {
		warn := widget.NewLabel("⚠ FFmpeg wurde nicht gefunden. Bitte installieren und im PATH verfügbar machen, dann die App neu starten.")
		warn.Wrapping = fyne.TextWrapWord
		warn.Importance = widget.WarningImportance
		content.Add(warn)
	}

	w.SetContent(container.NewPadded(content))
	w.ShowAndRun()
}

// Öffnet den System-Dateimanager plattformunabhängig
func openWorkingFolder(path string) {
	if path == "" {
		return
	}
	cleanPath := filepath.FromSlash(filepath.Clean(path))

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("explorer", cleanPath)
	case "linux":
		cmd = exec.Command("xdg-open", cleanPath)
	default:
		cmd = exec.Command("open", cleanPath)
	}

	if err := cmd.Start(); err != nil {
		return
	}
	// Prozess im Hintergrund einsammeln, damit keine Zombie-Prozesse entstehen
	go func() { _ = cmd.Wait() }()
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

// tail kürzt lange Texte (z.B. FFmpeg-Ausgaben) auf die letzten n Zeichen
func tail(s string, n int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= n {
		return string(r)
	}
	return "…" + string(r[len(r)-n:])
}

// Fenster für den Videoschnitt
func openCutWindow(fyneApp fyne.App, dirPath string, parentWin fyne.Window) {
	videos := loadVideoFiles(dirPath)
	if len(videos) == 0 {
		dialog.ShowInformation("Hinweis", "Keine Videodateien im Ordner gefunden.", parentWin)
		return
	}

	cutWin := fyneApp.NewWindow("Video schneiden")
	cutWin.Resize(fyne.NewSize(600, 450))

	var selectedFile string

	videoList := widget.NewList(
		func() int { return len(videos) },
		func() fyne.CanvasObject { return widget.NewLabel("Template Video.mp4") },
		func(i widget.ListItemID, o fyne.CanvasObject) {
			o.(*widget.Label).SetText(videos[i])
		},
	)
	videoList.OnSelected = func(id widget.ListItemID) {
		selectedFile = videos[id]
	}
	videoList.OnUnselected = func(id widget.ListItemID) {
		selectedFile = ""
	}

	startEntry := widget.NewEntry()
	startEntry.SetPlaceHolder("Start (z.B. 50 oder 1:20)")

	stopEntry := widget.NewEntry()
	stopEntry.SetPlaceHolder("Ende (z.B. 120 oder 2:45)")

	executeCutBtn := widget.NewButton("Ausschnitt exportieren", func() {
		if selectedFile == "" {
			dialog.ShowInformation("Hinweis", "Bitte wähle ein Video aus der Liste aus.", cutWin)
			return
		}

		startText := strings.TrimSpace(startEntry.Text)
		stopText := strings.TrimSpace(stopEntry.Text)
		if startText == "" || stopText == "" {
			dialog.ShowInformation("Hinweis", "Bitte Start- und Endzeit angeben.", cutWin)
			return
		}

		// Eingaben prüfen, solange das Fenster noch offen ist
		if _, _, err := videocut.ValidateRange(startText, stopText); err != nil {
			dialog.ShowError(err, cutWin)
			return
		}

		// Nach der Prüfung enthalten die Eingaben nur noch Ziffern, ':', '.' und ','
		nameSafe := strings.NewReplacer(":", "-", ",", ".")
		cleanStart := nameSafe.Replace(startText)
		cleanStop := nameSafe.Replace(stopText)

		ext := filepath.Ext(selectedFile)
		baseName := strings.TrimSuffix(selectedFile, ext)

		outName := uniqueFileName(dirPath, fmt.Sprintf("%s_cut_%s_bis_%s", baseName, cleanStart, cleanStop), ext)
		outputPath := filepath.Join(dirPath, outName)
		inputPath := filepath.Join(dirPath, selectedFile)

		cutWin.Close()

		// Ladeindikator einblenden
		progressDialog := showProgressDialog("Video wird geschnitten...", parentWin)

		go func() {
			err := videocut.ProcessCut(inputPath, outputPath, startText, stopText)

			// UI-Änderungen müssen im Fyne-Hauptthread laufen
			fyne.Do(func() {
				progressDialog.Hide()
				if err != nil {
					dialog.ShowError(fmt.Errorf("Fehler beim Schneiden:\n%v", err), parentWin)
				} else {
					dialog.ShowInformation("Erfolg", fmt.Sprintf("Ausschnitt erfolgreich gespeichert als:\n%s", outName), parentWin)
				}
			})
		}()
	})
	executeCutBtn.Importance = widget.HighImportance

	topLabel := widget.NewLabelWithStyle("1. Video in der Liste markieren:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

	inputForm := container.NewVBox(
		widget.NewLabelWithStyle("2. Zeiten festlegen (Sekunden oder MM:SS):", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewGridWithColumns(2, startEntry, stopEntry),
		widget.NewLabel("Hinweis: Der Schnitt erfolgt verlustfrei an Keyframes und ist daher nicht sekundengenau."),
		widget.NewSeparator(),
		executeCutBtn,
	)

	content := container.NewBorder(
		topLabel,
		inputForm,
		nil, nil,
		videoList,
	)

	cutWin.SetContent(container.NewPadded(content))
	cutWin.Show()
}

// Bestätigungsfenster vor der Ausführung mit Checkbox-Auswahl
func openCombineConfirmationWindow(fyneApp fyne.App, dirPath string, parentWin fyne.Window) {
	result, err := camprocessing.AnalyzeAndGroup(dirPath)
	if err != nil {
		dialog.ShowError(err, parentWin)
		return
	}

	confirmWin := fyneApp.NewWindow("Operation bestätigen: Kombinieren")
	confirmWin.Resize(fyne.NewSize(600, 480))

	checks := make([]*widget.Check, len(result.Blocks))
	checkListContainer := container.NewVBox()

	for i, b := range result.Blocks {
		formattedDate := fmt.Sprintf("%s.%s.%s", b.DateStr[6:8], b.DateStr[4:6], b.DateStr[0:4])

		var labelText string
		if len(b.Files) == 1 {
			labelText = fmt.Sprintf("Block %d (%s): 1 Datei (Einzeldatei)", i+1, formattedDate)
		} else {
			labelText = fmt.Sprintf("Block %d (%s): %d Dateien", i+1, formattedDate, len(b.Files))
		}

		chk := widget.NewCheck(labelText, nil)
		// Blöcke mit nur einer Datei sind standardmäßig abgewählt
		chk.SetChecked(len(b.Files) >= 2)

		checks[i] = chk
		checkListContainer.Add(chk)
	}

	// Sind alle ausgewählt, werden alle abgewählt – sonst werden alle ausgewählt
	selectAllBtn := widget.NewButton("Alle / Keine auswählen", func() {
		allChecked := true
		for _, chk := range checks {
			if !chk.Checked {
				allChecked = false
				break
			}
		}
		for _, chk := range checks {
			chk.SetChecked(!allChecked)
		}
	})

	// Checkbox für das automatische Verschieben verarbeiteter Rohdateien
	moveFilesCheck := widget.NewCheck("Quelldateien nach dem Kombinieren in Ordner 'verarbeitet' verschieben", nil)
	moveFilesCheck.SetChecked(false)

	cancelBtn := widget.NewButton("Abbrechen", func() {
		confirmWin.Close()
	})

	okBtn := widget.NewButton("OK (Ausführen)", func() {
		var selectedBlocks []camprocessing.DayBlock
		for i, b := range result.Blocks {
			if checks[i].Checked {
				selectedBlocks = append(selectedBlocks, b)
			}
		}

		if len(selectedBlocks) == 0 {
			dialog.ShowInformation("Hinweis", "Bitte wähle mindestens einen Tagesblock zum Kombinieren aus.", confirmWin)
			return
		}

		shouldMove := moveFilesCheck.Checked

		confirmWin.Close()

		// Ladeindikator einblenden
		progressDialog := showProgressDialog("Videos werden zusammengefügt...", parentWin)

		go func() {
			var failures []string
			var outputs []string
			movedAll := true

			for _, block := range selectedBlocks {
				listPath, err := camprocessing.CreateConcatList(dirPath, block)
				if err != nil {
					failures = append(failures, fmt.Sprintf("Tag %s: %v", block.DateStr, err))
					continue
				}

				ext := filepath.Ext(block.Files[0])
				cmdName, args, outName := camprocessing.BuildFFmpegCmd(dirPath, listPath, block.DateStr, ext)
				cmd := exec.Command(cmdName, args...)

				output, err := cmd.CombinedOutput()
				_ = os.Remove(listPath)

				if err != nil {
					failures = append(failures, fmt.Sprintf("Tag %s: %v\nFFmpeg-Ausgabe: %s", block.DateStr, err, tail(string(output), 600)))
					continue
				}

				// Sicherheitsnetz: Ausgabedatei muss wirklich existieren
				if info, statErr := os.Stat(filepath.Join(dirPath, outName)); statErr != nil || info.Size() == 0 {
					failures = append(failures, fmt.Sprintf("Tag %s: FFmpeg hat keine Ausgabedatei erzeugt\nFFmpeg-Ausgabe: %s", block.DateStr, tail(string(output), 600)))
					continue
				}

				outputs = append(outputs, outName)

				// Wenn gewünscht, Quelldateien nach "verarbeitet" verschieben
				if shouldMove {
					if err := moveFilesToSubfolder(dirPath, block.Files, "verarbeitet"); err != nil {
						movedAll = false
						failures = append(failures, fmt.Sprintf("Tag %s: Quelldateien konnten nicht (vollständig) verschoben werden:\n%v", block.DateStr, err))
					}
				}
			}

			// UI-Änderungen müssen im Fyne-Hauptthread laufen
			fyne.Do(func() {
				progressDialog.Hide()

				created := ""
				if len(outputs) > 0 {
					created = fmt.Sprintf("Erstellt (%d):\n%s", len(outputs), strings.Join(outputs, "\n"))
				}

				if len(failures) > 0 {
					msg := "Fehler bei der Ausführung:\n" + strings.Join(failures, "\n")
					if created != "" {
						msg += "\n\n" + created
					}
					dialog.ShowError(errors.New(msg), parentWin)
					return
				}

				msg := fmt.Sprintf("Erfolgreich %d ausgewählte(n) Tagesblock/Blöcke zusammengefügt!\n\n%s", len(outputs), created)
				if shouldMove && movedAll {
					msg += "\n\nDie Quelldateien wurden in den Ordner 'verarbeitet' verschoben."
				}
				dialog.ShowInformation("Erfolg", msg, parentWin)
			})
		}()
	})
	okBtn.Importance = widget.HighImportance

	headerText := fmt.Sprintf("Erkannter Kamera-Typ: %s\nArbeitsordner: %s", result.CamType, dirPath)
	if result.Ignored > 0 {
		headerText += fmt.Sprintf("\nHinweis: %d Datei(en) anderer Kamera-Schemata werden ignoriert.", result.Ignored)
	}
	headerText += "\n\nWähle die Tagesblöcke aus, die zusammengefügt werden sollen:"
	headerLabel := widget.NewLabel(headerText)
	headerLabel.Wrapping = fyne.TextWrapWord

	topBox := container.NewVBox(
		headerLabel,
		selectAllBtn,
		widget.NewSeparator(),
	)

	bottomBox := container.NewVBox(
		widget.NewSeparator(),
		moveFilesCheck,
		container.NewCenter(container.NewHBox(cancelBtn, okBtn)),
	)

	content := container.NewBorder(
		topBox,
		bottomBox,
		nil, nil,
		container.NewVScroll(checkListContainer),
	)

	confirmWin.SetContent(container.NewPadded(content))
	confirmWin.Show()
}

// Erstellt und zeigt einen Lade-Dialog mit unendlicher Progressbar
func showProgressDialog(message string, parentWin fyne.Window) dialog.Dialog {
	progressBar := widget.NewProgressBarInfinite()
	label := widget.NewLabel(message)
	label.TextStyle = fyne.TextStyle{Bold: true}

	content := container.NewVBox(
		label,
		widget.NewLabel("Bitte warten, FFmpeg arbeitet im Hintergrund..."),
		progressBar,
	)

	d := dialog.NewCustomWithoutButtons("Verarbeitung läuft", content, parentWin)
	d.Show()
	return d
}

// Verschiebt eine Liste von Dateien in einen Unterordner.
// Vorhandene Dateien im Zielordner werden nicht überschrieben.
// Alle aufgetretenen Fehler werden gesammelt zurückgegeben.
func moveFilesToSubfolder(dirPath string, filenames []string, subfolder string) error {
	targetDir := filepath.Join(dirPath, subfolder)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return err
	}

	var errs []error
	for _, fname := range filenames {
		oldPath := filepath.Join(dirPath, fname)
		newPath := filepath.Join(targetDir, fname)

		if _, err := os.Stat(newPath); err == nil {
			errs = append(errs, fmt.Errorf("%s: existiert im Ordner '%s' bereits", fname, subfolder))
			continue
		}
		if err := os.Rename(oldPath, newPath); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", fname, err))
		}
	}

	return errors.Join(errs...)
}

// Öffnet Popup zur Dateianzeige
func openFileListWindow(fyneApp fyne.App, dirPath string) {
	videoFiles := loadVideoFiles(dirPath)

	listWin := fyneApp.NewWindow("Gefundene Videos - " + filepath.Base(dirPath))
	listWin.Resize(fyne.NewSize(500, 400))

	header := widget.NewLabel(fmt.Sprintf("%d Videodatei(en) gefunden in:\n%s", len(videoFiles), dirPath))
	header.TextStyle = fyne.TextStyle{Italic: true}

	fileList := widget.NewList(
		func() int { return len(videoFiles) },
		func() fyne.CanvasObject { return widget.NewLabel("Template Video.mp4") },
		func(i widget.ListItemID, o fyne.CanvasObject) { o.(*widget.Label).SetText(videoFiles[i]) },
	)

	listContent := container.NewBorder(
		container.NewVBox(header, widget.NewSeparator()),
		nil, nil, nil, fileList,
	)

	listWin.SetContent(container.NewPadded(listContent))
	listWin.Show()
}

// Liest den Pfad neutral aus und zeigt ALLE Videos an
func loadVideoFiles(dirPath string) []string {
	var files []string

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return files
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext == ".mp4" || ext == ".mov" || ext == ".ts" || ext == ".avi" {
			files = append(files, entry.Name())
		}
	}

	return files
}
