//! Verlustfreier Schnitt via FFmpeg (Port von videocut/cut.go).

use std::fs;
use std::io;
use std::path::Path;
use std::sync::LazyLock;

use regex::Regex;

use crate::util;

// Hinweis: bewusst [0-9] statt \d, denn \d matcht in Rust-Regex auch Unicode-Ziffern.
/// Alle Teile außer dem letzten: nur ganze Zahlen (Stunden / Minuten)
static INT_PART: LazyLock<Regex> = LazyLock::new(|| Regex::new(r"^[0-9]{1,6}$").unwrap());
/// Letzter Teil: Sekunden, optional mit Nachkommastellen
static SEC_PART: LazyLock<Regex> = LazyLock::new(|| Regex::new(r"^[0-9]{1,6}(\.[0-9]{1,3})?$").unwrap());

/// Wandelt "50", "50.5", "50,5", "1:20" oder "1:02:03" in Sekunden um.
/// Alles andere wird abgelehnt, damit nichts Unerwartetes an FFmpeg gelangt.
pub fn parse_time(input: &str) -> Result<f64, String> {
    let s = input.replace(',', ".");
    let s = s.trim();
    if s.is_empty() {
        return Err("leere Zeitangabe".into());
    }

    let usage = || format!("ungültige Zeitangabe {s:?} (erlaubt: Sekunden, MM:SS oder HH:MM:SS)");

    let parts: Vec<&str> = s.split(':').collect();
    if parts.len() > 3 {
        return Err(usage());
    }

    let mut total = 0.0_f64;
    for (i, p) in parts.iter().enumerate() {
        let is_last = i == parts.len() - 1;
        let re = if is_last { &*SEC_PART } else { &*INT_PART };
        if !re.is_match(p) {
            return Err(usage());
        }

        let v: f64 = p.parse().map_err(|_| format!("ungültige Zeitangabe {s:?}"))?;

        // Bei Angaben mit Doppelpunkt müssen Minuten und Sekunden unter 60 liegen
        if parts.len() > 1 && i > 0 && v >= 60.0 {
            return Err(format!(
                "ungültige Zeitangabe {s:?} (Minuten/Sekunden müssen unter 60 liegen)"
            ));
        }

        total = total * 60.0 + v;
    }
    Ok(total)
}

/// Prüft Start und Ende und liefert (Startzeit, Dauer) in Sekunden.
pub fn validate_range(start_time: &str, stop_time: &str) -> Result<(f64, f64), String> {
    let start = parse_time(start_time).map_err(|e| format!("Startzeit: {e}"))?;
    let stop = parse_time(stop_time).map_err(|e| format!("Endzeit: {e}"))?;
    if stop <= start {
        return Err("Die Endzeit muss nach der Startzeit liegen".into());
    }
    Ok((start, stop - start))
}

/// Führt den verlustfreien Schnitt via FFmpeg durch.
/// Mit "-c copy" wird nur an Keyframes geschnitten, Start und Ende sind
/// daher nicht auf die Sekunde genau.
pub fn process_cut(input: &Path, output: &Path, start_time: &str, stop_time: &str) -> Result<(), String> {
    let (start, duration) = validate_range(start_time, stop_time)?;

    let result = util::ffmpeg_command()
        .args(["-hide_banner", "-loglevel", "error", "-nostdin"])
        .arg("-n") // niemals eine vorhandene Datei überschreiben
        .arg("-ss")
        .arg(format!("{start:.3}"))
        .arg("-i")
        .arg(input)
        .arg("-t")
        .arg(format!("{duration:.3}"))
        .args(["-c", "copy"])
        .arg(output)
        .output();

    let out = match result {
        Ok(o) => o,
        Err(e) if e.kind() == io::ErrorKind::NotFound => {
            return Err("FFmpeg wurde nicht gefunden. Bitte installieren und im PATH verfügbar machen".into());
        }
        Err(e) => return Err(e.to_string()),
    };

    let text = util::combined_output(&out);
    if !out.status.success() {
        return Err(format!("{}\nFFmpeg-Ausgabe: {}", out.status, text.trim()));
    }

    // Sicherheitsnetz: FFmpeg kann sich mit Exit-Code 0 beenden, ohne etwas zu
    // schreiben. Daher prüfen, ob die Ausgabedatei wirklich existiert.
    match fs::metadata(output) {
        Ok(m) if m.len() > 0 => Ok(()),
        _ => Err(format!(
            "FFmpeg hat keine Ausgabedatei erzeugt\nFFmpeg-Ausgabe: {}",
            text.trim()
        )),
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn gueltige_zeiten() {
        assert_eq!(parse_time("50").unwrap(), 50.0);
        assert_eq!(parse_time("50,5").unwrap(), 50.5);
        assert_eq!(parse_time("1:20").unwrap(), 80.0);
        assert_eq!(parse_time("1:02:03").unwrap(), 3723.0);
        assert_eq!(parse_time(" 2:45 ").unwrap(), 165.0);
    }

    #[test]
    fn ungueltige_zeiten() {
        assert!(parse_time("").is_err());
        assert!(parse_time("abc").is_err());
        assert!(parse_time("1:2:3:4").is_err());
        assert!(parse_time("1:75").is_err());
        assert!(parse_time("1.5:20").is_err()); // Nachkommastellen nur im letzten Teil
        assert!(parse_time("-5").is_err());
        assert!(parse_time("1.2345").is_err());
        assert!(parse_time("١٢").is_err()); // Unicode-Ziffern müssen abgelehnt werden
    }

    #[test]
    fn bereich() {
        assert_eq!(validate_range("10", "70").unwrap(), (10.0, 60.0));
        assert!(validate_range("70", "10").is_err());
        assert!(validate_range("10", "10").is_err());
    }
}
