// Unter Windows im Release-Build kein Konsolenfenster öffnen
#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

mod camprocessing;
mod util;
mod videocut;

use std::path::{Path, PathBuf};
use std::rc::Rc;
use std::thread;
use std::time::Duration;

use slint::{ComponentHandle, Model, ModelRc, SharedString, VecModel};

use camprocessing::DayBlock;

slint::include_modules!();

fn main() -> Result<(), slint::PlatformError> {
    let ui = MainWindow::new()?;
    ui.set_ffmpeg_ok(camprocessing::ffmpeg_available());

    ui.on_choose_folder({
        let weak = ui.as_weak();
        move || {
            if let Some(path) = rfd::FileDialog::new().pick_folder() {
                if let Some(ui) = weak.upgrade() {
                    ui.set_work_dir(SharedString::from(path.to_string_lossy().as_ref()));
                }
            }
        }
    });

    ui.on_open_folder({
        let weak = ui.as_weak();
        move || {
            if let Some(ui) = weak.upgrade() {
                util::open_working_folder(&ui.get_work_dir());
            }
        }
    });

    ui.on_show_files({
        let weak = ui.as_weak();
        move || {
            if let Some(ui) = weak.upgrade() {
                if let Err(e) = open_file_list(&ui.get_work_dir()) {
                    util::error("Fehler", &e.to_string());
                }
            }
        }
    });

    ui.on_combine({
        let weak = ui.as_weak();
        move || {
            if let Some(ui) = weak.upgrade() {
                let dir = ui.get_work_dir().to_string();
                if let Err(e) = open_combine_window(dir, weak.clone()) {
                    util::error("Fehler", &e.to_string());
                }
            }
        }
    });

    ui.on_cut({
        let weak = ui.as_weak();
        move || {
            if let Some(ui) = weak.upgrade() {
                let dir = ui.get_work_dir().to_string();
                if let Err(e) = open_cut_window(dir, weak.clone()) {
                    util::error("Fehler", &e.to_string());
                }
            }
        }
    });

    // Schließt man das Hauptfenster, beendet sich die ganze App (wie bei Fyne)
    ui.window().on_close_requested(|| {
        let _ = slint::quit_event_loop();
        slint::CloseRequestResponse::HideWindow
    });

    ui.run()
}

// ------------------------------------------------------------------ Helfer

fn string_model(items: Vec<String>) -> ModelRc<SharedString> {
    ModelRc::new(VecModel::from(
        items.into_iter().map(SharedString::from).collect::<Vec<_>>(),
    ))
}

/// Blendet das Lade-Overlay ein, führt `work` in einem Thread aus und ruft danach
/// `done` im UI-Thread auf (nachdem das Overlay wieder ausgeblendet wurde).
fn run_background<T, W, D>(main: &slint::Weak<MainWindow>, text: &str, work: W, done: D)
where
    T: Send + 'static,
    W: FnOnce() -> T + Send + 'static,
    D: FnOnce(T) + Send + 'static,
{
    if let Some(ui) = main.upgrade() {
        ui.set_busy_text(text.into());
        ui.set_busy(true);
    }

    let main = main.clone();
    thread::spawn(move || {
        let result = work();
        let _ = main.upgrade_in_event_loop(move |ui| {
            ui.set_busy(false);
            // Kurze Verzögerung, damit das Overlay verschwindet, bevor der Dialog blockiert
            slint::Timer::single_shot(Duration::from_millis(50), move || done(result));
        });
    });
}

// -------------------------------------------------------------- Dateiliste

fn open_file_list(dir: &str) -> Result<(), slint::PlatformError> {
    let files = util::load_video_files(Path::new(dir));
    let base = Path::new(dir)
        .file_name()
        .map(|s| s.to_string_lossy().into_owned())
        .unwrap_or_else(|| dir.to_string());

    let win = FileListWindow::new()?;
    win.set_window_title(format!("Gefundene Videos - {base}").into());
    win.set_header(format!("{} Videodatei(en) gefunden in:\n{dir}", files.len()).into());
    win.set_files(string_model(files));
    win.show()
}

// ----------------------------------------------------------- Video schneiden

