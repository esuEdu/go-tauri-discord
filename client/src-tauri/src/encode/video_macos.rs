use std::ptr::NonNull;
use std::sync::{Arc, Mutex};
use std::time::Duration;

use block2::RcBlock;
use bytes::{Bytes, BytesMut};
use objc2_core_foundation::{CFBoolean, CFDictionary, CFNumber, CFRetained, CFString, CFType};
use objc2_core_media::{
    CMSampleBuffer, CMTime, CMVideoFormatDescriptionGetH264ParameterSetAtIndex,
};
use objc2_core_video::CVImageBuffer;
use objc2_video_toolbox::{
    kVTCompressionPropertyKey_AllowFrameReordering, kVTCompressionPropertyKey_AverageBitRate,
    kVTCompressionPropertyKey_ExpectedFrameRate, kVTCompressionPropertyKey_MaxKeyFrameInterval,
    kVTCompressionPropertyKey_ProfileLevel, kVTCompressionPropertyKey_RealTime,
    kVTEncodeFrameOptionKey_ForceKeyFrame, kVTProfileLevel_H264_Baseline_AutoLevel,
    VTCompressionSession, VTSessionSetProperty,
};

use super::h264::{append_annex_b, avcc_to_annex_b};

const CODEC_H264: u32 = u32::from_be_bytes(*b"avc1");
const KEYFRAME_EVERY: i32 = 120;

pub type OnSample = Arc<dyn Fn(Bytes, Duration, bool) + Send + Sync>;

pub struct VideoEncoder {
    session: CFRetained<VTCompressionSession>,
    parameter_sets: Arc<Mutex<Option<Bytes>>>,
    on_sample: OnSample,
    frame: Duration,
}

unsafe impl Send for VideoEncoder {}
unsafe impl Sync for VideoEncoder {}

fn set_bool(session: &VTCompressionSession, key: &CFString, value: bool) {
    let number = CFNumber::new_i32(i32::from(value));
    let value: &CFType = &number;
    unsafe { VTSessionSetProperty(session, key, Some(value)) };
}

fn set_i32(session: &VTCompressionSession, key: &CFString, value: i32) {
    let number = CFNumber::new_i32(value);
    let value: &CFType = &number;
    unsafe { VTSessionSetProperty(session, key, Some(value)) };
}

fn set_string(session: &VTCompressionSession, key: &CFString, value: &CFString) {
    let value: &CFType = value;
    unsafe { VTSessionSetProperty(session, key, Some(value)) };
}

impl VideoEncoder {
    pub fn new(
        width: u32,
        height: u32,
        bitrate: u32,
        frame_rate: u32,
        on_sample: OnSample,
    ) -> Result<Self, String> {
        let mut raw: *mut VTCompressionSession = std::ptr::null_mut();
        let status = unsafe {
            VTCompressionSession::create(
                None,
                width as i32,
                height as i32,
                CODEC_H264,
                None,
                None,
                None,
                None,
                std::ptr::null_mut(),
                NonNull::from(&mut raw),
            )
        };
        if status != 0 || raw.is_null() {
            return Err(format!("could not open the h264 encoder (status {status})"));
        }

        let session = unsafe { CFRetained::from_raw(NonNull::new(raw).unwrap()) };

        unsafe {
            set_bool(&session, kVTCompressionPropertyKey_RealTime, true);
            set_bool(
                &session,
                kVTCompressionPropertyKey_AllowFrameReordering,
                false,
            );
            set_string(
                &session,
                kVTCompressionPropertyKey_ProfileLevel,
                kVTProfileLevel_H264_Baseline_AutoLevel,
            );
            set_i32(
                &session,
                kVTCompressionPropertyKey_AverageBitRate,
                bitrate as i32,
            );
            set_i32(
                &session,
                kVTCompressionPropertyKey_ExpectedFrameRate,
                frame_rate as i32,
            );
            set_i32(
                &session,
                kVTCompressionPropertyKey_MaxKeyFrameInterval,
                KEYFRAME_EVERY,
            );
        }

        unsafe { session.prepare_to_encode_frames() };

        Ok(Self {
            session,
            parameter_sets: Arc::new(Mutex::new(None)),
            on_sample,
            frame: Duration::from_nanos(1_000_000_000 / frame_rate.max(1) as u64),
        })
    }

    pub fn encode(&self, image: &CVImageBuffer, pts: CMTime) {
        self.encode_frame(image, pts, false)
    }

    pub fn encode_keyframe(&self, image: &CVImageBuffer, pts: CMTime) {
        self.encode_frame(image, pts, true)
    }

