use std::time::Duration;

use bytes::Bytes;

pub const SAMPLE_RATE: u32 = 48_000;
pub const CHANNELS: usize = 2;
pub const FRAME_SAMPLES: usize = 960;

pub const FRAME: Duration = Duration::from_millis(20);

pub struct AudioEncoder {
    opus: opus::Encoder,
    pending: Vec<f32>,
    out: Vec<u8>,
}

impl AudioEncoder {
    pub fn new(bitrate: u32) -> Result<Self, String> {
        let mut opus = opus::Encoder::new(
            SAMPLE_RATE,
            opus::Channels::Stereo,
            opus::Application::Audio,
        )
        .map_err(|error| format!("opus encoder: {error}"))?;
        opus.set_bitrate(opus::Bitrate::Bits(bitrate as i32))
            .map_err(|error| format!("opus bitrate: {error}"))?;

        Ok(Self {
            opus,
            pending: Vec::with_capacity(FRAME_SAMPLES * CHANNELS * 4),
            out: vec![0u8; 4000],
        })
    }

    pub fn push(&mut self, interleaved: &[f32], mut emit: impl FnMut(Bytes, Duration)) {
        self.pending.extend_from_slice(interleaved);

        let per_frame = FRAME_SAMPLES * CHANNELS;
        while self.pending.len() >= per_frame {
            let frame: Vec<f32> = self.pending.drain(..per_frame).collect();
            match self.opus.encode_float(&frame, &mut self.out) {
                Ok(len) if len > 0 => {
                    emit(Bytes::copy_from_slice(&self.out[..len]), FRAME);
                }
                _ => {}
            }
        }
    }
}