fn open_cut_window(dir: String, main: slint::Weak<MainWindow>) -> Result<(), slint::PlatformError> {
    let videos = util::load_video_files(Path::new(&dir));
    if videos.is_empty() {
        util::info("Hinweis", "Keine Videodateien im Ordner gefunden.");
        return Ok(());
    }

    let win = CutWindow::new()?;
    win.set_videos(string_model(videos.clone()));

    let weak_win = win.as_weak();
    win.on_export_cut(move |selected, start, stop| {
        let Some(win) = weak_win.upgrade() else { return };

        if selected < 0 || selected as usize >= videos.len() {
            util::info("Hinweis", "Bitte wähle ein Video aus der Liste aus.");
            return;
        }

        let start = start.trim().to_string();
        let stop = stop.trim().to_string();
        if start.is_empty() || stop.is_empty() {
            util::info("Hinweis", "Bitte Start- und Endzeit angeben.");
            return;
        }

        // Eingaben prüfen, solange das Fenster noch offen ist
        if let Err(e) = videocut::validate_range(&start, &stop) {
            util::error("Fehler", &e);
            return;
        }

        // Nach der Prüfung enthalten die Eingaben nur noch Ziffern, ':', '.' und ','
        let name_safe = |s: &str| s.replace(':', "-").replace(',', ".");

        let file = videos[selected as usize].clone();
        let (base, ext) = util::split_ext(&file);
        let stem = format!("{base}_cut_{}_bis_{}", name_safe(&start), name_safe(&stop));
        let out_name = util::unique_file_name(Path::new(&dir), &stem, ext);

        let input = Path::new(&dir).join(&file);
        let output = Path::new(&dir).join(&out_name);

        let _ = win.hide();

        run_background(
            &main,
            "Video wird geschnitten...",
            move || videocut::process_cut(&input, &output, &start, &stop),
            move |res| match res {
                Ok(()) => util::info(
                    "Erfolg",
                    &format!("Ausschnitt erfolgreich gespeichert als:\n{out_name}"),
                ),
                Err(e) => util::error("Fehler", &format!("Fehler beim Schneiden:\n{e}")),
            },
        );
    });

    win.show()
}

// ---------------------------------------------------------------- Kombinieren

struct CombineReport {
    outputs: Vec<String>,
    failures: Vec<String>,
    moved_all: bool,
}

fn open_combine_window(dir: String, main: slint::Weak<MainWindow>) -> Result<(), slint::PlatformError> {
    let result = match camprocessing::analyze_and_group(Path::new(&dir)) {
        Ok(r) => r,
        Err(e) => {
            util::error("Fehler", &e);
            return Ok(());
        }
    };

    let items: Vec<BlockItem> = result
        .blocks
        .iter()
        .enumerate()
        .map(|(i, b)| {
            let d = &b.date_str;
            let date = format!("{}.{}.{}", &d[6..8], &d[4..6], &d[0..4]);
            let label = if b.files.len() == 1 {
                format!("Block {} ({date}): 1 Datei (Einzeldatei)", i + 1)
            } else {
                format!("Block {} ({date}): {} Dateien", i + 1, b.files.len())
            };
            BlockItem {
                label: label.into(),
                // Blöcke mit nur einer Datei sind standardmäßig abgewählt
                checked: b.files.len() >= 2,
            }
        })
        .collect();
    let model = Rc::new(VecModel::from(items));

    let mut header = format!("Erkannter Kamera-Typ: {}\nArbeitsordner: {dir}", result.cam_type);
    if result.ignored > 0 {
        header += &format!(
            "\nHinweis: {} Datei(en) anderer Kamera-Schemata werden ignoriert.",
            result.ignored
        );
    }
    header += "\n\nWähle die Tagesblöcke aus, die zusammengefügt werden sollen:";

    let win = CombineWindow::new()?;
    win.set_header(header.into());
    win.set_blocks(model.clone().into());

    win.on_toggle({
        let model = model.clone();
        move |i| {
            let i = i as usize;
            if let Some(mut item) = model.row_data(i) {
                item.checked = !item.checked;
                model.set_row_data(i, item);
            }
        }
    });

    // Sind alle ausgewählt, werden alle abgewählt – sonst werden alle ausgewählt
    win.on_select_all({
        let model = model.clone();
        move || {
            let all_checked = model.iter().all(|b| b.checked);
            for i in 0..model.row_count() {
                if let Some(mut item) = model.row_data(i) {
                    item.checked = !all_checked;
                    model.set_row_data(i, item);
                }
            }
        }
    });

    win.on_cancel({
        let weak_win = win.as_weak();
        move || {
            if let Some(w) = weak_win.upgrade() {
                let _ = w.hide();
            }
        }
    });

    let all_blocks: Rc<Vec<DayBlock>> = Rc::new(result.blocks);
    win.on_confirm({
        let weak_win = win.as_weak();
        let model = model.clone();
        move || {
            let Some(win) = weak_win.upgrade() else { return };

            let selected: Vec<DayBlock> = all_blocks
                .iter()
                .zip(model.iter())
                .filter(|(_, item)| item.checked)
                .map(|(block, _)| block.clone())
                .collect();

            if selected.is_empty() {
                util::info("Hinweis", "Bitte wähle mindestens einen Tagesblock zum Kombinieren aus.");
                return;
            }

            let should_move = win.get_move_files();
            let _ = win.hide();

            let work_dir = PathBuf::from(&dir);
            run_background(
                &main,
                "Videos werden zusammengefügt...",
                move || combine_blocks(&work_dir, &selected, should_move),
                move |report| show_combine_report(report, should_move),
            );
        }
    });

    win.show()
}

