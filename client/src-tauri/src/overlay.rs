use tauri::{AppHandle, LogicalPosition, LogicalSize, Manager, WebviewUrl, WebviewWindowBuilder};

pub const LABEL: &str = "overlay";

const WIDTH: f64 = 300.0;
const HEIGHT: f64 = 420.0;
const MARGIN: f64 = 24.0;

pub fn show(app: &AppHandle) -> Result<(), String> {
    if let Some(window) = app.get_webview_window(LABEL) {
        window.show().map_err(|error| error.to_string())?;
        return Ok(());
    }

    let window = WebviewWindowBuilder::new(
        app,
        LABEL,
        WebviewUrl::App("index.html".into()),
    )
    .title("Vocalis call")
    .inner_size(WIDTH, HEIGHT)
    .decorations(false)
    .transparent(true)
    .shadow(false)
    .always_on_top(true)
    .skip_taskbar(true)
    .focused(false)
    .resizable(false)
    .build()
    .map_err(|error| format!("overlay window: {error}"))?;

    window
        .set_ignore_cursor_events(true)
        .map_err(|error| error.to_string())?;

    place(&window);
    Ok(())
}

pub fn hide(app: &AppHandle) {
    if let Some(window) = app.get_webview_window(LABEL) {
        let _ = window.close();
    }
}

fn place(window: &tauri::WebviewWindow) {
    let Ok(Some(screen)) = window.current_monitor() else {
        return;
    };
    let scale = screen.scale_factor();
    let size = screen.size().to_logical::<f64>(scale);
    let origin = screen.position().to_logical::<f64>(scale);

    let _ = window.set_size(LogicalSize::new(WIDTH, HEIGHT));
    let _ = window.set_position(LogicalPosition::new(
        origin.x + size.width - WIDTH - MARGIN,
        origin.y + size.height - HEIGHT - MARGIN,
    ));
}
