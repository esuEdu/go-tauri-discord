use std::time::Duration;

use bytes::Bytes;
use serde::Deserialize;
use tokio::sync::mpsc::Sender;

use crate::sources::Target;

#[cfg(target_os = "macos")]
mod macos;
#[cfg(target_os = "windows")]
mod windows;

#[derive(Deserialize, Clone, Copy)]
pub struct Quality {
    pub width: u32,
    pub height: u32,
    pub frame_rate: u32,
    pub max_bitrate: u32,
}

pub struct Options {
    pub target: Target,
    pub quality: Quality,
    pub audio: bool,
    #[cfg(target_os = "windows")]
    pub webview: Option<u32>,
}

pub struct Encoded {
    pub data: Bytes,
    pub duration: Duration,
}

#[derive(Clone)]
pub struct Sink {
    pub video: Sender<Encoded>,
    pub audio: Sender<Encoded>,
}

pub struct Session {
    #[cfg(target_os = "macos")]
    inner: macos::Session,
    #[cfg(target_os = "windows")]
    inner: windows::Session,
    #[cfg(not(any(target_os = "macos", target_os = "windows")))]
    inner: (),
}

impl Session {
    pub fn stop(self) {
        #[cfg(any(target_os = "macos", target_os = "windows"))]
        self.inner.stop();
    }
}

pub fn start(options: Options, sink: Sink) -> Result<Session, String> {
    #[cfg(target_os = "macos")]
    {
        macos::start(options, sink).map(|inner| Session { inner })
    }
    #[cfg(target_os = "windows")]
    {
        windows::start(options, sink).map(|inner| Session { inner })
    }
    #[cfg(not(any(target_os = "macos", target_os = "windows")))]
    {
        let _ = (options, sink);
        Err("screen capture is not supported on this platform".to_owned())
    }
}
