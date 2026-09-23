package main

import (
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

var currentWorkDir string

func main() {
	a := app.NewWithID("com.dashcam.gui")
	w := a.NewWindow("Dashcam GUI")
	w.Resize(fyne.NewSize(850, 480))

	pathLabel := widget.NewLabel("Kein Arbeitsordner ausgewählt")
	pathLabel.TextStyle = fyne.TextStyle{Bold: true}

	showFilesBtn := widget.NewButton("Videos anzeigen", func() {
		openFileListWindow(a, currentWorkDir)
	})
	showFilesBtn.Disable()

	openFolderBtn := widget.NewButton("Arbeitsordner öffnen", func() {
		openWorkingFolder(currentWorkDir)
	})
	openFolderBtn.Disable()

	combineBtn := widget.NewButton("Kombinieren", func() {
		openCombineConfirmationWindow(a, currentWorkDir, w)
	})
	combineBtn.Disable()

	cutVideoBtn := widget.NewButton("Video schneiden", func() {
		openCutWindow(a, currentWorkDir, w)
	})
	cutVideoBtn.Disable()

	selectBtn := widget.NewButton("Arbeitsordner wählen...", func() {
		folderDialog := dialog.NewFolderOpen(func(uri fyne.ListableURI, err error) {
			if err != nil || uri == nil {
				return
			}
			currentWorkDir = uri.Path()
			pathLabel.SetText("Aktueller Pfad: " + currentWorkDir)

			showFilesBtn.Enable()
			combineBtn.Enable()
			openFolderBtn.Enable()
			cutVideoBtn.Enable()
		}, w)
		folderDialog.Show()
	})

	// Layout-Zeile unter der Pfadanzeige: "Arbeitsordner öffnen" nimmt den Hauptplatz ein, "Videos anzeigen" liegt rechts
	actionRow := container.NewBorder(nil, nil, nil, showFilesBtn, openFolderBtn)

	content := container.NewVBox(
		widget.NewLabel("Schritt 1: Wähle den Ordner mit deinen Dashcam-Aufnahmen"),
		selectBtn,
		pathLabel,
		actionRow,
		widget.NewSeparator(),
		widget.NewSeparator(),
		widget.NewLabel("Schritt 2: Operationen durchführen"),
		combineBtn,
		cutVideoBtn,
	)

	w.SetContent(container.NewPadded(content))
	w.ShowAndRun()
}

// Öffnet den System-Dateimanager plattformunabhängig
func openWorkingFolder(path string) {
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
	_ = cmd.Start()
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

	startEntry := widget.NewEntry()
	startEntry.SetPlaceHolder("Start (z.B. 50 oder 1:20)")

	stopEntry := widget.NewEntry()
	stopEntry.SetPlaceHolder("Ende (z.B. 120 oder 2:45)")

	executeCutBtn := widget.NewButton("Ausschnitt exportieren", func() {
		if selectedFile == "" {
			dialog.ShowInformation("Hinweis", "Bitte wähle ein Video aus der Liste aus.", cutWin)
			return
		}
		if startEntry.Text == "" || stopEntry.Text == "" {
			dialog.ShowInformation("Hinweis", "Bitte Start- und Endzeit angeben.", cutWin)
			return
		}

		// Bereinigen der Zeitangaben für den Dateinamen (z.B. "1:20" -> "1-20")
		cleanStart := strings.ReplaceAll(strings.TrimSpace(startEntry.Text), ":", "-")
		cleanStop := strings.ReplaceAll(strings.TrimSpace(stopEntry.Text), ":", "-")

		ext := filepath.Ext(selectedFile)
		baseName := strings.TrimSuffix(selectedFile, ext)

		// Eindeutigen Dateinamen generieren (z.B. Video_cut_50_bis_2-20.mp4)
		outName := fmt.Sprintf("%s_cut_%s_bis_%s%s", baseName, cleanStart, cleanStop, ext)
		outputPath := filepath.Join(dirPath, outName)

		// Falls die Datei bereits existiert, laufende Nummer anhängen (_1, _2, ...)
		counter := 1
		for {
			if _, err := os.Stat(outputPath); os.IsNotExist(err) {
				break
			}
			outName = fmt.Sprintf("%s_cut_%s_bis_%s_(%d)%s", baseName, cleanStart, cleanStop, counter, ext)
			outputPath = filepath.Join(dirPath, outName)
			counter++
		}

		inputPath := filepath.Join(dirPath, selectedFile)

		cutWin.Close()

		go func() {
			err := videocut.ProcessCut(inputPath, outputPath, startEntry.Text, stopEntry.Text)
			if err != nil {
				dialog.ShowError(fmt.Errorf("Fehler beim Schneiden:\n%v", err), parentWin)
			} else {
				dialog.ShowInformation("Erfolg", fmt.Sprintf("Ausschnitt erfolgreich gespeichert als:\n%s", outName), parentWin)
			}
		}()
	})
	executeCutBtn.Importance = widget.HighImportance

	inputForm := container.NewVBox(
		widget.NewLabelWithStyle("1. Video in der Liste markieren:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle("2. Zeiten festlegen (Sekunden oder MM:SS):", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewGridWithColumns(2, startEntry, stopEntry),
		widget.NewSeparator(),
		executeCutBtn,
	)

	content := container.NewBorder(
		nil,
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
	confirmWin.Resize(fyne.NewSize(600, 450))

	checkMap := make(map[int]*widget.Check)
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

		if len(b.Files) >= 2 {
			chk.SetChecked(true)
		} else {
			chk.SetChecked(false)
		}

		checkMap[i] = chk
		checkListContainer.Add(chk)
	}

	allSelected := true
	selectAllBtn := widget.NewButton("Alle / Keine auswählen", func() {
		allSelected = !allSelected
		for _, chk := range checkMap {
			chk.SetChecked(allSelected)
		}
	})

	cancelBtn := widget.NewButton("Abbrechen", func() {
		confirmWin.Close()
	})

	okBtn := widget.NewButton("OK (Ausführen)", func() {
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
				listPath, err := camprocessing.CreateConcatList(dirPath, block)
				if err != nil {
					errors = append(errors, fmt.Sprintf("Tag %s: %v", block.DateStr, err))
					continue
				}

				ext := filepath.Ext(block.Files[0])
				cmdName, args := camprocessing.BuildFFmpegCmd(dirPath, listPath, block.DateStr, ext)
				cmd := exec.Command(cmdName, args...)

				output, err := cmd.CombinedOutput()
				_ = os.Remove(listPath)

				if err != nil {
					errors = append(errors, fmt.Sprintf("Tag %s Fehler: %v\nOutput: %s", block.DateStr, err, string(output)))
				} else {
					processedBlocks++
				}
			}

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
