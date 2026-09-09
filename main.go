package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"dashcam-gui/camprocessing"
)

var currentWorkDir string

func main() {
	a := app.NewWithID("com.renegade2k.dashcamgui")
	w := a.NewWindow("Dashcam GUI")
	w.Resize(fyne.NewSize(850, 450))

	// Pfad-Label
	pathLabel := widget.NewLabel("Kein Arbeitsordner ausgewählt")
	pathLabel.TextStyle = fyne.TextStyle{Bold: true}

	// Button: Dateiliste-Popup (anfangs deaktiviert)
	showFilesBtn := widget.NewButton("Videos anzeigen", func() {
		openFileListWindow(a, currentWorkDir)
	})
	showFilesBtn.Disable()

	// NEW: Button "Kombinieren" (anfangs deaktiviert)
	combineBtn := widget.NewButton("Kombinieren", func() {
		openCombineConfirmationWindow(a, currentWorkDir, w)
	})
	combineBtn.Disable()

	// Button: Ordnerauswahl
	selectBtn := widget.NewButton("Arbeitsordner wählen...", func() {
		folderDialog := dialog.NewFolderOpen(func(uri fyne.ListableURI, err error) {
			if err != nil || uri == nil {
				return
			}

			currentWorkDir = uri.Path()
			pathLabel.SetText("Aktueller Pfad: " + currentWorkDir)

			showFilesBtn.Enable()
			combineBtn.Enable()
		}, w)

		folderDialog.Show()
	})

	pathRow := container.NewBorder(nil, nil, nil, showFilesBtn, pathLabel)

	content := container.NewVBox(
		widget.NewLabel("Schritt 1: Wähle den Ordner mit deinen Dashcam-Aufnahmen"),
		selectBtn,
		widget.NewSeparator(),
		pathRow,
		widget.NewSeparator(),
		widget.NewLabel("Schritt 2: Operationen durchführen"),
		combineBtn,
	)

	w.SetContent(container.NewPadded(content))
	w.ShowAndRun()
}

// Bestätigungsfenster vor der Ausführung mit Checkbox-Auswahl
func openCombineConfirmationWindow(fyneApp fyne.App, dirPath string, parentWin fyne.Window) {
	result, err := camprocessing.AnalyzeAndGroup(dirPath)
	if err != nil {
		dialog.ShowError(err, parentWin)
		return
	}

	confirmWin := fyneApp.NewWindow("Operation bestätigen: Kombinieren")
	confirmWin.Resize(fyne.NewSize(600, 450))

	// Map / Liste zur Nachverfolgung der Checkboxen
	// Key: Index des Blocks, Value: Pointer zur Checkbox
	checkMap := make(map[int]*widget.Check)

	// Container für die vertikale Liste der Checkboxen
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

		// Vorauswahl: Blöcke mit >= 2 Dateien aktivieren, Einzeldateien deaktivieren
		if len(b.Files) >= 2 {
			chk.SetChecked(true)
		} else {
			chk.SetChecked(false)
		}

		checkMap[i] = chk
		checkListContainer.Add(chk)
	}

	// Button zum schnellen Auswählen / Abwählen aller Häkchen
	allSelected := true
	selectAllBtn := widget.NewButton("Alle / Keine auswählen", func() {
		allSelected = !allSelected
		for _, chk := range checkMap {
			chk.SetChecked(allSelected)
		}
	})

	// OK & Abbrechen Buttons
	cancelBtn := widget.NewButton("Abbrechen", func() {
		confirmWin.Close()
	})

	okBtn := widget.NewButton("OK (Ausführen)", func() {
		// Ermitteln, welche Blöcke angehakt wurden
		var selectedBlocks []camprocessing.DayBlock
		for i, b := range result.Blocks {
			if chk, ok := checkMap[i]; ok && chk.Checked {
				selectedBlocks = append(selectedBlocks, b)
			}
		}

		if len(selectedBlocks) == 0 {
			dialog.ShowInformation("Hinweis", "Bitte wähle mindestens einen Tagesblock zum Kombinieren aus.", confirmWin)
			return
		}

		confirmWin.Close()

		go func() {
			var errors []string
			processedBlocks := 0

			for _, block := range selectedBlocks {
				// 1. Concat-Liste für den Tag schreiben
				listPath, err := camprocessing.CreateConcatList(dirPath, block)
				if err != nil {
					errors = append(errors, fmt.Sprintf("Tag %s: %v", block.DateStr, err))
					continue
				}

				// Endung der ersten Datei ermitteln (.mp4 / .mov)
				ext := filepath.Ext(block.Files[0])

				// 2. FFmpeg Command vorbereiten
				cmdName, args := camprocessing.BuildFFmpegCmd(dirPath, listPath, block.DateStr, ext)
				cmd := exec.Command(cmdName, args...)

				// Ausführen
				output, err := cmd.CombinedOutput()

				// Temporäre Liste nach Aufruf wieder löschen
				_ = os.Remove(listPath)

				if err != nil {
					errors = append(errors, fmt.Sprintf("Tag %s Fehler: %v\nOutput: %s", block.DateStr, err, string(output)))
				} else {
					processedBlocks++
				}
			}

			// Ergebnis anzeigen
			if len(errors) > 0 {
				dialog.ShowError(fmt.Errorf("Fehler bei der Ausführung:\n%s", strings.Join(errors, "\n")), parentWin)
			} else {
				dialog.ShowInformation("Erfolg", fmt.Sprintf("Erfolgreich %d ausgewählte(n) Tagesblock/Blöcke zusammengefügt!", processedBlocks), parentWin)
			}
		}()
	})
	okBtn.Importance = widget.HighImportance

	headerText := fmt.Sprintf("Erkannter Kamera-Typ: %s\nArbeitsordner: %s\n\nWähle die Tagesblöcke aus, die zusammengefügt werden sollen:", result.CamType, dirPath)
	headerLabel := widget.NewLabel(headerText)
	headerLabel.Wrapping = fyne.TextWrapWord

	topBox := container.NewVBox(
		headerLabel,
		selectAllBtn,
		widget.NewSeparator(),
	)

	buttonRow := container.NewHBox(
		cancelBtn,
		okBtn,
	)

	content := container.NewBorder(
		topBox,
		container.NewCenter(buttonRow),
		nil, nil,
		container.NewVScroll(checkListContainer),
	)

	confirmWin.SetContent(container.NewPadded(content))
	confirmWin.Show()
}

// Öffnet Popup zur Dateianzeige (unverändert)
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

// Liest den Pfad neutral aus und zeigt ALLE Videos an (unabhängig vom Schema)
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
