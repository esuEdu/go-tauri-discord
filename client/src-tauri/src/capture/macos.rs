use std::sync::mpsc;
use std::sync::{Arc, Mutex};
use std::time::Duration;

use block2::RcBlock;
use bytes::Bytes;
use objc2::rc::Retained;
use objc2::runtime::{NSObject, NSObjectProtocol, ProtocolObject};
use objc2::{define_class, msg_send, AnyThread, DefinedClass};
use objc2_core_audio_types::AudioBufferList;
use objc2_core_media::CMSampleBuffer;
use objc2_foundation::NSError;
use objc2_screen_capture_kit::{
    SCContentFilter, SCDisplay, SCShareableContent, SCStream, SCStreamConfiguration,
    SCStreamOutput, SCStreamOutputType, SCWindow,
};

use crate::encode::audio::{AudioEncoder, CHANNELS, SAMPLE_RATE};
use crate::encode::video_macos::VideoEncoder;
use crate::sources::Target;

use super::{Encoded, Options, Sink};

const PIXEL_FORMAT_420V: u32 = u32::from_be_bytes(*b"420v");
const AUDIO_BITRATE: u32 = 128_000;
const CONTENT_TIMEOUT: Duration = Duration::from_secs(10);
const HEARTBEAT: Duration = Duration::from_secs(2);
const NO_PERMISSION: &str =
    "Vocalis needs permission to record the screen. Grant it in System Settings › Privacy & Security › Screen & System Audio Recording.";

struct Handlers {
    video: Arc<VideoEncoder>,
    audio: Mutex<Option<AudioEncoder>>,
    sink: Sink,
    last: Arc<Mutex<Option<LastFrame>>>,
}

struct LastFrame {
    image: objc2_core_foundation::CFRetained<objc2_core_video::CVImageBuffer>,
    pts: objc2_core_media::CMTime,
    at: std::time::Instant,
}

unsafe impl Send for LastFrame {}

define_class!(
    #[unsafe(super(NSObject))]
    #[name = "VocalisStreamOutput"]
    #[ivars = Handlers]
    struct StreamOutput;

    unsafe impl NSObjectProtocol for StreamOutput {}

    unsafe impl SCStreamOutput for StreamOutput {
        #[unsafe(method(stream:didOutputSampleBuffer:ofType:))]
        #[allow(non_snake_case)]
        unsafe fn stream_didOutputSampleBuffer_ofType(
            &self,
            _stream: &SCStream,
            buffer: &CMSampleBuffer,
            kind: SCStreamOutputType,
        ) {
            match kind {
                SCStreamOutputType::Screen => self.on_video(buffer),
                SCStreamOutputType::Audio => self.on_audio(buffer),
                _ => {}
            }
        }
    }
);

impl StreamOutput {
    fn on_video(&self, buffer: &CMSampleBuffer) {
        let Some(image) = (unsafe { buffer.image_buffer() }) else {
            return;
        };
        let pts = unsafe { buffer.presentation_time_stamp() };
        self.ivars().video.encode(&image, pts);

        if let Ok(mut slot) = self.ivars().last.lock() {
            *slot = Some(LastFrame {
                image,
                pts,
                at: std::time::Instant::now(),
            });
        }
    }

    fn on_audio(&self, buffer: &CMSampleBuffer) {
        let Ok(mut guard) = self.ivars().audio.lock() else {
            return;
        };
        let Some(encoder) = guard.as_mut() else {
            return;
        };
        let Some(interleaved) = interleaved_pcm(buffer) else {
            return;
        };

        let sink = self.ivars().sink.audio.clone();
        encoder.push(&interleaved, |data, duration| {
            let _ = sink.try_send(Encoded { data, duration });
        });
    }
}

fn interleaved_pcm(buffer: &CMSampleBuffer) -> Option<Vec<f32>> {
    let mut needed = 0usize;
    let status = unsafe {
        buffer.audio_buffer_list_with_retained_block_buffer(
            &mut needed,
            std::ptr::null_mut(),
            0,
            None,
            None,
            0,
            std::ptr::null_mut(),
        )
    };
    if status != 0 || needed == 0 {
        return None;
    }

    let mut storage = vec![0u8; needed];
    let list = storage.as_mut_ptr() as *mut AudioBufferList;
    let mut block: *mut objc2_core_media::CMBlockBuffer = std::ptr::null_mut();

    let status = unsafe {
        buffer.audio_buffer_list_with_retained_block_buffer(
            std::ptr::null_mut(),
            list,
            needed,
            None,
            None,
            0,
            &mut block,
        )
    };
    if status != 0 {
        return None;
    }

    let held = std::ptr::NonNull::new(block)
        .map(|block| unsafe { objc2_core_foundation::CFRetained::from_raw(block) });

    let out = read_planes(unsafe { &*list });
    drop(held);
    out
}

