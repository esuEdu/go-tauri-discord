use base64::engine::general_purpose::STANDARD;
use base64::Engine;
use serde::Serialize;

#[cfg(target_os = "macos")]
fn enable_media_capture(window: &tauri::WebviewWindow) {
    use objc2::msg_send;
    use objc2::runtime::AnyObject;
    use objc2_foundation::{NSNumber, NSString};

    let _ = window.with_webview(|platform| unsafe {
        let webview = platform.inner() as *mut AnyObject;
        if webview.is_null() {
            return;
        }
        let configuration: *mut AnyObject = msg_send![webview, configuration];
        let preferences: *mut AnyObject = msg_send![configuration, preferences];

        for key in ["mediaDevicesEnabled", "screenCaptureEnabled"] {
            let value = NSNumber::new_bool(true);
            let name = NSString::from_str(key);
            let _: () = msg_send![preferences, setValue: &*value, forKey: &*name];
        }
    });
}

#[derive(Serialize)]
pub struct CaptureSource {
    id: String,
    kind: &'static str,
    title: String,
    thumbnail: Option<String>,
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

#[tauri::command]
async fn capture_sources() -> Vec<CaptureSource> {
    tauri::async_runtime::spawn_blocking(collect_sources)
        .await
        .unwrap_or_default()
}

fn collect_sources() -> Vec<CaptureSource> {
    let mut sources = Vec::new();

    if let Ok(monitors) = xcap::Monitor::all() {
        for (index, monitor) in monitors.iter().enumerate() {
            let title = monitor
                .name()
                .ok()
                .filter(|name| !name.is_empty())
                .unwrap_or_else(|| format!("Screen {}", index + 1));
            sources.push(CaptureSource {
                id: format!("screen:{}", monitor.id().unwrap_or(index as u32)),
                kind: "screen",
                title,
                thumbnail: monitor.capture_image().ok().and_then(thumbnail_of),
            });
        }
    }

    if let Ok(windows) = xcap::Window::all() {
        for window in windows.iter() {
            if window.is_minimized().unwrap_or(false) {
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
            sources.push(CaptureSource {
                id: format!("window:{id}"),
                kind: "app",
                title: if title.is_empty() { app } else { title },
                thumbnail: None,
            });
        }
    }

    sources
}

pub fn run() {
    tauri::Builder::default()
        .invoke_handler(tauri::generate_handler![capture_sources])
        .setup(|app| {
            #[cfg(target_os = "macos")]
            {
                use tauri::Manager;
                if let Some(window) = app.get_webview_window("main") {
                    enable_media_capture(&window);
                }
            }
            let _ = app;
            Ok(())
        })
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
