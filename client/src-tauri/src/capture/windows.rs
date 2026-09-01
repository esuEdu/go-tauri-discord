use std::collections::VecDeque;
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::{Arc, Mutex};
use std::time::{Duration, Instant};

use bytes::Bytes;
use openh264::encoder::{BitRate, Encoder, EncoderConfig, FrameRate, IntraFramePeriod, UsageType};
use openh264::formats::{BgraSliceU8, YUVBuffer};
use openh264::{OpenH264API, Timestamp};
use wasapi::{initialize_mta, AudioClient, Direction, SampleType, StreamMode, WaveFormat};
use windows_capture::capture::{Context, GraphicsCaptureApiHandler};
use windows_capture::frame::Frame;
use windows_capture::graphics_capture_api::InternalCaptureControl;
use windows_capture::monitor::Monitor;
use windows_capture::settings::{
    ColorFormat, CursorCaptureSettings, DirtyRegionSettings, DrawBorderSettings,
    MinimumUpdateIntervalSettings, SecondaryWindowSettings, Settings,
};
use windows_capture::window::Window;

use crate::encode::audio::{AudioEncoder, CHANNELS, SAMPLE_RATE};
use crate::sources::Target;

use super::{Encoded, Options, Quality, Sink};

const AUDIO_BITRATE: u32 = 128_000;
const HEARTBEAT: Duration = Duration::from_secs(2);
const KEYFRAME_SECONDS: u32 = 2;
const AUDIO_CHUNK_FRAMES: usize = 480;
const AUDIO_BUFFER_HNS: i64 = 200_000;
const AUDIO_WAIT_MS: u32 = 500;

struct Shared {
    encoder: Mutex<Option<Encoder>>,
    last: Mutex<Option<(YUVBuffer, Instant)>>,
    sink: tokio::sync::mpsc::Sender<Encoded>,
    quality: Quality,
    started: Instant,
}

impl Shared {
    fn emit(&self, encoder: &mut Encoder, yuv: &YUVBuffer, at: Duration) {
        let stamp = Timestamp::from_millis(at.as_millis() as u64);
        let Ok(stream) = encoder.encode_at(yuv, stamp) else {
            return;
        };

        let data = stream.to_vec();
        if data.is_empty() {
            return;
        }

        let frame = Duration::from_nanos(1_000_000_000 / self.quality.frame_rate.max(1) as u64);
        let _ = self.sink.try_send(Encoded {
            data: Bytes::from(data),
            duration: frame,
        });
    }
}

struct VideoHandler {
    shared: Arc<Shared>,
    running: Arc<AtomicBool>,
}

impl GraphicsCaptureApiHandler for VideoHandler {
    type Flags = (Arc<Shared>, Arc<AtomicBool>);
    type Error = Box<dyn std::error::Error + Send + Sync>;

    fn new(ctx: Context<Self::Flags>) -> Result<Self, Self::Error> {
        let (shared, running) = ctx.flags;
        Ok(Self { shared, running })
    }

    fn on_frame_arrived(
        &mut self,
        frame: &mut Frame,
        capture_control: InternalCaptureControl,
    ) -> Result<(), Self::Error> {
        if !self.running.load(Ordering::Relaxed) {
            capture_control.stop();
            return Ok(());
        }

        let mut buffer = frame.buffer()?;
        let width = buffer.width() as usize;
        let height = buffer.height() as usize;

        let mut tight = Vec::new();
        let bgra = buffer.as_nopadding_buffer(&mut tight);

        let Some((scaled, out_width, out_height)) = fit(
            bgra,
            width,
            height,
            self.shared.quality.width as usize,
            self.shared.quality.height as usize,
        ) else {
            return Ok(());
        };

        let source = BgraSliceU8::new(&scaled, (out_width, out_height));
        let yuv = YUVBuffer::from_bgra8_source(source);

        let mut guard = self
            .shared
            .encoder
            .lock()
            .map_err(|_| "the encoder lock was poisoned")?;

        if guard.is_none() {
            let rate = self.shared.quality.frame_rate.max(1);
            let gop = rate * KEYFRAME_SECONDS;
            let config = EncoderConfig::new()
                .bitrate(BitRate::from_bps(self.shared.quality.max_bitrate))
                .max_frame_rate(FrameRate::from_hz(rate as f32))
                .usage_type(UsageType::ScreenContentRealTime)
                .intra_frame_period(IntraFramePeriod::from_num_frames(gop));
            log::info!(
                "screen: encoding {out_width}x{out_height} at {rate} fps, keyframe every {gop} frames"
            );
            *guard = Some(Encoder::with_api_config(
                OpenH264API::from_source(),
                config,
            )?);
        }

        let encoder = guard.as_mut().ok_or("no encoder")?;
        self.shared
            .emit(encoder, &yuv, self.shared.started.elapsed());
        drop(guard);

        if let Ok(mut last) = self.shared.last.lock() {
            *last = Some((yuv, Instant::now()));
        }

        Ok(())
    }

