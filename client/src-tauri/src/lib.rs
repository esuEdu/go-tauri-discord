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

pub fn run() {
    tauri::Builder::default()
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