fn read_planes(list: &AudioBufferList) -> Option<Vec<f32>> {
    let count = list.mNumberBuffers as usize;
    if count == 0 {
        return None;
    }

    let buffers = unsafe { std::slice::from_raw_parts(list.mBuffers.as_ptr(), count) };

    let planes: Vec<&[f32]> = buffers
        .iter()
        .filter(|plane| !plane.mData.is_null() && plane.mDataByteSize > 0)
        .map(|plane| unsafe {
            std::slice::from_raw_parts(
                plane.mData as *const f32,
                plane.mDataByteSize as usize / std::mem::size_of::<f32>(),
            )
        })
        .collect();

    if planes.is_empty() {
        return None;
    }

    if planes.len() == 1 {
        let only = planes[0];
        if buffers[0].mNumberChannels as usize == CHANNELS {
            return Some(only.to_vec());
        }
        let mut out = Vec::with_capacity(only.len() * CHANNELS);
        for sample in only {
            for _ in 0..CHANNELS {
                out.push(*sample);
            }
        }
        return Some(out);
    }

    let frames = planes.iter().map(|plane| plane.len()).min()?;
    let mut out = Vec::with_capacity(frames * CHANNELS);
    for index in 0..frames {
        for plane in planes.iter().take(CHANNELS) {
            out.push(plane[index]);
        }
        for _ in planes.len()..CHANNELS {
            out.push(planes[planes.len() - 1][index]);
        }
    }
    Some(out)
}

fn shareable_content() -> Result<Retained<SCShareableContent>, String> {
    let (send, receive) = mpsc::channel::<Result<Retained<SCShareableContent>, String>>();

    let handler = RcBlock::new(
        move |content: *mut SCShareableContent, error: *mut NSError| {
            let outcome = if content.is_null() {
                let reason = if error.is_null() {
                    "screen recording permission has not been granted".to_owned()
                } else {
                    unsafe { &*error }.localizedDescription().to_string()
                };
                Err(reason)
            } else {
                Ok(unsafe { Retained::retain(content) }.expect("content is not null"))
            };
            let _ = send.send(outcome);
        },
    );

    unsafe { SCShareableContent::getShareableContentWithCompletionHandler(&handler) };

    receive
        .recv_timeout(CONTENT_TIMEOUT)
        .map_err(|_| "timed out asking the system what can be shared".to_owned())?
}

fn filter_for(
    target: Target,
    content: &SCShareableContent,
) -> Result<Retained<SCContentFilter>, String> {
    match target {
        Target::Display(id) => {
            let displays = unsafe { content.displays() };
            if displays.is_empty() {
                return Err(NO_PERMISSION.to_owned());
            }
            let display: Retained<SCDisplay> = displays
                .iter()
                .find(|display| unsafe { display.displayID() } == id)
                .ok_or_else(|| "that screen is no longer there".to_owned())?;

            let filter = SCContentFilter::alloc();
            Ok(unsafe {
                SCContentFilter::initWithDisplay_excludingWindows(
                    filter,
                    &display,
                    &objc2_foundation::NSArray::new(),
                )
            })
        }
        Target::Window(id) => {
            let windows = unsafe { content.windows() };
            if windows.is_empty() {
                return Err(NO_PERMISSION.to_owned());
            }
            let window: Retained<SCWindow> = windows
                .iter()
                .find(|window| unsafe { window.windowID() } == id)
                .ok_or_else(|| "that window has closed".to_owned())?;

            let displays = unsafe { content.displays() };
            let display = displays
                .iter()
                .next()
                .ok_or_else(|| NO_PERMISSION.to_owned())?;

            let owner = unsafe { window.owningApplication() }
                .ok_or_else(|| "that window has no application".to_owned())?;
            let apps = objc2_foundation::NSArray::from_retained_slice(&[owner]);
            let filter = SCContentFilter::alloc();
            Ok(unsafe {
                SCContentFilter::initWithDisplay_includingApplications_exceptingWindows(
                    filter,
                    &display,
                    &apps,
                    &objc2_foundation::NSArray::new(),
                )
            })
        }
    }
}