    fn on_closed(&mut self) -> Result<(), Self::Error> {
        self.running.store(false, Ordering::Relaxed);
        Ok(())
    }
}

fn fit(
    bgra: &[u8],
    width: usize,
    height: usize,
    max_width: usize,
    max_height: usize,
) -> Option<(Vec<u8>, usize, usize)> {
    if width < 2 || height < 2 {
        return None;
    }

    let scale = f64::min(
        max_width as f64 / width as f64,
        max_height as f64 / height as f64,
    )
    .min(1.0);

    let out_width = even(((width as f64 * scale) as usize).max(2));
    let out_height = even(((height as f64 * scale) as usize).max(2));

    let mut out = vec![0u8; out_width * out_height * 4];
    for y in 0..out_height {
        let source_y = (y * height / out_height).min(height - 1);
        for x in 0..out_width {
            let source_x = (x * width / out_width).min(width - 1);
            let from = (source_y * width + source_x) * 4;
            let to = (y * out_width + x) * 4;
            if from + 4 <= bgra.len() {
                out[to..to + 4].copy_from_slice(&bgra[from..from + 4]);
            }
        }
    }

    Some((out, out_width, out_height))
}

const fn even(value: usize) -> usize {
    value & !1
}

fn beat(shared: Arc<Shared>, running: Arc<AtomicBool>) {
    std::thread::spawn(move || {
        while running.load(Ordering::Relaxed) {
            std::thread::sleep(HEARTBEAT);

            let Ok(last) = shared.last.lock() else {
                continue;
            };
            let Some((yuv, at)) = last.as_ref() else {
                continue;
            };
            if at.elapsed() < HEARTBEAT {
                continue;
            }

            let Ok(mut guard) = shared.encoder.lock() else {
                continue;
            };
            let Some(encoder) = guard.as_mut() else {
                continue;
            };
            encoder.force_intra_frame();
            shared.emit(encoder, yuv, shared.started.elapsed());
        }
    });
}

#[derive(Clone, Copy)]
struct AudioSource {
    process_id: u32,
    include_tree: bool,
}

fn capture_audio(
    source: AudioSource,
    sink: tokio::sync::mpsc::Sender<Encoded>,
    running: Arc<AtomicBool>,
) {
    std::thread::spawn(move || {
        if let Err(reason) = pump_audio(source, sink, &running) {
            log::warn!("screen: sharing without sound: {reason}");
        }
    });
}

