use base64::engine::general_purpose::STANDARD;
use base64::Engine;
use serde::Serialize;

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

pub fn collect() -> Vec<CaptureSource> {
    let mut sources = Vec::new();

    if let Ok(monitors) = xcap::Monitor::all() {
        for (index, monitor) in monitors.iter().enumerate() {
            let title = monitor
                .name()
                .ok()
                .filter(|name| !name.is_empty())
                .unwrap_or_else(|| format!("Screen {}", index + 1));
            let id = match monitor.id() {
                Ok(id) => id,
                Err(_) => continue,
            };

            sources.push(CaptureSource {
                id: format!("screen:{id}"),
                kind: "screen",
                title,
                thumbnail: monitor.capture_image().ok().and_then(thumbnail_of),
                pid: None,
            });
        }
    }

    if let Ok(windows) = xcap::Window::all() {
        let mut seen: Vec<u32> = Vec::new();

        for window in windows.iter() {
            if window.is_minimized().unwrap_or(false) {
                continue;
            }
            if window.z().unwrap_or(1) != 0 {
                continue;
            }
            let title = window.title().unwrap_or_default();
            let app = window.app_name().unwrap_or_default();
            if title.is_empty() && app.is_empty() {
                continue;
            }
            let id = match window.id() {
                Ok(id) => id,
                Err(_) => continue,
            };

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
                thumbnail: window.capture_image().ok().and_then(thumbnail_of),
                pid,
            });
        }
    }

    sources
}
