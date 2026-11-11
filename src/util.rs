//! Gemeinsame Hilfsfunktionen (Dateisystem, Prozesse, Dialoge).

use std::fs;
use std::path::Path;
use std::process::{Command, Output};

/// ffmpeg-Prozess; unter Windows ohne aufblitzendes Konsolenfenster.
#[allow(unused_mut)]
pub fn ffmpeg_command() -> Command {
    let mut cmd = Command::new("ffmpeg");
    #[cfg(windows)]
    {
        use std::os::windows::process::CommandExt;
        cmd.creation_flags(0x0800_0000); // CREATE_NO_WINDOW
    }
    cmd
}

/// stderr + stdout eines Prozesses als Text (ffmpeg schreibt Fehler nach stderr).
pub fn combined_output(out: &Output) -> String {
    format!(
        "{}{}",
        String::from_utf8_lossy(&out.stderr),
        String::from_utf8_lossy(&out.stdout)
    )
}

/// Wie Go's filepath.Ext: trennt am letzten Punkt, die Endung enthält den Punkt.
/// "a.MP4" -> ("a", ".MP4"), "abc" -> ("abc", "")
pub fn split_ext(name: &str) -> (&str, &str) {
    match name.rfind('.') {
        Some(i) => (&name[..i], &name[i..]),
        None => (name, ""),
    }
}

/// Liefert einen Dateinamen, der im Ordner noch nicht existiert:
/// stem+ext, danach stem_(1)+ext, stem_(2)+ext, ...
pub fn unique_file_name(dir: &Path, stem: &str, ext: &str) -> String {
    let mut name = format!("{stem}{ext}");
    let mut i = 1;
    while dir.join(&name).exists() {
        name = format!("{stem}_({i}){ext}");
        i += 1;
    }
    name
}

/// Kürzt lange Texte (z.B. FFmpeg-Ausgaben) auf die letzten n Zeichen.
pub fn tail(s: &str, n: usize) -> String {
    let t = s.trim();
    let chars: Vec<char> = t.chars().collect();
    if chars.len() <= n {
        t.to_string()
    } else {
        format!("…{}", chars[chars.len() - n..].iter().collect::<String>())
    }
}

/// Alle Videodateien (mp4, mov, ts, avi) im Ordner, alphabetisch sortiert.
pub fn load_video_files(dir: &Path) -> Vec<String> {
    let Ok(entries) = fs::read_dir(dir) else {
        return Vec::new();
    };
    let mut files: Vec<String> = entries
        .flatten()
        .filter(|e| !e.file_type().map(|t| t.is_dir()).unwrap_or(false))
        .filter_map(|e| e.file_name().into_string().ok())
        .filter(|name| {
            let (_, ext) = split_ext(name);
            matches!(ext.to_lowercase().as_str(), ".mp4" | ".mov" | ".ts" | ".avi")
        })
        .collect();
    files.sort();
    files
}

/// Verschiebt Dateien in einen Unterordner, ohne vorhandene zu überschreiben.
/// Alle aufgetretenen Fehler werden gesammelt zurückgegeben.
pub fn move_files_to_subfolder(dir: &Path, filenames: &[String], subfolder: &str) -> Result<(), String> {
    let target_dir = dir.join(subfolder);
    fs::create_dir_all(&target_dir).map_err(|e| e.to_string())?;

    let mut errs: Vec<String> = Vec::new();
    for fname in filenames {
        let old_path = dir.join(fname);
        let new_path = target_dir.join(fname);

        if new_path.exists() {
            errs.push(format!("{fname}: existiert im Ordner '{subfolder}' bereits"));
            continue;
        }
        if let Err(e) = fs::rename(&old_path, &new_path) {
            errs.push(format!("{fname}: {e}"));
        }
    }

    if errs.is_empty() {
        Ok(())
    } else {
        Err(errs.join("\n"))
    }
}

/// Öffnet den System-Dateimanager plattformunabhängig.
pub fn open_working_folder(path: &str) {
    if path.is_empty() {
        return;
    }
    let program = if cfg!(target_os = "windows") {
        "explorer"
    } else if cfg!(target_os = "linux") {
        "xdg-open"
    } else {
        "open"
    };

    if let Ok(mut child) = Command::new(program).arg(path).spawn() {
        // Im Hintergrund einsammeln, damit keine Zombie-Prozesse entstehen
        std::thread::spawn(move || {
            let _ = child.wait();
        });
    }
}

// ----------------------------------------------------------------- Dialoge

pub fn info(title: &str, message: &str) {
    let _ = rfd::MessageDialog::new()
        .set_level(rfd::MessageLevel::Info)
        .set_title(title)
        .set_description(message)
        .set_buttons(rfd::MessageButtons::Ok)
        .show();
}

pub fn error(title: &str, message: &str) {
    let _ = rfd::MessageDialog::new()
        .set_level(rfd::MessageLevel::Error)
        .set_title(title)
        .set_description(message)
        .set_buttons(rfd::MessageButtons::Ok)
        .show();
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn split_ext_wie_go() {
        assert_eq!(split_ext("a.MP4"), ("a", ".MP4"));
        assert_eq!(split_ext("a.b.mov"), ("a.b", ".mov"));
        assert_eq!(split_ext("abc"), ("abc", ""));
    }

    #[test]
    fn tail_kuerzt() {
        assert_eq!(tail("  abc  ", 10), "abc");
        assert_eq!(tail("abcdef", 3), "…def");
    }
}
