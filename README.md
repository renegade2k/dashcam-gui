# 📹 Dashcam GUI

![Go](https://img.shields.io/badge/Go-1.20+-00ADD8?style=for-the-badge&logo=go&logoColor=white)
![Fyne](https://img.shields.io/badge/Fyne-GUI-29BEB0?style=for-the-badge)
![FFmpeg](https://img.shields.io/badge/FFmpeg-Lossless_Merge-007800?style=for-the-badge&logo=ffmpeg&logoColor=white)
![Platform](https://img.shields.io/badge/Platform-Windows_%7C_Linux-blue?style=for-the-badge)

Ein leichtgewichtiges, schnelles und plattformübergreibendes Tool zum automatischen Sortieren, Gruppieren und verlustfreien Zusammenfügen von Dashcam-Videoaufnahmen.

---

## ✨ Key Features

* **⚡ Blitzschnell & Verlustfrei:** Nutzt den FFmpeg Concat Demuxer (`-c copy`) – kein zeitintensives Neu-Codieren, kein Qualitätsverlust!
* **🔍 Automatische Kamera-Erkennung:** Prüft den Ordner automatisch auf verschiedene Kamera-Namensschemata.
* **📅 Tages-Gruppierung:** Fasst Aufnahmen desselben Tages chronologisch in logische Blöcke zusammen.
* **🎛️ Volle Kontrolle:** Übersichtlicher Bestätigungsdialog mit Checkbox-Auswahl für jeden Tagesblock.
* **🛡️ Smartes Handling:** Blöcke mit nur einer Datei werden erkannt und standardmäßig übersprungen.

---

## 🚀 Workflow & Bedienung

1. **Arbeitsordner wählen:** Wähle den Quellordner mit deinen Dashcam-Videos aus. Dieser dient gleichzeitig als Ausgabeordner für die kombinierten Dateien.
2. **Videos anzeigen *(Optional)*:** Zeigt eine reine Kontrollliste aller erkannten Videodateien im Ordner an.
3. **Kombinieren:**
   * Scanned den Ordner nach unterstützten Kamera-Mustern.
   * Gruppiert die Videos tagesweise.
   * Öffnet ein Dialogfenster, in dem du einzelne Tagesblöcke per Checkbox an- oder abwählen kannst.
4. **Ausführung:** Nach der Bestätigung werden die gewählten Clips chronologisch zu einer langen Videodatei (`Kombiniert_YYYY-MM-DD.mp4`) zusammengefügt.

---

## 📷 Unterstützte Kamera-Muster

Das Tool erkennt und verarbeitet automatisch folgende Dateinamensschemata:

| Kamera-Typ | Schema / Muster | Beispiel |
| :--- | :--- | :--- |
| **Cam A** | `YYYYMMDDHHMMSS_XXXXXX` | `20260327122939_000194.MP4` |
| **Cam B** | `FILEYYMMDD-HHMMSSF` | `FILE250923-042437F.MP4` |
| **Cam C** | `YYYY_MMDD_HHMMSS` | `2018_1121_080217.MOV` |

---

## ⚙️ Voraussetzung

* **FFmpeg** muss auf dem System installiert und über die Umgebungsvariablen (`PATH`) erreichbar sein.

---

<p align="center">
  <em>-= happy vibe-coding =-</em>
</p>
