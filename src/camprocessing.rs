//! Kamera-Erkennung, Tages-Gruppierung und Concat-Befehl (Port von camprocessing/cama.go).

use std::collections::BTreeMap;
use std::ffi::OsString;
use std::fs;
use std::path::{Path, PathBuf};
use std::process::Stdio;
use std::sync::LazyLock;

use regex::Regex;

use crate::util;

// Hinweis: bewusst [0-9] statt \d (in Rust-Regex matcht \d auch Unicode-Ziffern).
/// Pattern A: YYYYMMDDHHMMSS_XXXXXX (z.B. 20260327122939_000194.MP4)
static PATTERN_CAM_A: LazyLock<Regex> =
    LazyLock::new(|| Regex::new(r"(?i)^([0-9]{8})[0-9]{6}_[0-9]+\.(mp4|mov|ts|avi)$").unwrap());
/// Pattern B: FILEYYMMDD-HHMMSSF (z.B. FILE250923-042437F.MP4)
static PATTERN_CAM_B: LazyLock<Regex> = LazyLock::new(|| {
    Regex::new(r"(?i)^FILE([0-9]{2})([0-9]{2})([0-9]{2})-[0-9]{6}F\.(mp4|mov|ts|avi)$").unwrap()
});
/// Pattern C: YYYY_MMDD_HHMMSS (z.B. 2018_1121_080217.MOV)
static PATTERN_CAM_C: LazyLock<Regex> =
    LazyLock::new(|| Regex::new(r"(?i)^([0-9]{4})_([0-9]{4})_[0-9]{6}\.(mp4|mov|ts|avi)$").unwrap());

#[derive(Clone, Copy, PartialEq, Eq, Debug)]
pub enum Cam {
    A = 0,
    B = 1,
    C = 2,
}

#[derive(Clone, Debug)]
pub struct DayBlock {
    /// Standardisiertes Format: YYYYMMDD
    pub date_str: String,
    /// Aufsteigend sortierte Dateinamen
    pub files: Vec<String>,
}

pub struct ProcessingResult {
    pub cam_type: String,
    pub blocks: Vec<DayBlock>,
    /// Dateien anderer Kamera-Schemata, die nicht berücksichtigt werden
    pub ignored: usize,
}

/// Erkennt Kamera-Typ und Datum (YYYYMMDD) anhand des Dateinamens.
pub fn match_name(filename: &str) -> Option<(Cam, String)> {
    if let Some(c) = PATTERN_CAM_A.captures(filename) {
        return Some((Cam::A, c[1].to_string()));
    }
    if let Some(c) = PATTERN_CAM_B.captures(filename) {
        return Some((Cam::B, format!("20{}{}{}", &c[1], &c[2], &c[3])));
    }
    if let Some(c) = PATTERN_CAM_C.captures(filename) {
        return Some((Cam::C, format!("{}{}", &c[1], &c[2])));
    }
    None
}

pub fn analyze_and_group(dir: &Path) -> Result<ProcessingResult, String> {
    let entries = fs::read_dir(dir).map_err(|e| format!("Fehler beim Lesen des Ordners: {e}"))?;

    // Zwischenspeicher pro Kameraschema (BTreeMap = Tage automatisch sortiert)
    let mut maps: [BTreeMap<String, Vec<String>>; 3] = Default::default();
    let mut counts = [0usize; 3];

    for entry in entries.flatten() {
        if entry.file_type().map(|t| t.is_dir()).unwrap_or(false) {
            continue;
        }
        // Dateinamen, die kein gültiges UTF-8 sind, passen ohnehin auf kein Muster
        let Ok(name) = entry.file_name().into_string() else {
            continue;
        };
        if let Some((cam, date)) = match_name(&name) {
            maps[cam as usize].entry(date).or_default().push(name);
            counts[cam as usize] += 1;
        }
    }

    // Gewinner-Typ anhand der meisten Treffer (bei Gleichstand: A vor B vor C)
    let (a, b, c) = (counts[0], counts[1], counts[2]);
    let (idx, label) = if a > 0 && a >= b && a >= c {
        (0, "Cam A (Standard Dashcam)")
    } else if b > 0 && b >= a && b >= c {
        (1, "Cam B (FILE-Schema mit F-Suffix)")
    } else if c > 0 && c >= a && c >= b {
        (2, "Cam C (YYYY_MMDD-Schema)")
    } else {
        return Err("keine Dateien mit den bekannten Kamera-Mustern (A, B oder C) gefunden".into());
    };

    let winner_count = counts[idx];
    let active = std::mem::take(&mut maps[idx]);

    // Tages-Blöcke aufbauen, Dateinamen chronologisch sortieren
    let blocks = active
        .into_iter()
        .map(|(date_str, mut files)| {
            files.sort();
            DayBlock { date_str, files }
        })
        .collect();

    Ok(ProcessingResult {
        cam_type: label.to_string(),
        blocks,
        ignored: a + b + c - winner_count,
    })
}