fn configuration(options: &Options) -> Retained<SCStreamConfiguration> {
    let config = unsafe { SCStreamConfiguration::new() };

    unsafe {
        config.setWidth(options.quality.width as usize);
        config.setHeight(options.quality.height as usize);
        config.setPixelFormat(PIXEL_FORMAT_420V);
        config.setScalesToFit(true);
        config.setPreservesAspectRatio(true);
        config.setShowsCursor(true);
        config.setQueueDepth(5);
        config.setMinimumFrameInterval(objc2_core_media::CMTime {
            value: 1,
            timescale: options.quality.frame_rate.max(1) as i32,
            flags: objc2_core_media::CMTimeFlags(1),
            epoch: 0,
        });

        config.setCapturesAudio(options.audio);
        if options.audio {
            config.setSampleRate(SAMPLE_RATE as isize);
            config.setChannelCount(CHANNELS as isize);
            config.setExcludesCurrentProcessAudio(true);
        }
    }

    config
}

pub struct Session {
    stream: Retained<SCStream>,
    output: Retained<StreamOutput>,
    beating: Arc<std::sync::atomic::AtomicBool>,
}

unsafe impl Send for Session {}

impl Session {
    pub fn stop(self) {
        self.beating
            .store(false, std::sync::atomic::Ordering::Relaxed);
        unsafe { self.stream.stopCaptureWithCompletionHandler(None) };
        drop(self.output);
    }
}

fn beat(
    beating: Arc<std::sync::atomic::AtomicBool>,
    video: Arc<VideoEncoder>,
    last: Arc<Mutex<Option<LastFrame>>>,
) {
    std::thread::spawn(move || {
        while beating.load(std::sync::atomic::Ordering::Relaxed) {
            std::thread::sleep(HEARTBEAT);

            let Ok(slot) = last.lock() else { continue };
            let Some(frame) = slot.as_ref() else { continue };
            if frame.at.elapsed() < HEARTBEAT {
                continue;
            }
            video.encode_keyframe(&frame.image, frame.pts);
        }
    });
}

pub fn start(options: Options, sink: Sink) -> Result<Session, String> {
    let content = shareable_content()?;
    let filter = filter_for(options.target, &content)?;
    let config = configuration(&options);

    let video_sink = sink.video.clone();
    let video = Arc::new(VideoEncoder::new(
        options.quality.width,
        options.quality.height,
        options.quality.max_bitrate,
        options.quality.frame_rate,
        Arc::new(move |data: Bytes, duration: Duration, _keyframe: bool| {
            let _ = video_sink.try_send(Encoded { data, duration });
        }),
    )?);

    let last: Arc<Mutex<Option<LastFrame>>> = Arc::new(Mutex::new(None));

    let audio = if options.audio {
        Some(AudioEncoder::new(AUDIO_BITRATE)?)
    } else {
        None
    };

    let handlers = Handlers {
        video: Arc::clone(&video),
        audio: Mutex::new(audio),
        sink,
        last: Arc::clone(&last),
    };

    let output = StreamOutput::alloc().set_ivars(handlers);
    let output: Retained<StreamOutput> = unsafe { msg_send![super(output), init] };

    let stream = SCStream::alloc();
    let stream =
        unsafe { SCStream::initWithFilter_configuration_delegate(stream, &filter, &config, None) };

    let protocol = ProtocolObject::from_ref(&*output);
    let queue = dispatch2::DispatchQueue::new("dev.esuedu.vocalis.capture", None);

    unsafe {
        stream
            .addStreamOutput_type_sampleHandlerQueue_error(
                protocol,
                SCStreamOutputType::Screen,
                Some(&queue),
            )
            .map_err(|error| error.localizedDescription().to_string())?;

        if options.audio {
            stream
                .addStreamOutput_type_sampleHandlerQueue_error(
                    protocol,
                    SCStreamOutputType::Audio,
                    Some(&queue),
                )
                .map_err(|error| error.localizedDescription().to_string())?;
        }
    }

    let (send, receive) = mpsc::channel::<Option<String>>();
    let handler = RcBlock::new(move |error: *mut NSError| {
        let reason = if error.is_null() {
            None
        } else {
            Some(unsafe { &*error }.localizedDescription().to_string())
        };
        let _ = send.send(reason);
    });

    unsafe { stream.startCaptureWithCompletionHandler(Some(&handler)) };

    match receive.recv_timeout(CONTENT_TIMEOUT) {
        Ok(None) => {
            let beating = Arc::new(std::sync::atomic::AtomicBool::new(true));
            beat(Arc::clone(&beating), video, last);
            Ok(Session {
                stream,
                output,
                beating,
            })
        }
        Ok(Some(reason)) => Err(reason),
        Err(_) => Err("the capture did not start".to_owned()),
    }
}