/// Läuft im Hintergrund-Thread: fügt jeden gewählten Tagesblock per FFmpeg zusammen.
fn combine_blocks(dir: &Path, blocks: &[DayBlock], should_move: bool) -> CombineReport {
    let mut outputs = Vec::new();
    let mut failures = Vec::new();
    let mut moved_all = true;

    for block in blocks {
        let day = &block.date_str;

        let list_path = match camprocessing::create_concat_list(dir, block) {
            Ok(p) => p,
            Err(e) => {
                failures.push(format!("Tag {day}: {e}"));
                continue;
            }
        };

        let (_, ext) = util::split_ext(&block.files[0]);
        let (args, out_name) = camprocessing::build_ffmpeg_args(dir, &list_path, day, ext);

        let run = util::ffmpeg_command().args(&args).output();
        let _ = std::fs::remove_file(&list_path);

        let output = match run {
            Ok(o) => o,
            Err(e) => {
                failures.push(format!("Tag {day}: {e}"));
                continue;
            }
        };
        let text = util::combined_output(&output);

        if !output.status.success() {
            failures.push(format!(
                "Tag {day}: {}\nFFmpeg-Ausgabe: {}",
                output.status,
                util::tail(&text, 600)
            ));
            continue;
        }

        // Sicherheitsnetz: Ausgabedatei muss wirklich existieren
        match std::fs::metadata(dir.join(&out_name)) {
            Ok(m) if m.len() > 0 => {}
            _ => {
                failures.push(format!(
                    "Tag {day}: FFmpeg hat keine Ausgabedatei erzeugt\nFFmpeg-Ausgabe: {}",
                    util::tail(&text, 600)
                ));
                continue;
            }
        }

        outputs.push(out_name);

        // Wenn gewünscht, Quelldateien nach "verarbeitet" verschieben
        if should_move {
            if let Err(e) = util::move_files_to_subfolder(dir, &block.files, "verarbeitet") {
                moved_all = false;
                failures.push(format!(
                    "Tag {day}: Quelldateien konnten nicht (vollständig) verschoben werden:\n{e}"
                ));
            }
        }
    }

    CombineReport { outputs, failures, moved_all }
}

fn show_combine_report(r: CombineReport, should_move: bool) {
    let created = if r.outputs.is_empty() {
        String::new()
    } else {
        format!("Erstellt ({}):\n{}", r.outputs.len(), r.outputs.join("\n"))
    };

    if !r.failures.is_empty() {
        let mut msg = format!("Fehler bei der Ausführung:\n{}", r.failures.join("\n"));
        if !created.is_empty() {
            msg.push_str("\n\n");
            msg.push_str(&created);
        }
        util::error("Fehler", &msg);
        return;
    }

    let mut msg = format!(
        "Erfolgreich {} ausgewählte(n) Tagesblock/Blöcke zusammengefügt!\n\n{created}",
        r.outputs.len()
    );
    if should_move && r.moved_all {
        msg.push_str("\n\nDie Quelldateien wurden in den Ordner 'verarbeitet' verschoben.");
    }
    util::info("Erfolg", &msg);
}