/// Erzeugt die temporäre Textdatei für FFmpeg für einen einzelnen Tagesblock.
pub fn create_concat_list(dir: &Path, block: &DayBlock) -> Result<PathBuf, String> {
    let list_path = dir.join(format!("concat_list_{}.txt", block.date_str));

    let mut content = String::new();
    for name in &block.files {
        // Einfache Anführungszeichen für FFmpegs concat-Syntax maskieren: ' -> '\''
        let escaped = name.replace('\'', r"'\''");
        content.push_str(&format!("file '{escaped}'\n"));
    }

    fs::write(&list_path, content).map_err(|e| format!("Konnte Concat-Liste nicht erstellen: {e}"))?;
    Ok(list_path)
}

/// Liefert die ffmpeg-Argumente und den Namen der Ausgabedatei.
/// `extension` mit Punkt, z.B. ".MP4". Existiert die Ausgabedatei schon, wird ein
/// Zähler angehängt (Kombiniert_2026-03-27_(1).MP4).
pub fn build_ffmpeg_args(dir: &Path, list_path: &Path, date_str: &str, extension: &str) -> (Vec<OsString>, String) {
    // Datum von YYYYMMDD in YYYY-MM-DD umwandeln
    let formatted = if date_str.len() == 8 {
        format!("{}-{}-{}", &date_str[0..4], &date_str[4..6], &date_str[6..8])
    } else {
        date_str.to_string()
    };

    let out_name = util::unique_file_name(dir, &format!("Kombiniert_{formatted}"), extension);

    let mut args: Vec<OsString> = ["-hide_banner", "-loglevel", "error", "-nostdin", "-f", "concat", "-safe", "0", "-i"]
        .iter()
        .map(OsString::from)
        .collect();
    args.push(list_path.into());
    args.extend(["-c", "copy", "-n"].iter().map(OsString::from)); // -n: nie überschreiben
    args.push(dir.join(&out_name).into());

    (args, out_name)
}

/// Prüft, ob FFmpeg aufrufbar ist.
pub fn ffmpeg_available() -> bool {
    util::ffmpeg_command()
        .arg("-version")
        .stdout(Stdio::null())
        .stderr(Stdio::null())
        .status()
        .map(|s| s.success())
        .unwrap_or(false)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn cam_a() {
        assert_eq!(
            match_name("20260327122939_000194.MP4"),
            Some((Cam::A, "20260327".to_string()))
        );
    }

    #[test]
    fn cam_b() {
        assert_eq!(
            match_name("FILE250923-042437F.MP4"),
            Some((Cam::B, "20250923".to_string()))
        );
        assert_eq!(
            match_name("file250923-042437f.mp4"),
            Some((Cam::B, "20250923".to_string()))
        );
    }

    #[test]
    fn cam_c() {
        assert_eq!(match_name("2018_1121_080217.MOV"), Some((Cam::C, "20181121".to_string())));
    }

    #[test]
    fn kein_treffer() {
        assert_eq!(match_name("urlaub.mp4"), None);
        assert_eq!(match_name("20260327122939_000194.txt"), None);
        assert_eq!(match_name("٢٠٢٦٠٣٢٧١٢٢٩٣٩_000194.MP4"), None); // Unicode-Ziffern
    }
}
