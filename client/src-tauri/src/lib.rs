pub mod capture;
pub mod encode;
pub mod publish;
pub mod sources;

use std::sync::Arc;

use serde::Deserialize;
use tauri::{AppHandle, Emitter, Manager, State};
use tokio::sync::mpsc::channel;
use tokio::sync::Mutex;

use capture::{Options, Quality, Sink};
use publish::{IceServer, Publisher};
use sources::CaptureSource;

const OFFER: &str = "screen://offer";
const ENDED: &str = "screen://ended";

const FRAME_QUEUE: usize = 120;

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

#[derive(Deserialize)]
struct IceServerInput {
    urls: Vec<String>,
    username: Option<String>,
    credential: Option<String>,
}

struct Active {
    capture: capture::Session,
    publisher: Publisher,
}

#[derive(Default)]
struct Screen {
    active: Mutex<Option<Active>>,
    #[cfg(target_os = "windows")]
    webview: std::sync::atomic::AtomicU32,
}

#[cfg(target_os = "windows")]
fn note_webview_process(window: &tauri::WebviewWindow, screen: Arc<Screen>) {
    use std::sync::atomic::Ordering;

    let _ = window.with_webview(move |platform| unsafe {
        let mut pid = 0u32;
        let Ok(core) = platform.controller().CoreWebView2() else {
            log::warn!("screen: no webview to take a process id from");
            return;
        };
        if core.BrowserProcessId(&mut pid).is_ok() && pid != 0 {
            log::info!("screen: our own sound plays from process {pid}");
            screen.webview.store(pid, Ordering::Relaxed);
        }
    });
}

#[tauri::command]
async fn capture_sources() -> Vec<CaptureSource> {
    tauri::async_runtime::spawn_blocking(sources::collect)
        .await
        .unwrap_or_default()
}

#[tauri::command]
async fn start_screen_share(
    app: AppHandle,
    screen: State<'_, Arc<Screen>>,
    source_id: String,
    quality: Quality,
    audio: bool,
    ice_servers: Vec<IceServerInput>,
) -> Result<(), String> {
    let target = sources::parse_target(&source_id).ok_or("that is not something we can share")?;

    log::info!(
        "screen: sharing {source_id} at {}x{} {} fps, sound {}",
        quality.width,
        quality.height,
        quality.frame_rate,
        if audio { "on" } else { "off" }
    );

    stop(&screen).await;

    let (video_tx, video_rx) = channel(FRAME_QUEUE);
    let (audio_tx, audio_rx) = channel(FRAME_QUEUE);

    let session = capture::start(
        Options {
            target,
            quality,
            audio,
            #[cfg(target_os = "windows")]
            webview: match screen.webview.load(std::sync::atomic::Ordering::Relaxed) {
                0 => None,
                pid => Some(pid),
            },
        },
        Sink {
            video: video_tx,
            audio: audio_tx,
        },
    )?;

    let servers = ice_servers
        .into_iter()
        .map(|server| IceServer {
            urls: server.urls,
            username: server.username,
            credential: server.credential,
        })
        .collect();

    let started = match publish::start(servers, video_rx, audio.then_some(audio_rx)).await {
        Ok(started) => started,
        Err(reason) => {
            session.stop();
            return Err(reason);
        }
    };

    app.emit(OFFER, started.offer)
        .map_err(|error| error.to_string())?;

    *screen.active.lock().await = Some(Active {
        capture: session,
        publisher: started.publisher,
    });

    let watcher = app.clone();
    let held = Arc::clone(&screen);
    tauri::async_runtime::spawn(async move {
        let _ = started.ended.await;
        stop(&held).await;
        let _ = watcher.emit(ENDED, ());
    });

    Ok(())
}

#[tauri::command]
async fn screen_answer(screen: State<'_, Arc<Screen>>, sdp: String) -> Result<(), String> {
    let guard = screen.active.lock().await;
    let Some(active) = guard.as_ref() else {
        return Ok(());
    };
    active.publisher.set_answer(sdp).await
}

#[tauri::command]
async fn screen_candidate(
    screen: State<'_, Arc<Screen>>,
    candidate: String,
    sdp_mid: Option<String>,
    sdp_mline_index: Option<u16>,
) -> Result<(), String> {
    let guard = screen.active.lock().await;
    let Some(active) = guard.as_ref() else {
        return Ok(());
    };
    active
        .publisher
        .add_candidate(candidate, sdp_mid, sdp_mline_index)
        .await
}

#[tauri::command]
async fn stop_screen_share(screen: State<'_, Arc<Screen>>) -> Result<(), String> {
    stop(&screen).await;
    Ok(())
}

async fn stop(screen: &Screen) {
    let taken = screen.active.lock().await.take();
    if let Some(active) = taken {
        active.capture.stop();
        active.publisher.close().await;
    }
}

pub fn run() {
    tauri::Builder::default()
        .plugin(tauri_plugin_updater::Builder::new().build())
        .plugin(tauri_plugin_process::init())
        .plugin(
            tauri_plugin_log::Builder::new()
                .level(log::LevelFilter::Info)
                .targets([
                    tauri_plugin_log::Target::new(tauri_plugin_log::TargetKind::LogDir {
                        file_name: Some("vocalis".to_owned()),
                    }),
                    tauri_plugin_log::Target::new(tauri_plugin_log::TargetKind::Stdout),
                ])
                .build(),
        )
        .invoke_handler(tauri::generate_handler![
            capture_sources,
            start_screen_share,
            screen_answer,
            screen_candidate,
            stop_screen_share
        ])
        .setup(|app| {
            log::info!("vocalis {} starting", app.package_info().version);
            app.manage(Arc::new(Screen::default()));

            #[cfg(target_os = "macos")]
            if let Some(window) = app.get_webview_window("main") {
                enable_media_capture(&window);
            }

            #[cfg(target_os = "windows")]
            if let Some(window) = app.get_webview_window("main") {
                let screen = Arc::clone(&*app.state::<Arc<Screen>>());
                note_webview_process(&window, screen);
            }

            Ok(())
        })
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
