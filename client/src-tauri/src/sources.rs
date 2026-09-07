use std::sync::mpsc;
use std::time::{Duration, Instant};

use base64::engine::general_purpose::STANDARD;
use base64::Engine;
use serde::Serialize;

const MIN_SIDE: u32 = 160;

const PICTURE_BUDGET: Duration = Duration::from_secs(5);

const SYSTEM_APPS: [&str; 5] = [
    "Window Server",
    "Control Center",
    "loginwindow",
    "Dock",
    "Notification Center",
];

#[derive(Serialize, Clone)]
pub struct CaptureSource {
    pub id: String,
    pub kind: &'static str,
    pub title: String,
    pub thumbnail: Option<String>,
    pub pid: Option<u32>,
}

#[derive(Clone, Copy, PartialEq, Eq)]
pub enum Target {
    Display(u32),
    Window(u32),
}

pub fn parse_target(id: &str) -> Option<Target> {
    let (kind, raw) = id.split_once(':')?;
    let number = raw.parse::<u32>().ok()?;
    match kind {
        "screen" => Some(Target::Display(number)),
        "window" => Some(Target::Window(number)),
        _ => None,
    }
}

fn thumbnail_of(image: xcap::image::RgbaImage) -> Option<String> {
    let small = xcap::image::DynamicImage::ImageRgba8(image).thumbnail(320, 200);
    let mut bytes = std::io::Cursor::new(Vec::new());
    small
        .write_to(&mut bytes, xcap::image::ImageFormat::Png)
        .ok()?;
    Some(format!(
        "data:image/png;base64,{}",
        STANDARD.encode(bytes.into_inner())
    ))
}

enum Handle {
    Screen(xcap::Monitor),
    App(xcap::Window),
}

enum Progress {
    Listed(Vec<CaptureSource>),
    Picture(usize, String),
}

pub fn collect() -> Vec<CaptureSource> {
    let (send, receive) = mpsc::channel::<Progress>();
    std::thread::spawn(move || look(&send));

    let mut sources: Vec<CaptureSource> = Vec::new();
    let deadline = Instant::now() + PICTURE_BUDGET;

    while let Ok(progress) =
        receive.recv_timeout(deadline.saturating_duration_since(Instant::now()))
    {
        match progress {
            Progress::Listed(listed) => sources = listed,
            Progress::Picture(at, picture) => {
                if let Some(source) = sources.get_mut(at) {
                    source.thumbnail = Some(picture);
                }
            }
        }
    }

    if sources.is_empty() {
        log::warn!("screen: nothing came back when asking what is on screen");
    } else if Instant::now() >= deadline {
        log::info!(
            "screen: {} sources, some without a picture — the look ran out of time",
            sources.len()
        );
    }

    sources
}

fn look(send: &mpsc::Sender<Progress>) {
    let mut sources = Vec::new();
    let mut handles = Vec::new();

    if let Ok(monitors) = xcap::Monitor::all() {
        for (index, monitor) in monitors.into_iter().enumerate() {
            let title = monitor
                .name()
                .ok()
                .filter(|name| !name.is_empty())
                .unwrap_or_else(|| format!("Screen {}", index + 1));
            let Ok(id) = monitor.id() else { continue };

            sources.push(CaptureSource {
                id: format!("screen:{id}"),
                kind: "screen",
                title,
                thumbnail: None,
                pid: None,
            });
            handles.push(Handle::Screen(monitor));
        }
    }

    if let Ok(windows) = xcap::Window::all() {
        let mut seen: Vec<u32> = Vec::new();

        for window in windows {
            if window.is_minimized().unwrap_or(false) {
                continue;
            }
            if window.width().unwrap_or(0) < MIN_SIDE || window.height().unwrap_or(0) < MIN_SIDE {
                continue;
            }
            let title = window.title().unwrap_or_default();
            let app = window.app_name().unwrap_or_default();
            if title.is_empty() && app.is_empty() {
                continue;
            }
            if SYSTEM_APPS.contains(&app.as_str()) {
                continue;
            }
            let Ok(id) = window.id() else { continue };

            let pid = window.pid().ok();
            if let Some(pid) = pid {
                if seen.contains(&pid) {
                    continue;
                }
                seen.push(pid);
            }

            sources.push(CaptureSource {
                id: format!("window:{id}"),
                kind: "app",
                title: if app.is_empty() { title } else { app },
                thumbnail: None,
                pid,
            });
            handles.push(Handle::App(window));
        }
    }

    log::info!(
        "screen: picker found {} screens and {} apps",
        sources
            .iter()
            .filter(|source| source.kind == "screen")
            .count(),
        sources.iter().filter(|source| source.kind == "app").count()
    );

    if send.send(Progress::Listed(sources)).is_err() {
        return;
    }

    for (at, handle) in handles.iter().enumerate() {
        let picture = match handle {
            Handle::Screen(monitor) => monitor.capture_image().ok().and_then(thumbnail_of),
            Handle::App(window) => window.capture_image().ok().and_then(thumbnail_of),
        };
        let Some(picture) = picture else { continue };
        if send.send(Progress::Picture(at, picture)).is_err() {
            return;
        }
    }
}
