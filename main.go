package main

import (
	"fmt"
	"os"
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
	a := app.New()
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

// Bestätigungsfenster vor der Ausführung
func openCombineConfirmationWindow(fyneApp fyne.App, dirPath string, parentWin fyne.Window) {
	// Vorab-Analyse für die Bestätigungs-Vorschau
	result, err := camprocessing.AnalyzeAndGroup(dirPath)
	if err != nil {
		dialog.ShowError(err, parentWin)
		return
	}

	confirmWin := fyneApp.NewWindow("Operation bestätigen: Kombinieren")
	confirmWin.Resize(fyne.NewSize(550, 350))

	// Zusammenfassung für den Benutzer aufbauen
	summaryText := fmt.Sprintf("Erkannter Kamera-Typ: %s\n", result.CamType)
	summaryText += fmt.Sprintf("Gefundene Tages-Blöcke: %d\n\n", len(result.Blocks))

	for i, b := range result.Blocks {
		formattedDate := fmt.Sprintf("%s.%s.%s", b.DateStr[6:8], b.DateStr[4:6], b.DateStr[0:4])
		summaryText += fmt.Sprintf("• Block %d (%s): %d Dateien\n", i+1, formattedDate, len(b.Files))
	}

	summaryText += "\nBeim Bestätigen wird die Dateiliste tagesweise strukturiert und eine 'kombinieren_uebersicht.txt' im Arbeitsordner erstellt."

	infoLabel := widget.NewLabel(summaryText)
	infoLabel.Wrapping = fyne.TextWrapWord

	// OK & Abbrechen Buttons
	cancelBtn := widget.NewButton("Abbrechen", func() {
		confirmWin.Close()
	})

	okBtn := widget.NewButton("OK (Ausführen)", func() {
		outPath, err := camprocessing.WriteSummaryFile(dirPath, result)
		confirmWin.Close()

		if err != nil {
			dialog.ShowError(err, parentWin)
		} else {
			dialog.ShowInformation("Erfolg", fmt.Sprintf("Kombinieren-Analyse abgeschlossen!\n\nÜbersicht wurde gespeichert unter:\n%s", outPath), parentWin)
		}
	})
	okBtn.Importance = widget.HighImportance

	buttonRow := container.NewHBox(
		cancelBtn,
		okBtn,
	)

	content := container.NewBorder(
		widget.NewLabelWithStyle("Folgende Operation wird ausgeführt:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewCenter(buttonRow),
		nil, nil,
		container.NewVScroll(infoLabel),
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