    fn encode_frame(&self, image: &CVImageBuffer, pts: CMTime, force_key: bool) {
        let parameter_sets = Arc::clone(&self.parameter_sets);
        let on_sample = Arc::clone(&self.on_sample);
        let frame = self.frame;

        let handler = RcBlock::new(
            move |status: i32,
                  _flags: objc2_video_toolbox::VTEncodeInfoFlags,
                  buffer: *mut CMSampleBuffer| {
                if status != 0 || buffer.is_null() {
                    return;
                }
                let buffer = unsafe { &*buffer };
                if let Some((data, keyframe)) = access_unit(buffer, &parameter_sets) {
                    on_sample(data, frame, keyframe);
                }
            },
        );

        let duration = CMTime {
            value: 1,
            timescale: (1_000_000_000u64 / self.frame.as_nanos().max(1) as u64) as i32,
            flags: objc2_core_media::CMTimeFlags(1),
            epoch: 0,
        };

        let forced = force_key.then(|| {
            let key: &CFType = unsafe { kVTEncodeFrameOptionKey_ForceKeyFrame };
            let value: &CFType = CFBoolean::new(true);
            let dict: CFRetained<CFDictionary<CFType, CFType>> =
                CFDictionary::from_slices(&[key], &[value]);
            unsafe { CFRetained::cast_unchecked::<CFDictionary>(dict) }
        });

        unsafe {
            self.session.encode_frame_with_output_handler(
                image,
                pts,
                duration,
                forced.as_deref(),
                std::ptr::null_mut(),
                &*handler as *const block2::DynBlock<_> as *mut _,
            );
        }
    }
}

impl Drop for VideoEncoder {
    fn drop(&mut self) {
        unsafe { self.session.invalidate() };
    }
}

fn access_unit(
    buffer: &CMSampleBuffer,
    parameter_sets: &Mutex<Option<Bytes>>,
) -> Option<(Bytes, bool)> {
    let block = unsafe { buffer.data_buffer() }?;

    let mut length = 0usize;
    let mut pointer: *mut u8 = std::ptr::null_mut();
    let status = unsafe {
        block.data_pointer(
            0,
            std::ptr::null_mut(),
            &mut length,
            &mut pointer as *mut *mut u8 as *mut *mut _,
        )
    };
    if status != 0 || pointer.is_null() || length == 0 {
        return None;
    }

    let avcc = unsafe { std::slice::from_raw_parts(pointer, length) };
    let annex_b = avcc_to_annex_b(avcc, 4).ok()?;
    let keyframe = has_idr(&annex_b);

    if keyframe {
        if let Some(sets) = extract_parameter_sets(buffer) {
            *parameter_sets.lock().ok()? = Some(sets);
        }
    }

    if !keyframe {
        return Some((annex_b.freeze(), false));
    }

    let sets = parameter_sets.lock().ok()?.clone();
    let Some(sets) = sets else {
        return Some((annex_b.freeze(), true));
    };

    let mut out = BytesMut::with_capacity(sets.len() + annex_b.len());
    out.extend_from_slice(&sets);
    out.extend_from_slice(&annex_b);
    Some((out.freeze(), true))
}

fn has_idr(annex_b: &[u8]) -> bool {
    let mut at = 0usize;
    while at + 5 <= annex_b.len() {
        if annex_b[at..at + 4] == [0, 0, 0, 1] {
            if annex_b[at + 4] & 0x1F == 5 {
                return true;
            }
            at += 4;
        } else {
            at += 1;
        }
    }
    false
}

fn extract_parameter_sets(buffer: &CMSampleBuffer) -> Option<Bytes> {
    let description = unsafe { buffer.format_description() }?;

    let mut count = 0usize;
    let status = unsafe {
        CMVideoFormatDescriptionGetH264ParameterSetAtIndex(
            &description,
            0,
            std::ptr::null_mut(),
            std::ptr::null_mut(),
            &mut count,
            std::ptr::null_mut(),
        )
    };
    if status != 0 || count == 0 {
        return None;
    }

    let mut out = BytesMut::new();
    for index in 0..count {
        let mut pointer: *const u8 = std::ptr::null();
        let mut size = 0usize;
        let status = unsafe {
            CMVideoFormatDescriptionGetH264ParameterSetAtIndex(
                &description,
                index,
                &mut pointer,
                &mut size,
                std::ptr::null_mut(),
                std::ptr::null_mut(),
            )
        };
        if status != 0 || pointer.is_null() || size == 0 {
            continue;
        }
        append_annex_b(&mut out, unsafe {
            std::slice::from_raw_parts(pointer, size)
        });
    }

    if out.is_empty() {
        None
    } else {
        Some(out.freeze())
    }
}
