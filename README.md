# 📹 Dashcam GUI

Ein leichtgewichtiges, schnelles und plattformübergreifendes Tool zum automatischen Sortieren, Gruppieren, verlustfreien Zusammenfügen und Schneiden von Dashcam-Videoaufnahmen.

[![Rust](https://img.shields.io/badge/Rust-1.80+-000000?style=for-the-badge&logo=rust&logoColor=white)](https://www.rust-lang.org/)
[![Slint](https://img.shields.io/badge/Slint-GUI-2379F4?style=for-the-badge)](https://slint.dev/)
[![FFmpeg](https://img.shields.io/badge/FFmpeg-Lossless_Merge-007800?style=for-the-badge&logo=ffmpeg&logoColor=white)](https://ffmpeg.org/)
[![Platform](https://img.shields.io/badge/Platform-Windows_|_Linux-blue?style=for-the-badge)](#)

---

## ✨ Key Features

- **⚡ Blitzschnell & Verlustfrei:** Nutzt den FFmpeg Concat Demuxer (`-c copy`) – kein zeitintensives Neu-Codieren, kein Qualitätsverlust!
- **🔍 Automatische Kamera-Erkennung:** Prüft den Ordner automatisch auf verschiedene Kamera-Namensschemata.
- **📅 Tages-Gruppierung:** Fasst Aufnahmen desselben Tages chronologisch in logische Blöcke zusammen.
- **🎛️ Volle Kontrolle:** Übersichtlicher Bestätigungsdialog mit Checkbox-Auswahl für jeden Tagesblock.
- **🛡️ Smartes Handling:** Blöcke mit nur einer Datei werden erkannt und standardmäßig übersprungen. Vorhandene Dateien werden nie überschrieben.
- **✂️ Video schneiden:** Ausschnitte verlustfrei exportieren (Start/Ende in Sekunden, `MM:SS` oder `HH:MM:SS`).
- **📦 Aufräumen (optional):** Verarbeitete Quelldateien lassen sich automatisch in den Ordner `verarbeitet` verschieben.

---

## 🚀 Workflow & Bedienung

1. **Arbeitsordner wählen:** Wähle den Quellordner mit deinen Dashcam-Videos aus. Dieser dient gleichzeitig als Ausgabeordner.
2. **Videos anzeigen *(Optional)*:** Zeigt eine reine Kontrollliste aller erkannten Videodateien im Ordner an.
3. **Kombinieren:**

   - Scannt den Ordner nach unterstützten Kamera-Mustern.
   - Gruppiert die Videos tagesweise.
   - Öffnet ein Dialogfenster, in dem du einzelne Tagesblöcke per Checkbox an- oder abwählen kannst.
   - Nach der Bestätigung werden die gewählten Clips chronologisch zu einer langen Videodatei (`Kombiniert_YYYY-MM-DD.mp4`) zusammengefügt.
4. **Video schneiden:** Video in der Liste markieren, Start- und Endzeit eingeben und exportieren. Die Ausgabe heißt `<Name>_cut_<Start>_bis_<Ende>.<Endung>`.

> **Hinweis zum Schneiden:** Der Schnitt erfolgt verlustfrei an Keyframes und ist daher nicht sekundengenau.

---

## 📷 Unterstützte Kamera-Muster

Das Tool erkennt und verarbeitet automatisch folgende Dateinamensschemata:

| Kamera-Typ | Schema / Muster         | Beispiel                    |
|:-----------|:------------------------|:----------------------------|
| **Cam A**  | `YYYYMMDDHHMMSS_XXXXXX` | `20260327122939_000194.MP4` |
| **Cam B**  | `FILEYYMMDD-HHMMSSF`    | `FILE250923-042437F.MP4`    |
| **Cam C**  | `YYYY_MMDD_HHMMSS`      | `2018_1121_080217.MOV`      |

Unterstützte Dateiendungen: `.mp4`, `.mov`, `.ts`, `.avi`

---

## ⚙️ Voraussetzung

- **FFmpeg** muss auf dem System installiert und über die Umgebungsvariablen (`PATH`) erreichbar sein.

---

## 🛠️ Aus dem Quellcode bauen

Benötigt wird [Rust](https://rustup.rs) (Version 1.80 oder neuer).

```bash
git clone https://github.com/renegade2k/dashcam-gui.git
cd dashcam-gui
cargo run --release      # starten
cargo build --release    # Binary liegt danach in target/release/
cargo test               # Tests ausführen
```

---

*-= happy vibe-coding =-*