fn pump_audio(
    source: AudioSource,
    sink: tokio::sync::mpsc::Sender<Encoded>,
    running: &AtomicBool,
) -> Result<(), String> {
    if initialize_mta().is_err() {
        return Err("com would not start on the audio thread".to_owned());
    }

    let format = WaveFormat::new(
        32,
        32,
        &SampleType::Float,
        SAMPLE_RATE as usize,
        CHANNELS,
        None,
    );

    let mut client =
        AudioClient::new_application_loopback_client(source.process_id, source.include_tree)
            .map_err(|error| format!("no loopback for process {}: {error}", source.process_id))?;

    let mode = StreamMode::EventsShared {
        autoconvert: true,
        buffer_duration_hns: AUDIO_BUFFER_HNS,
    };
    client
        .initialize_client(&format, &Direction::Capture, &mode)
        .map_err(|error| format!("loopback would not initialise: {error}"))?;

    let event = client
        .set_get_eventhandle()
        .map_err(|error| format!("no audio event handle: {error}"))?;
    let capture = client
        .get_audiocaptureclient()
        .map_err(|error| format!("no audio capture client: {error}"))?;
    let mut opus = AudioEncoder::new(AUDIO_BITRATE)?;

    let block = format.get_blockalign() as usize;
    let chunk = block * AUDIO_CHUNK_FRAMES;
    let mut queue: VecDeque<u8> = VecDeque::new();

    client
        .start_stream()
        .map_err(|error| format!("the loopback stream would not start: {error}"))?;

    let scope = if source.include_tree {
        "that app"
    } else {
        "everything but vocalis"
    };
    log::info!(
        "screen: capturing audio from {scope} (process {})",
        source.process_id
    );

    while running.load(Ordering::Relaxed) {
        while queue.len() >= chunk {
            let bytes: Vec<u8> = queue.drain(..chunk).collect();
            let samples: Vec<f32> = bytes
                .chunks_exact(4)
                .map(|four| f32::from_le_bytes([four[0], four[1], four[2], four[3]]))
                .collect();

            let sink = sink.clone();
            opus.push(&samples, |data, duration| {
                let _ = sink.try_send(Encoded { data, duration });
            });
        }

        if let Err(error) = capture.read_from_device_to_deque(&mut queue) {
            let _ = client.stop_stream();
            return Err(format!("the loopback stream ended: {error}"));
        }

        let _ = event.wait_for_event(AUDIO_WAIT_MS);
    }

    let _ = client.stop_stream();
    Ok(())
}

pub struct Session {
    running: Arc<AtomicBool>,
}

impl Session {
    pub fn stop(self) {
        self.running.store(false, Ordering::Relaxed);
    }
}

pub fn start(options: Options, sink: Sink) -> Result<Session, String> {
    let running = Arc::new(AtomicBool::new(true));

    let shared = Arc::new(Shared {
        encoder: Mutex::new(None),
        last: Mutex::new(None),
        sink: sink.video.clone(),
        quality: options.quality,
        started: Instant::now(),
    });

    let flags = (Arc::clone(&shared), Arc::clone(&running));

    match options.target {
        Target::Display(id) => {
            let monitor = Monitor::from_raw_hmonitor(id as usize as *mut std::ffi::c_void);
            let settings = Settings::new(
                monitor,
                CursorCaptureSettings::WithCursor,
                DrawBorderSettings::Default,
                SecondaryWindowSettings::Default,
                MinimumUpdateIntervalSettings::Default,
                DirtyRegionSettings::Default,
                ColorFormat::Bgra8,
                flags,
            );
            VideoHandler::start_free_threaded(settings)
                .map_err(|error| format!("screen capture: {error}"))?;
        }
        Target::Window(id) => {
            let window = Window::from_raw_hwnd(id as usize as *mut std::ffi::c_void);
            let settings = Settings::new(
                window,
                CursorCaptureSettings::WithCursor,
                DrawBorderSettings::Default,
                SecondaryWindowSettings::Default,
                MinimumUpdateIntervalSettings::Default,
                DirtyRegionSettings::Default,
                ColorFormat::Bgra8,
                flags,
            );
            VideoHandler::start_free_threaded(settings)
                .map_err(|error| format!("screen capture: {error}"))?;
        }
    }

    beat(Arc::clone(&shared), Arc::clone(&running));

    if options.audio {
        let source = match options.target {
            Target::Window(id) => {
                let window = Window::from_raw_hwnd(id as usize as *mut std::ffi::c_void);
                match window.process_id() {
                    Ok(process_id) => Some(AudioSource {
                        process_id,
                        include_tree: true,
                    }),
                    Err(error) => {
                        log::warn!("screen: no process behind that window ({error}), sharing without sound");
                        None
                    }
                }
            }
            Target::Display(_) => Some(AudioSource {
                process_id: options.webview.unwrap_or_else(std::process::id),
                include_tree: false,
            }),
        };

        if let Some(source) = source {
            capture_audio(source, sink.audio.clone(), Arc::clone(&running));
        }
    }

    Ok(Session { running })
}
