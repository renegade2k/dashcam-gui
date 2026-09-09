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
)

var currentWorkDir string

func main() {
	a := app.New()
	w := a.NewWindow("Dashcam GUI")
	w.Resize(fyne.NewSize(850, 400))

	// Pfad-Label
	pathLabel := widget.NewLabel("Kein Arbeitsordner ausgewählt")
	pathLabel.TextStyle = fyne.TextStyle{Bold: true}

	// Button zum Öffnen der Dateiliste in einem neuen Fenster (anfangs deaktiviert)
	showFilesBtn := widget.NewButton("Videos anzeigen", func() {
		openFileListWindow(a, currentWorkDir)
	})
	showFilesBtn.Disable()

	// Button zur Ordnerauswahl
	selectBtn := widget.NewButton("Arbeitsordner wählen...", func() {
		folderDialog := dialog.NewFolderOpen(func(uri fyne.ListableURI, err error) {
			if err != nil || uri == nil {
				return
			}

			// Pfad in Variable sichern
			currentWorkDir = uri.Path()
			pathLabel.SetText("Aktueller Pfad: " + currentWorkDir)

			// Button aktivieren, sobald ein Pfad feststeht
			showFilesBtn.Enable()
		}, w)

		folderDialog.Show()
	})

	// Zeile 1: Ordner-Auswahl-Button
	// Zeile 2: Pfad-Anzeige (links) & "Videos anzeigen"-Button (rechts)
	pathRow := container.NewBorder(nil, nil, nil, showFilesBtn, pathLabel)

	content := container.NewVBox(
		widget.NewLabel("Schritt 1: Wähle den Ordner mit deinen Dashcam-Aufnahmen"),
		selectBtn,
		widget.NewSeparator(),
		pathRow,
	)

	w.SetContent(container.NewPadded(content))
	w.ShowAndRun()
}

// Öffnet ein separates Fenster mit der Liste aller gefundenen Videos
func openFileListWindow(fyneApp fyne.App, dirPath string) {
	videoFiles := loadVideoFiles(dirPath)

	listWin := fyneApp.NewWindow("Gefundene Videos - " + filepath.Base(dirPath))
	listWin.Resize(fyne.NewSize(500, 400))

	header := widget.NewLabel(fmt.Sprintf("%d Videodatei(en) gefunden in:\n%s", len(videoFiles), dirPath))
	header.TextStyle = fyne.TextStyle{Italic: true}

	fileList := widget.NewList(
		func() int {
			return len(videoFiles)
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("Template Video.mp4")
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			o.(*widget.Label).SetText(videoFiles[i])
		},
	)

	listContent := container.NewBorder(
		container.NewVBox(header, widget.NewSeparator()),
		nil, nil, nil,
		fileList,
	)

	listWin.SetContent(container.NewPadded(listContent))
	listWin.Show()
}

// Liest den Pfad aus und filtert nach Videoendungen
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
